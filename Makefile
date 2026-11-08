BINARY   := psv
PKG      := github.com/brian-nunez/psv
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

# Real-world example videos and their reports, see testdata/examples.
EXAMPLES := $(wildcard testdata/examples/*/video.mp4)

.PHONY: all build install test examples serve vet lint fmt fmt-check clean

all: fmt-check lint build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/psv

install:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) ./cmd/psv

test:
	go test -race -short ./...
	go test -timeout 30m ./...

# Regenerate the example reports with the current build. Exit status 1 means
# the video failed, which is expected for most examples.
examples: build
	@for v in $(EXAMPLES); do \
		d=$$(dirname $$v); echo "analysing $$d"; \
		(cd $$d && ../../../bin/$(BINARY) -progress=false -format json -frames video.mp4 > report.json); \
		[ $$? -le 1 ] || exit 2; \
	done

serve: build
	./bin/$(BINARY) serve

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
