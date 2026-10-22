BINARY   := psv
PKG      := github.com/brian-nunez/psv
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

# Trace RERC PSE benchmark media used by the conformance tests.
MEDIA_REF := ef78542aad496827f25064f57aaf13d6a741ba75
MEDIA_DIR := testdata/pse-test-media

.PHONY: all build install test testdata vet lint fmt fmt-check clean

all: fmt-check lint build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/psv

install:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) ./cmd/psv

test: testdata
	go test -race -short ./...
	go test -timeout 30m ./...

testdata: $(MEDIA_DIR)

$(MEDIA_DIR):
	mkdir -p $@.tmp
	curl -fsSL https://codeload.github.com/traceRERC/pse-test-media/tar.gz/$(MEDIA_REF) | tar xz --strip-components=1 -C $@.tmp
	mv $@.tmp $@

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
	golangci-lint run --fix ./...

fmt-check:
	golangci-lint fmt --diff ./...

clean:
	rm -rf bin dist
