# Kaizen developer tasks. `make help` lists them.
#
# The review UI (web/review) is a Vite + React app built into pkg/reviewui/static and
# embedded in the Go binary, so `make ui` must run (and its output be committed)
# whenever web/review changes. CI fails when the committed build is stale.

UI_DIR := web/review
BINARY := kaizen
KAIZEN_URL ?= http://127.0.0.1:7799

.PHONY: help ui ui-deps ui-dev ui-test build test lint clean

help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

ui-deps:
	@if [ ! -d $(UI_DIR)/node_modules ]; then cd $(UI_DIR) && npm ci; fi

ui: ui-deps ## Build the review UI into pkg/reviewui/static (commit the result)
	cd $(UI_DIR) && npm run build

ui-dev: ui-deps ## Vite dev server with hot reload; proxies /api to KAIZEN_URL (a running `kaizen review`)
	@echo "Proxying /api to $(KAIZEN_URL). Start 'kaizen review <PR> --port <port>' (or review-demo) first,"
	@echo "then open the Vite URL with the same ?t=<token> that kaizen printed."
	cd $(UI_DIR) && KAIZEN_URL=$(KAIZEN_URL) npm run dev

ui-test: ui-deps ## Typecheck and unit-test the review UI
	cd $(UI_DIR) && npm run typecheck && npm test

build: ## Build ./kaizen (embeds the committed UI build)
	go build -o $(BINARY) ./cmd/kaizen

test: ui-test ## Go tests with the race detector, plus the UI tests
	go test -race ./...

lint: ## go vet, golangci-lint and the UI typecheck
	go vet ./...
	golangci-lint run ./...
	cd $(UI_DIR) && npm run typecheck

clean: ## Remove the local binary
	rm -f $(BINARY)
