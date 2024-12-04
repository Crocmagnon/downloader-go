.PHONY: all test lint downloader

all: test lint downloader

dist:
	mkdir -p dist

downloader: dist
	GOOS=linux GOARCH=amd64 go build -o ./dist/downloader-linux-amd64 .
	go build -o ./dist/downloader .

deploy: all
	scp ./dist/downloader-linux-amd64 ubuntu:/mnt/data/paperless-ngx/downloader

lint:
	golangci-lint run --fix ./...

test:
	go test ./...
