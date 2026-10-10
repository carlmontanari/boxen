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
	hadolint build/profile-overlay.Dockerfile

## Run unit tests
test:
	bash build/check-install.sh
	go test ./...

## Run unit tests with race flag
test-race:
	bash build/check-install.sh
	go test ./... -race

## Build the boxen binary
.PHONY: build
build:
	GOOS=linux GOARCH=amd64 go build -trimpath -a -o dist/boxen cmd/main.go

## Build the base boxen agent container image
BOXEN_IMAGE ?= ghcr.io/carlmontanari/boxen:0.0.0
VERSION ?= 0.0.0
.PHONY: build-image
build-image:
	docker build \
        -f build/agent.Dockerfile \
        --build-arg VERSION=$(VERSION) \
        --build-arg GO_VERSION=$(GO_VERSION) \
        -t "$(BOXEN_IMAGE)" .
        # .	\
        # --platform \
        # linux/amd64 .

## Rebuild Boxen runtime and profile without repackaging the VM disk
.PHONY: rebuild-profile-image
rebuild-profile-image: build-image
	@test -n "$(SOURCE_IMAGE)" || { echo "SOURCE_IMAGE is required" >&2; exit 1; }
	@test -n "$(PROFILE_FILE)" || { echo "PROFILE_FILE is required" >&2; exit 1; }
	@test -n "$(TARGET_IMAGE)" || { echo "TARGET_IMAGE is required" >&2; exit 1; }
	@test -f "$(PROFILE_FILE)" || { echo "Profile file not found: $(PROFILE_FILE)" >&2; exit 1; }
	docker image inspect "$(SOURCE_IMAGE)" >/dev/null
	docker build --pull=false --network=none \
		--build-arg SOURCE_IMAGE="$(SOURCE_IMAGE)" \
		--build-arg BOXEN_IMAGE="$(BOXEN_IMAGE)" \
		--build-arg PROFILE_FILE="$(PROFILE_FILE)" \
		-f build/profile-overlay.Dockerfile \
		-t "$(TARGET_IMAGE)" .

##@ Documentation
UV ?= uv
DOCS_ADDR ?= 127.0.0.1:8000
DOCS_REPO ?= carlmontanari/boxen
DOCS_REF ?= main

## Install pinned uv locally without changing shell configuration
.PHONY: install-uv
install-uv:
	@mkdir -p .tools/bin
	curl --fail --location --silent --show-error \
		https://astral.sh/uv/$(UV_VERSION)/install.sh --output .tools/uv-install.sh
	UV_UNMANAGED_INSTALL="$(CURDIR)/.tools/bin" sh .tools/uv-install.sh

## Ensure uv is available for documentation commands
.PHONY: docs-tools
docs-tools:
	@command -v "$(UV)" >/dev/null 2>&1 || $(MAKE) install-uv

.PHONY: docs docs-build
## Build the documentation (alias for docs-build)
docs: docs-build
## Build the documentation, failing on broken links
docs-build: docs-tools
	$(UV) run --locked --group docs zensical build --clean --strict

## Preview the documentation with live reload (DOCS_ADDR overrides the address)
.PHONY: docs-serve
docs-serve: docs-tools
	$(UV) run --locked --group docs zensical serve --dev-addr "$(DOCS_ADDR)"

## Publish the committed DOCS_REF through the Cloudflare workflow (requires gh)
.PHONY: docs-publish
docs-publish: docs-build
	gh workflow run docs.yaml --repo "$(DOCS_REPO)" --ref "$(DOCS_REF)"
