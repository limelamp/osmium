# Osmium - terminal Minecraft server manager.
#
# `make` on its own lists what there is. Every target is a shorthand for
# something you could type by hand; nothing here hides anything from you.

BINARY      := osmium
MODULE      := github.com/limelamp/osmium
BIN_DIR     := bin

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        ?= $(shell date -u +%Y-%m-%d)

# What the binary knows about itself, set at link time.
LDFLAGS := -s -w \
	-X $(MODULE)/internal/build.Version=$(VERSION) \
	-X $(MODULE)/internal/build.Commit=$(COMMIT) \
	-X $(MODULE)/internal/build.Date=$(DATE)

GO      ?= go
GOFLAGS := -trimpath

.DEFAULT_GOAL := help
.PHONY: help run build build-small install test test-race lint lint-fix fmt tidy vet completions snapshot release clean

help: ## List the targets
	@echo "Osmium $(VERSION)"
	@echo
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

run: ## Open the interface (go run .)
	$(GO) run . $(ARGS)

build: ## Build ./bin/osmium
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .
	@echo "built $(BIN_DIR)/$(BINARY) $(VERSION)"

build-small: build ## Build, then compress with UPX if it is installed
	@command -v upx >/dev/null 2>&1 \
		&& upx --best $(BIN_DIR)/$(BINARY) \
		|| echo "upx not installed; leaving $(BIN_DIR)/$(BINARY) uncompressed"

install: ## Install osmium into the Go bin directory
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" .
	@echo "installed $$($(GO) env GOPATH)/bin/$(BINARY)"

test: ## Run the tests
	$(GO) test ./...

test-race: ## Run the tests with the race detector
	$(GO) test -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

lint-fix: ## Run golangci-lint, fixing what it can
	golangci-lint run --fix ./...

fmt: ## Format the code (gofumpt and goimports, through golangci-lint)
	golangci-lint fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

tidy: ## Tidy and verify the module
	$(GO) mod tidy
	$(GO) mod verify

completions: build ## Write shell completions into ./completions
	@mkdir -p completions
	$(BIN_DIR)/$(BINARY) completion bash > completions/$(BINARY).bash
	$(BIN_DIR)/$(BINARY) completion zsh  > completions/_$(BINARY)
	$(BIN_DIR)/$(BINARY) completion fish > completions/$(BINARY).fish
	@echo "wrote completions/"

snapshot: ## Build release artefacts locally, without tagging
	goreleaser build --snapshot --clean

release: ## Publish a release for the current tag
	goreleaser release --clean

clean: ## Remove build output
	rm -rf $(BIN_DIR) dist completions
	$(GO) clean -testcache
