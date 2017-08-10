GO =? go

all: fontdir.go
	go build

fontdir.go: assets/fontdir.zip go-bindata
	go generate

clean:
	rm -f fontdir.go assets/fontdir.zip csv2pdf
