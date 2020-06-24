VERSION := $(shell git describe --always --dirty --tags)
CGO_EN := 0

.PHONY: default get test all

default:
	CGO_ENABLED=${CGO_EN} go build -i -ldflags="-X 'main.Version=${VERSION}'" -o ${GOPATH}/bin/exeq

get:
	go get

fmt:
	go fmt ./...

linux_64:
	CGO_ENABLED=${CGO_EN} GOOS=linux GOARCH=amd64 go build -i -ldflags="-X 'main.Version=${VERSION}'" -o ${GOPATH}/bin/build/exeq-linux_amd64

darwin_64:
	CGO_ENABLED=${CGO_EN} GOOS=darwin GOARCH=amd64 go build -i -ldflags="-X 'main.Version=${VERSION}'" -o ${GOPATH}/bin/build/exeq-darwin_amd64

all: default linux_64 darwin_64


