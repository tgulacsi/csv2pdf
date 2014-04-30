GO =? go

all: fontdir.go
	go build

fontdir.go: assets/fontdir.zip go-bindata
	go-bindata -nomemcopy -nocompress -prefix=assets -o=./fontdir.go assets

go-bindata:
	which go-bindata || go get github.com/jteeuwen/go-bindata/...

assets/fontdir.zip: font/
	mkdir -p assets
	zip -jr9 assets/fontdir.zip font

clean:
	rm -f fontdir.go assets/fontdir.zip csv2pdf
