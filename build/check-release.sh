#!/usr/bin/env bash
set -euo pipefail

release_tag=${1:?Usage: bash build/check-release.sh TAG [OUTPUT_DIR]}
release_version=${release_tag#v}
output_dir=${2:-dist/release}
native_os=$(go env GOOS)
native_arch=$(go env GOARCH)

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

(cd "$output_dir" && sha256sum --check "boxen_${release_version}_checksums.txt")

for os in linux darwin; do
  for arch in amd64 arm64; do
    archive="$output_dir/boxen_${release_version}_${os}_${arch}.tar.gz"
    tar -xzf "$archive" -C "$build_dir" boxen LICENSE README.md
    metadata=$(go version -m "$build_dir/boxen")
    printf '%s\n' "$metadata" | grep -Fx $'\tbuild\tGOOS='"$os"
    printf '%s\n' "$metadata" | grep -Fx $'\tbuild\tGOARCH='"$arch"
    if [[ "$os" == "$native_os" && "$arch" == "$native_arch" ]]; then
      "$build_dir/boxen" --version | grep -Fx "boxen version $release_version"
      "$build_dir/boxen" build --help >/dev/null
    fi
  done
done
