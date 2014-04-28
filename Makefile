all: csv2pdf
	go build

csv2pdf: font.rice.rice-box.go

font.rice-box.go: font.rice/fontdir.zip main.go
	rice embed

rice:
	which rice || go get github.com/GeertJohan/go.rice/rice

font.rice/fontdir.zip: font/
	mkdir -p font.rice
	(cd font && zip -r9 ../font.rice/fontdir.zip *)

fontzip.zip: font/
	zip -r9 fontzip.zip font

fontzip.c: fontzip.zip rsrc
	rsrc -data=fontzip.zip -o fontzip.syso >fontzip.c

rsrc:
	which rsrc || go get github.com/akavel/rsrc


clean:
	rm -f fontzip.c fontzip.zip fontzip.syso csv2pdf font.rice-box.go
