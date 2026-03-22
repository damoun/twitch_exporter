GO     ?= go
GOFMT  ?= $(GO)fmt
pkgs   = ./...

.PHONY: all
all: style lint build test

.PHONY: build
build:
	goreleaser build --snapshot --clean --single-target

.PHONY: test
test:
	$(GO) test -race $(pkgs)

.PHONY: test-short
test-short:
	$(GO) test -short $(pkgs)

.PHONY: format
format:
	$(GO) fmt $(pkgs)

.PHONY: style
style:
	@echo ">> checking code style"
	@fmtRes=$$($(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print)); \
	if [ -n "$${fmtRes}" ]; then \
		echo "gofmt checking failed!"; echo "$${fmtRes}"; \
		exit 1; \
	fi

.PHONY: lint
lint:
	golangci-lint run $(pkgs)
