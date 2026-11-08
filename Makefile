BINARY   := psv
PKG      := github.com/brian-nunez/psv
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: all build install test vet lint fmt fmt-check clean prototype

all: fmt-check lint build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/psv

install:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) ./cmd/psv

test:
	go test -race -short ./...
	go test -timeout 30m ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...
	golangci-lint run --fix ./...

fmt-check:
	golangci-lint fmt --diff ./...

# PROTOTYPE: throwaway report UI, see prototype/reportui.
prototype:
	go run ./prototype/reportui

clean:
	rm -rf bin dist
