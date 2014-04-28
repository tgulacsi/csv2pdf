GO =? go

all: fontzip.c
	go build

fontzip.zip: font/
	zip -r9 fontzip.zip font

fontzip.c: fontzip.zip rsrc
	rsrc -data=fontzip.zip -o fontzip.syso >fontzip.c

rsrc:
	which rsrc || go get https://github.com/akavel/rsrc

clean:
	rm -f fontzip.c fontzip.zip fontzip.syso csv2pdf
