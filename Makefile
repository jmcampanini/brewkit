.PHONY: help build test lint lint-fix fmt fmt-check tidy tidy-check check clean

BUILD_DIR   := build
BINARY      := $(BUILD_DIR)/brewkit
CMD         := ./cmd/brewkit
PKG         := ./...
GOFMT_FILES := $(shell git ls-files --cached --others --exclude-standard -- '*.go')

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/brewkit/internal/cli.Version=$(VERSION)"

.DEFAULT_GOAL := help

help: ## list tasks
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ \
	     {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## compile binary to ./build/brewkit
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test: ## run tests with -race
	go test -race $(PKG)

lint: ## run golangci-lint
	golangci-lint run $(PKG)

lint-fix: ## run golangci-lint with --fix
	golangci-lint run --fix $(PKG)

fmt: ## apply gofmt -w to tracked/non-ignored Go files
	@if [ -n "$(GOFMT_FILES)" ]; then gofmt -w $(GOFMT_FILES); fi

fmt-check: ## fail if tracked/non-ignored Go files need gofmt
	@if [ -z "$(GOFMT_FILES)" ]; then exit 0; fi; \
	diff=$$(gofmt -l $(GOFMT_FILES) 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "gofmt failed (rc=$$rc):"; echo "$$diff"; exit $$rc; fi; \
	if [ -n "$$diff" ]; then echo "gofmt issues:"; echo "$$diff"; exit 1; fi

tidy: ## apply go mod tidy
	go mod tidy

tidy-check: ## fail if go mod tidy would change go.mod/go.sum
	@out=$$(go mod tidy -diff); rc=$$?; \
	if [ $$rc -eq 0 ]; then exit 0; fi; \
	if [ -n "$$out" ]; then echo "$$out"; echo "go mod tidy would change go.mod/go.sum"; exit 1; fi; \
	echo "go mod tidy failed (rc=$$rc)"; exit $$rc

check: fmt-check tidy-check lint test ## CI gate: fmt-check + tidy-check + lint + test

clean: ## remove build artifacts + test cache
	rm -rf $(BUILD_DIR) dist brewkit coverage.out coverage.html
	go clean -testcache
