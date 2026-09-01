VERSION=v1.0.0
LDFLAGS=-X 'github.com/shredd0r/anki-card-creator/internal/version'


install:
	

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags='${LDFLAGS}.Version=${VERSION}' -o anki-card-creator.exe
	
build-macos-arm:
	GOOS=darwin GOARCH=arm64 go build -ldflags='${LDFLAGS}.Version=${VERSION}' -o anki-card-creator

build-macos-intel:
	GOOS=darwin GOARCH=amd64 go build -ldflags='${LDFLAGS}.Version=${VERSION}' -o anki-card-creator

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags='${LDFLAGS}.Version=${VERSION}' -o anki-card-creator

build-linux-arm:
	GOOS=linux GOARCH=arm64 go build -ldflags='${LDFLAGS}.Version=${VERSION}' -o anki-card-creator

test:
	go generate ./...
	go test -v -race -count=5 ./...
	
