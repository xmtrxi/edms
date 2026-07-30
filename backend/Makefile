BINARY=api
MAIN=./cmd/api

.PHONY: all build dev fmt vet lint test clean

all: fmt vet test build

build:
	go build -ldflags="-s -w" -trimpath -o bin/$(BINARY) $(MAIN)

dev:
	go run $(MAIN)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./... 2>/dev/null || go vet ./...

test:
	go test ./... -count=1

clean:
	rm -rf bin/
