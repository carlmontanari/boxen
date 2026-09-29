.DEFAULT_GOAL := help
include .github/vars.env
export PATH := $(CURDIR)/.tools/bin:$(PATH)

## Show this help
help:
	@awk -f build/makefile-doc.awk $(MAKEFILE_LIST)

##@ Development
## Install pinned development tools
.PHONY: tools
tools:
	bash build/install-tools.sh

## Run protoc generation bits
generate:
	rm proto/v1/*.pb.go || true
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		proto/v1/*.proto

## Run the formatters
fmt: tools
	buf format . -w
	gofumpt -w .
	gci write --skip-generated .
	golines --base-formatter="gofmt" -w .

## Check formatting without changing files
.PHONY: fmt-check
fmt-check: tools
	bash build/check-format.sh

## Run the linters
lint: fmt-check
	go mod tidy -diff
	buf lint .
	golangci-lint run
	hadolint build/agent.Dockerfile

## Run unit tests
test:
	go test ./...

## Run unit tests with race flag
test-race:
	go test ./... -race

## Build the boxen binary
.PHONY: build
build:
	GOOS=linux GOARCH=amd64 go build -trimpath -a -o dist/boxen cmd/main.go

## Build the base boxen agent container image
build-image:
	docker build \
        -f build/agent.Dockerfile \
        --build-arg GO_VERSION=$(GO_VERSION) \
        -t ghcr.io/carlmontanari/boxen:0.0.0 .
        # .	\
        # --platform \
        # linux/amd64 .
