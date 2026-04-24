BINARY   := gh-wrapper
MODULE   := $(shell go list -m)
COVER    := coverage.out

.PHONY: build test fmt clean default

default: fmt build test

build:
	go build -o $(BINARY) .

test:
	go test -race -coverprofile=$(COVER) ./...
	go tool cover -func=$(COVER)

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) $(COVER)
