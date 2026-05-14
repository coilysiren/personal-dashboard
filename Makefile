.PHONY: build test run fmt vet tidy help

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "};{printf "%-12s %s\n",$$1,$$2}'

build: ## Compile the daemon binary to ./personal-dashboard.
	go build -o personal-dashboard ./cmd/personal-dashboard

test: ## Run the full Go test suite.
	go test ./...

run: ## Start the daemon locally on 127.0.0.1:31337 (loopback only).
	go run ./cmd/personal-dashboard

dev: ## Watch sources and auto-rebuild + restart the daemon on change.
	go run ./cmd/dev --addr=127.0.0.1:31337

fmt: ## Format Go sources in place.
	go fmt ./...

vet: ## Run go vet across the module.
	go vet ./...

tidy: ## Tidy go.mod and go.sum.
	go mod tidy
