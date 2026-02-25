.DEFAULT_GOAL := help

## Show this help
help:
	@awk -f build/makefile-doc.awk $(MAKEFILE_LIST)

##@ Development
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
fmt:
	buf format . -w
	gofumpt -w .
	gci write --skip-generated .
	golines --base-formatter="gofmt" -w .

## Run the linters
lint:
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
        -t ghcr.io/carlmontanari/boxen:dev-latest .
        # .	\
        # --platform \
        # linux/amd64 .
