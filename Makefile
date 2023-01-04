.DEFAULT_GOAL := help
BIN_DIR = $$(pwd)/bin
BINARY = $$(pwd)/bin/boxen

build: ## Build boxen
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) -ldflags="-s -w" main.go

lint: ## Run linters
	gofmt -w -s .
	goimports -w .
	golines -w .
	golangci-lint run

docker-lint: ## Run linters with docker
	docker run -it --rm -v $$(pwd):/work ghcr.io/hellt/golines:0.8.0 golines -w .
	docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.45.0 golangci-lint run --timeout 5m -v

test: ## Run unit tests
	gotestsum --format testname --hide-summary=skipped -- -coverprofile=cover.out ./...

test-race: ## Run unit tests with race flag
	gotestsum --format testname --hide-summary=skipped -- -coverprofile=cover.out ./... -race

ttl-push: build ## push locally built binary to ttl.sh container registry
	docker run --rm -v $$(pwd)/bin:/workspace ghcr.io/oras-project/oras:v0.12.0 push ttl.sh/boxen-$$(git rev-parse --short HEAD):1d ./boxen
	@echo "download with: docker run --rm -v \$$(pwd):/workspace ghcr.io/oras-project/oras:v0.12.0 pull ttl.sh/boxen-$$(git rev-parse --short HEAD):1d"

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'