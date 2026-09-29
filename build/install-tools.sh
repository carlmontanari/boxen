#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# The same version file is sourced by GitHub Actions before setup-go.
source "$repo_root/.github/vars.env"

if [[ $(go env GOVERSION) != "go${GO_VERSION}" ]]; then
  echo "Go ${GO_VERSION} is required; found $(go env GOVERSION)" >&2
  exit 1
fi

bin_dir="$repo_root/.tools/bin"
stamp="$repo_root/.tools/vars.env"
mkdir -p "$bin_dir"
if [[ -f "$stamp" ]] && cmp -s "$repo_root/.github/vars.env" "$stamp" &&
  [[ -x "$bin_dir/buf" && -x "$bin_dir/hadolint" && -x "$bin_dir/golangci-lint" &&
     -x "$bin_dir/gofumpt" && -x "$bin_dir/gci" && -x "$bin_dir/golines" ]]; then
  exit 0
fi

os=$(go env GOOS)
arch=$(go env GOARCH)
case "$os/$arch" in
  linux/amd64) buf_platform=Linux-x86_64; hadolint_platform=linux-x86_64 ;;
  linux/arm64) buf_platform=Linux-aarch64; hadolint_platform=linux-arm64 ;;
  darwin/amd64) buf_platform=Darwin-x86_64; hadolint_platform=macos-x86_64 ;;
  darwin/arm64) buf_platform=Darwin-arm64; hadolint_platform=macos-arm64 ;;
  *) echo "Unsupported tool platform: $os/$arch" >&2; exit 1 ;;
esac

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

download() {
  curl --fail --location --silent --show-error --retry 3 "$1" --output "$2"
}

verify() {
  local expected
  expected=$(awk -v asset="$2" '{name=$2; sub(/^\*/, "", name); sub(/^.*\//, "", name); if (name == asset) {print $1; exit}}' "$1")
  if [[ -z "$expected" ]]; then
    echo "No checksum found for $2" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null; then
    printf '%s  %s\n' "$expected" "$3" | sha256sum --check --status
  else
    printf '%s  %s\n' "$expected" "$3" | shasum -a 256 --check --status
  fi
}

golangci_asset="golangci-lint-${GOLANGCI_LINT_VERSION#v}-${os}-${arch}.tar.gz"
golangci_url="https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_LINT_VERSION}"
download "$golangci_url/$golangci_asset" "$tmp_dir/$golangci_asset"
download "$golangci_url/golangci-lint-${GOLANGCI_LINT_VERSION#v}-checksums.txt" "$tmp_dir/golangci-checksums.txt"
verify "$tmp_dir/golangci-checksums.txt" "$golangci_asset" "$tmp_dir/$golangci_asset"
tar -xzf "$tmp_dir/$golangci_asset" -C "$tmp_dir"
install -m 755 "$tmp_dir/golangci-lint-${GOLANGCI_LINT_VERSION#v}-${os}-${arch}/golangci-lint" "$bin_dir/golangci-lint"

buf_asset="buf-${buf_platform}"
buf_url="https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}"
download "$buf_url/$buf_asset" "$tmp_dir/$buf_asset"
download "$buf_url/sha256.txt" "$tmp_dir/buf-checksums.txt"
verify "$tmp_dir/buf-checksums.txt" "$buf_asset" "$tmp_dir/$buf_asset"
install -m 755 "$tmp_dir/$buf_asset" "$bin_dir/buf"

hadolint_asset="hadolint-${hadolint_platform}"
hadolint_url="https://github.com/hadolint/hadolint/releases/download/${HADOLINT_VERSION}"
download "$hadolint_url/$hadolint_asset" "$tmp_dir/$hadolint_asset"
download "$hadolint_url/checksums.sha256" "$tmp_dir/hadolint-checksums.txt"
verify "$tmp_dir/hadolint-checksums.txt" "$hadolint_asset" "$tmp_dir/$hadolint_asset"
install -m 755 "$tmp_dir/$hadolint_asset" "$bin_dir/hadolint"

GOBIN="$bin_dir" go install "mvdan.cc/gofumpt@${GOFUMPT_VERSION}"
GOBIN="$bin_dir" go install "github.com/daixiang0/gci@${GCI_VERSION}"
GOBIN="$bin_dir" go install "github.com/golangci/golines@${GOLINES_VERSION}"

cp "$repo_root/.github/vars.env" "$stamp"
echo "Installed pinned tools in $bin_dir"
