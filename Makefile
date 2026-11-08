# psv build. Requires Go and, to analyse video or run every test, FFmpeg.
#
#   make build                  bin/psv
#   make test                   race detector on a subset, then every test
#   make release VERSION=vX.Y.Z clean dist/, build every platform and write dist/SHA256SUMS
#
# Releases are made by GitHub Actions (.github/workflows/release.yml), not from a local machine.

BINARY   := psv
PKG      := github.com/brian-nunez/psv
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

# Release platforms, GOOS/GOARCH.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# Real-world example videos and their reports, see testdata/examples.
EXAMPLES := $(wildcard testdata/examples/*/video.mp4)

.PHONY: all build install test test-scripts examples serve vet lint fmt fmt-check release clean

all: fmt-check lint build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/psv

install:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) ./cmd/psv

test:
	go test -race -short ./...
	go test -timeout 30m ./...

# Tests for the release scripts in scripts/.
test-scripts:
	bash scripts/next-version_test.sh

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

# Release files in dist/: one binary per platform, named psv-<os>-<arch>, plus SHA256SUMS.
release:
	rm -rf dist
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		out=dist/$(BINARY)-$$os-$$arch$$ext; echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $$out ./cmd/psv || exit 1; \
	done
	cd dist && shasum -a 256 $(BINARY)-* > SHA256SUMS
	@cat dist/SHA256SUMS

clean:
	rm -rf bin dist
