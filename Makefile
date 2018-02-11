GO =? go

all: fontdir.go
	go build

fontdir.go: assets/fontdir.zip statik
	go generate

clean:
	rm -f fontdir.go assets/fontdir.zip csv2pdf
