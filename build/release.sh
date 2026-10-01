#!/usr/bin/env bash
set -euo pipefail

release_tag=${1:?Usage: bash build/release.sh TAG [OUTPUT_DIR]}
release_version=${release_tag#v}
output_dir=${2:-dist/release}
mkdir -p "$output_dir"

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

checksums="$output_dir/boxen_${release_version}_checksums.txt"
: > "$checksums"

for os in linux darwin; do
  for arch in amd64 arm64; do
    archive="boxen_${release_version}_${os}_${arch}.tar.gz"
    echo "Building $archive"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
      -trimpath \
      -ldflags "-s -w -X github.com/carlmontanari/boxen/constants.Version=$release_version" \
      -o "$build_dir/boxen" ./cmd
    tar -czf "$output_dir/$archive" -C "$build_dir" boxen -C "$PWD" LICENSE README.md
    (cd "$output_dir" && sha256sum "$archive") >> "$checksums"
  done
done
