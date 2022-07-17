// Copyright 2014 The Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// Package main of csv2pdf implements a csv -> PDF printer
package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/pkg/errors"
	"github.com/tgulacsi/go/text"
	"golang.org/x/sync/errgroup"

	_ "github.com/tgulacsi/csv2pdf/statik"
	"github.com/tgulacsi/statik/fs"
)

//go:generate mkdir -p assets
//go:generate zip -qjr9 assets/fontdir.zip font
//go:generate go get github.com/tgulacsi/statik
//go:generate statik -Z -f -src=./assets/

func main() {
	flagCharset := flag.String("charset", "utf-8", "input charset")
	flagFontDir := flag.String("fontdir", "", "font directory")
	flag.Parse()

	fontDir, closeFontDir, err := prepareFontDir(*flagFontDir)
	if err != nil {
		log.Fatalf("error preparing font dir %q: %v", *flagFontDir, err)
	}
	defer closeFontDir()

	encoding := text.GetEncoding(*flagCharset)
	csDecoder := func(r io.Reader) io.Reader { return text.NewDecodingReader(r, encoding) }
	cs := *flagCharset
	if cs == "utf-8" {
		cs = "iso-8859-2"
	}
	fn := filepath.Join(fontDir, strings.ToLower(cs)+".map")
	pdfTranslator, err := gofpdf.UnicodeTranslatorFromFile(fn)
	if err != nil {
		log.Fatalf("error loading charset mapping from %q: %v", fn, err)
	}

	var (
		csvFn   = flag.Arg(0)
		csvFile *os.File
	)
	if csvFn == "" || csvFn == "-" {
		// we must save it somewhere
		csvFile, err = os.CreateTemp("", "csv2pdf-")
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
		log.Fatalf("error opening %q: %v", csvFn, err)
	}
	defer csvFile.Close()
	parts, err := parseCsv(csDecoder(csvFile))
	if err != nil {
		log.Fatalf("error parsing csv %q: %v", csvFn, err)
	}
	if _, err = csvFile.Seek(0, 0); err != nil {
		log.Fatalf("error seeking back on %v: %v", csvFile, err)
	}
	cr := csv.NewReader(csDecoder(csvFile))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	pdf := gofpdf.New("P", "mm", "A4", fontDir)
	defPageWidth, defPageHeight, _ := pdf.PageSize(0)
	defPageSize := gofpdf.SizeType{Wd: defPageWidth, Ht: defPageHeight}
	n := 0
	for _, part := range parts {
		log.Printf("head=%q, colwidths=%+v", part.head, part.widths)
		totalWidth := 0
		for i := range part.head {
			if len(part.head[i]) > part.widths[i] {
				totalWidth += len(part.head[i])
			} else {
				totalWidth += part.widths[i]
			}
		}
		orientation := "P"
		if totalWidth > 190 {
			orientation = "L"
		}
		pdf.AddPageFormat(orientation, defPageSize)

		rowWriter := makeTable(pdf, pdfTranslator, part.head, part.widths)
		if _, err = cr.Read(); err != nil {
			log.Fatalf("error reading head of %v: %v", cr, err)
		}
		for n++; n < part.lastLine; n++ {
			record, err := cr.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("error reading csv %v: %v", cr, err)
			}
			rowWriter(record)
		}
		if err = pdf.Output(os.Stdout); err != nil {
			log.Fatalf("error writing PDF: %v", err)
		}
	}
}

func prepareFontDir(path string) (fontDir string, closeDir func() error, err error) {
	fontDir = path
	if fontDir != "" {
		return
	}

	statikFS, e := fs.New()
	if e != nil {
		err = errors.Wrap(e, "statik")
		return
	}
	fontZipData, e := fs.ReadFile(statikFS, "/fontdir.zip")
	if e != nil {
		err = errors.Wrap(e, "read fontdir.zip")
		return
	}
	zr, e := zip.NewReader(bytes.NewReader(fontZipData), int64(len(fontZipData)))
	if e != nil {
		err = errors.Wrap(e, "opening zip")
		return
	}

	if fontDir, err = os.MkdirTemp("", "csv2pdf-font-"); err != nil {
		err = errors.Wrap(err, "create temp dir for fonts")
		return
	}
	closeDir = func() error { return os.RemoveAll(fontDir) }
	defer func() {
		if err != nil && closeDir != nil {
			closeDir()
			closeDir = nil
		}
	}()

	tokens := make(chan struct{}, 16)
	var token struct{}
	var grp errgroup.Group
	for _, fi := range zr.File {
		fi := fi
		grp.Go(func() error {
			tokens <- token
			defer func() { <-tokens }()
			src, err := fi.Open()
			if err != nil {
				log.Printf("error opening %q: %v", fi.Name, err)
				return nil
			}
			defer src.Close()
			dstFn := filepath.Join(fontDir, fi.Name)
			dst, err := os.Create(dstFn)
			if err != nil {
				src.Close()
				log.Printf("error creating %q: %v", dstFn, err)
				return nil
			}
			defer dst.Close()
			//log.Printf("copying %s to %s", fi.Name, dstFn)
			if _, err = io.Copy(dst, src); err != nil {
				log.Printf("error copying: %v", err)
				return errors.Wrapf(err, "copy %q to %q", fi.Name, dstFn)
			}
			return dst.Close()
		})
	}
	err = grp.Wait()
	return
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
	pdf.SetFont("Arial", "B", 10)

	colwidths := make([]float64, len(widths))
	for i, w := range widths {
		colwidths[i] = maxFloat(float64(w)*1.75, float64(len(header[i]))*2)
	}
	// Header
	for i, v := range header {
		pdf.CellFormat(colwidths[i], 7, pdfTranslator(v), "1", 0, "C", true, 0, "")
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
			pdf.CellFormat(colwidths[i], 6, pdfTranslator(v), "LR", 0, "L", fill, 0, "")
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

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
