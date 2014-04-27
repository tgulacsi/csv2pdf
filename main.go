// Copyright 2014 The Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// Package main of csv2pdf implements a csv -> PDF printer
package main

import (
	"encoding/csv"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/gofpdf"
	"github.com/tgulacsi/go/text"
)

func main() {
	flagCharset := flag.String("charset", "utf-8", "input charset")
	flagFontDir := flag.String("fontdir", "font", "font and mapping directory")
	flag.Parse()

	encoding := text.GetEncoding(*flagCharset)
	var (
		csDecoder     func(r io.Reader) io.Reader
		err           error
		pdfTranslator = func(t string) string { return t }
	)
	if encoding != nil {
		csDecoder = func(r io.Reader) io.Reader { return text.NewDecodingReader(r, encoding) }
		fn := filepath.Join(*flagFontDir, strings.ToLower(*flagCharset)+".map")
		if pdfTranslator, err = gofpdf.UnicodeTranslatorFromFile(fn); err != nil {
			log.Fatalf("error loading charset mapping from %q: %v", fn, err)
		}
	} else {
		csDecoder = func(r io.Reader) io.Reader { return text.NewReplacementReader(r) }
	}

	var (
		csvFn   = flag.Arg(0)
		csvFile *os.File
	)
	if csvFn == "" || csvFn == "-" {
		// we must save it somewhere
		csvFile, err = ioutil.TempFile("", "csv2pdf-")
		if err != nil {
			log.Fatalf("error creating tempfile: %v", err)
		}
		if _, err := io.Copy(csvFile, os.Stdin); err != nil {
			csvFile.Close()
			log.Fatalf("error saving csv: %v", err)
		}
		csvFn = csvFile.Name()
		csvFile.Close()
	}
	if csvFile, err = os.Open(csvFn); err != nil {
		log.Fatalf("error opening %q: %v", err)
	}
	defer csvFile.Close()
	parts, err := parseCsv(csDecoder(csvFile))
	if err != nil {
		log.Fatalf("error parsing csv %q: %v", csvFn, err)
	}
	if _, err = csvFile.Seek(0, 0); err != nil {
		log.Fatalf("error seeking back on %s: %v", csvFile, err)
	}
	cr := csv.NewReader(csDecoder(csvFile))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	pdf := gofpdf.New("P", "mm", "A4", "font")
	defPageWidth, defPageHeight, _ := pdf.PageSize(0)
	defPageSize := gofpdf.SizeType{defPageWidth, defPageHeight}
	n := 0
	for _, part := range parts {
		log.Printf("head=%q, colwidths=%+v", part.head, part.widths)
		totalWidth := 0
		for i := range part.head {
			if len(part.head[i]) > part.widths[i] {
				part.widths[i] = len(part.head[i])
			}
			totalWidth += part.widths[i]
		}
		orientation := "P"
		if totalWidth > 190 {
			orientation = "L"
		}
		pdf.AddPageFormat(orientation, defPageSize)

		rowWriter := makeTable(pdf, pdfTranslator, part.head, part.widths)
		if _, err = cr.Read(); err != nil {
			log.Fatalf("error reading head of %s: %v", cr, err)
		}
		for n++; n < part.lastLine; n++ {
			record, err := cr.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("error reading csv %s: %v", cr, err)
			}
			rowWriter(record)
		}
		if err = pdf.Output(os.Stdout); err != nil {
			log.Fatalf("error writing PDF: %v", err)
		}
	}
}

// makeTable prepares a table and returns a function for inserting the rows
func makeTable(pdf *gofpdf.Fpdf, pdfTranslator func(string) string,
	header []string, widths []int) func([]string,
) {
	// Colors, line width and bold font
	pdf.SetFillColor(255, 0, 0)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(128, 0, 0)
	pdf.SetLineWidth(.3)
	pdf.SetFont("Arial", "B", 12)

	// Header
	for i, v := range header {
		pdf.CellFormat(float64(widths[i])*1.25, 7, pdfTranslator(v), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Color and font restoration
	pdf.SetFillColor(224, 235, 255)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 8)

	// Data
	fill := false
	return func(record []string) {
		for i, v := range record {
			pdf.CellFormat(float64(widths[i])*1.25, 6, pdfTranslator(v), "LR", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}
}

type partDesc struct {
	firstLine, lastLine int
	head                []string
	widths              []int
}

func parseCsv(r io.Reader) ([]partDesc, error) {
	var err error
	cr := csv.NewReader(r)
	// TODO(tgulacsi): heuristics for finding out the comma from the first line
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	parts := make([]partDesc, 0, 1)
	var part partDesc
	// read heading
	part.head, err = cr.Read()
	if err != nil {
		return nil, err
	}
	part.widths = make([]int, len(part.head))

	n := 1
	for {
		record, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		n++
		if len(record) != len(part.head) {
			log.Printf("new part with %d cols (previous part had %d)", len(record), len(part.head))
			parts = append(parts, part)
			part.lastLine = n - 1
			part.firstLine = n
			part.head = record
			part.widths = make([]int, len(part.head))
			continue
		}
		for i, v := range record {
			if len(v) > part.widths[i] {
				part.widths[i] = len(v)
			}
		}
	}
	part.lastLine = n - 1
	parts = append(parts, part)

	return parts, nil
}
