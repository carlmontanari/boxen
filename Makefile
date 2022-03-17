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
	docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.43.0 golangci-lint run --timeout 5m -v

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'