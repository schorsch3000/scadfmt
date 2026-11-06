MODULE        := github.com/schorsch3000/scadfmt
BINARY        := scadfmt
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -s -w -X main.version=$(VERSION)
GOTESTFLAGS   ?=
FUZZTIME      ?= 30s
PLATFORMS     := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# The OpenSCAD source the formatter is tested against (grammar reference and
# test corpus). It is git-ignored and fetched on demand; see `make corpus`.
OPENSCAD_REPO := https://github.com/openscad/openscad.git
OPENSCAD_REF  := a81e863051c42ee04168683e945aca58d32d04a2

.DEFAULT_GOAL := build
.PHONY: help build install test vet fmt fmt-check lint check fuzz corpus dist clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build ./scadfmt
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/scadfmt

install: ## Install scadfmt into $GOBIN
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/scadfmt

test: ## Run all tests (the corpus tests need `make corpus` once)
	go test $(GOTESTFLAGS) ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the Go sources
	gofmt -w cmd internal

fmt-check: ## Fail if any Go source is not gofmt-formatted
	@out=$$(gofmt -l cmd internal); \
	if [ -n "$$out" ]; then echo "not gofmt-formatted:"; echo "$$out"; exit 1; fi

lint: fmt-check vet ## fmt-check and vet

check: lint test ## Everything CI runs

fuzz: ## Fuzz the formatter (FUZZTIME=30s)
	go test ./internal/format -run '^$$' -fuzz=FuzzFormat -fuzztime=$(FUZZTIME)

corpus: ## Fetch the pinned OpenSCAD source into ./openscad if missing
	@if [ -d openscad/tests ]; then echo "openscad/ already present"; else \
		mkdir -p openscad && cd openscad && git init -q && \
		git remote add origin $(OPENSCAD_REPO) && \
		git fetch -q --depth 1 origin $(OPENSCAD_REF) && git checkout -q FETCH_HEAD; \
	fi

dist: ## Cross-compile release archives and SHA256SUMS into ./dist
	rm -rf dist && mkdir -p dist/completions
	go run ./cmd/scadfmt --completion=bash > dist/completions/scadfmt.bash
	go run ./cmd/scadfmt --completion=zsh  > dist/completions/_scadfmt
	go run ./cmd/scadfmt --completion=fish > dist/completions/scadfmt.fish
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; [ "$$os" = windows ] && ext=.exe; \
		name=$(BINARY)_$(VERSION)_$${os}_$${arch}; \
		echo "building $$name"; \
		mkdir -p dist/$$name; \
		cp -r dist/completions dist/$$name/completions; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/$(BINARY)$$ext ./cmd/scadfmt; \
		if [ "$$os" = windows ]; then (cd dist && zip -qr $$name.zip $$name); \
		else tar -C dist -czf dist/$$name.tar.gz $$name; fi; \
		rm -rf dist/$$name; \
	done
	rm -rf dist/completions
	cd dist && sha256sum * > SHA256SUMS

clean: ## Remove build output
	rm -rf $(BINARY) $(BINARY).exe dist
