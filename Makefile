# psv build. Requires Go and, to analyse video or run every test, FFmpeg.
#
#   make build                  bin/psv
#   make test                   race detector on a subset, then every test (CI runs this)
#   make test-examples          the full-resolution example videos (CI runs these on demand)
#   make release VERSION=vX.Y.Z clean dist/, build every platform and write dist/SHA256SUMS
#   make site                   the project site in _site/ (GitHub Actions publishes it to Pages)
#   make proofs                 check the Lean model of the detector's counting in proofs/ (needs elan)
#   make model-vectors          regenerate testdata/model/vectors.json from the Lean model
#   make fuzz                   fuzz the detector against its model, FUZZTIME per target
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
REPORTS  := $(wildcard testdata/examples/*/report.json)

.PHONY: all build install test test-examples test-scripts examples clips serve site vet lint fmt fmt-check release clean proofs model-vectors fuzz

all: fmt-check lint build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/psv

install:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) ./cmd/psv

test:
	go test -race -short ./...
	go test -timeout 30m ./...

# The full-resolution example videos in testdata/examples, checked against their reports.
# EXAMPLE=<folder> runs one of them.
test-examples:
	PSV_TEST_EXAMPLES=1 go test -count=1 -timeout 30m -v \
		-run '^TestExamples$$$(if $(EXAMPLE),/^$(EXAMPLE)$$)' .

# Tests for the release scripts in scripts/.
test-scripts:
	bash scripts/next-version_test.sh

# The Lean model of the detector's per-pixel and per-region counting, see proofs/README.md. The
# build fails on any warning, so a proof left as sorry fails it. elan installs the Lean version
# pinned in proofs/lean-toolchain.
proofs:
	cd proofs && lake build

# The model's answers on fixed cases, which TestModelVectors replays against the detector. CI
# fails when the committed file differs from the model's output.
model-vectors: proofs
	cd proofs && lake exe vectors > ../testdata/model/vectors.json.tmp
	mv testdata/model/vectors.json.tmp testdata/model/vectors.json

# Fuzz the detector's counting against the model's Go translation in model_internal_test.go.
# go test already runs each target's seed corpus; this explores new inputs for FUZZTIME each.
FUZZTIME ?= 30s
FUZZ     := FuzzPixelMatchesModel FuzzRegionMatchesModel FuzzAgeMatchesShift

fuzz:
	@for f in $(FUZZ); do \
		echo "fuzzing $$f"; \
		go test -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) . || exit 1; \
	done

# Regenerate the example reports with the current build. Exit status 1 means
# the video failed, which is expected for most examples.
examples: build
	@for v in $(EXAMPLES); do \
		d=$$(dirname $$v); echo "analysing $$d"; \
		(cd $$d && ../../../bin/$(BINARY) -progress=false -format json -frames video.mp4 > report.json); \
		[ $$? -le 1 ] || exit 2; \
	done

# The small clips the CLI and API tests decode, see testdata/clips: 320x180 at 30 fps. pass.mp4 is
# 3 seconds of still colour bars and long.mp4 60 seconds of them; fail.mp4 flashes the whole frame
# white, then red, on black, five times a second for 3 seconds.
clips:
	ffmpeg -v error -y -f lavfi -i "smptebars=s=320x180:r=30:d=3" \
		-pix_fmt yuv420p -c:v libx264 -crf 23 -g 30 testdata/clips/pass.mp4
	ffmpeg -v error -y -f lavfi -i "smptebars=s=320x180:r=30:d=60" \
		-pix_fmt yuv420p -c:v libx264 -crf 23 -g 30 testdata/clips/long.mp4
	ffmpeg -v error -y -f lavfi -i "color=c=black:s=320x180:r=30:d=3" \
		-vf "drawbox=x=0:y=0:w=iw:h=ih:color=white:t=fill:enable='lt(n,45)*lt(mod(n,6),3)',drawbox=x=0:y=0:w=iw:h=ih:color=red:t=fill:enable='gte(n,45)*lt(mod(n,6),3)'" \
		-pix_fmt yuv420p -c:v libx264 -crf 23 -g 30 testdata/clips/fail.mp4

serve: build
	./bin/$(BINARY) serve

# The project site: site/ plus a psv render page of every committed example report. No video is
# analysed or copied. .github/workflows/pages.yml publishes the result to GitHub Pages.
site: build
	rm -rf _site
	mkdir -p _site/reports
	cp -R site/. _site/
	@for r in $(REPORTS); do \
		n=$$(basename $$(dirname $$r)); echo "rendering $$n"; \
		./bin/$(BINARY) render -o _site/reports/$$n.html $$r || exit 2; \
	done

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
	rm -rf bin dist _site
