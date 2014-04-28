GO =? go

all: font.c
	go build

font.zip:
	zip -r9 font.zip font

font.c: font.zip rsrc
	rsrc -data=font.zip -o font.syso >font.c

rsrc:
	which rsrc || go get https://github.com/akavel/rsrc

clean:
	rm -f font.c font.syso font.zip csv2pdf
