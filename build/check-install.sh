#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT
export TEST_ROOT="$test_root"
export REAL_INSTALL
REAL_INSTALL=$(command -v install)
mkdir -p "$test_root/bin" "$test_root/assets"
printf 'a test CLI binary\n' > "$test_root/assets/boxen"

# Mock only the network, platform, privilege, and destination boundaries.
cat > "$test_root/mock" <<'SH'
#!/bin/sh
set -eu
case "${0##*/}" in
  uname)
    case "$1" in -s) echo "$TEST_OS" ;; -m) echo "$TEST_ARCH" ;; esac ;;
  id) echo "$TEST_UID" ;;
  mktemp)
    mkdir -p "$TEST_ROOT/download"
    echo "$TEST_ROOT/download" ;;
  curl)
    printf '%s\n' "$*" >> "$TEST_ROOT/curl.log"
    url= output=
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --output) output=$2; shift ;;
        https://*) url=$1 ;;
      esac
      shift
    done
    if [ "$url" = https://github.com/carlmontanari/boxen/releases/latest ]; then
      [ -z "${BOXEN_TAG:-}" ]
      echo "https://github.com/carlmontanari/boxen/releases/tag/$TEST_TAG"
    else
      [ "$url" = "https://github.com/carlmontanari/boxen/releases/download/$TEST_TAG/${url##*/}" ]
      [ "${TEST_FAILURE:-}" != download ] || exit 22
      cp "$TEST_ROOT/assets/${url##*/}" "$output"
    fi ;;
  sudo)
    [ "$TEST_UID" != 0 ]
    echo sudo >> "$TEST_ROOT/sudo.log"
    "$@" ;;
  install)
    printf '%s\n' "$*" >> "$TEST_ROOT/install.log"
    case "$1" in
      -d) [ "$2" = /usr/local/bin ] ;;
      -m)
        [ "$2" = 0755 ]
        [ "$4" = /usr/local/bin/boxen ]
        "$REAL_INSTALL" -m "$2" "$3" "$TEST_ROOT/installed" ;;
      *) exit 1 ;;
    esac ;;
esac
SH
chmod +x "$test_root/mock"
for tool in uname id mktemp curl sudo install; do
  ln -s "$test_root/mock" "$test_root/bin/$tool"
done
for tool in sh mkdir cp rm awk tar gzip; do
  ln -s "$(command -v "$tool")" "$test_root/bin/$tool"
done
for tool in shasum sha256sum; do
  if tool_path=$(command -v "$tool"); then
    ln -s "$tool_path" "$test_root/bin/$tool"
  fi
done

export TEST_OS=Linux TEST_ARCH=x86_64 TEST_UID=1000 TEST_TAG=v0.0.4 BOXEN_TAG= TEST_FAILURE=

check_install() {
  local expected_status=$1 os=$2 arch=$3 version=${TEST_TAG#v}
  local archive="boxen_${version}_${os}_${arch}.tar.gz"
  rm -f "$test_root/installed" "$test_root/install.log" "$test_root/sudo.log" "$test_root/curl.log"
  rm -f "$test_root/assets/"*.tar.gz "$test_root/assets/"*_checksums.txt
  tar -czf "$test_root/assets/$archive" -C "$test_root/assets" boxen
  if [[ "$TEST_FAILURE" == extraction ]]; then
    printf 'invalid archive\n' > "$test_root/assets/$archive"
  fi
  (
    cd "$test_root/assets"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum "$archive"
    else
      shasum -a 256 "$archive"
    fi
  ) > "$test_root/assets/boxen_${version}_checksums.txt"
  if [[ "$TEST_FAILURE" == checksum ]]; then
    printf 'tampered archive\n' > "$test_root/assets/$archive"
  elif [[ "$TEST_FAILURE" == missing-checksum ]]; then
    : > "$test_root/assets/boxen_${version}_checksums.txt"
  fi

  local status=0
  cat "$repo_root/install.sh" | PATH="$test_root/bin" sh - > "$test_root/output" 2>&1 || status=$?
  if [[ "$expected_status" == success ]]; then
    [[ "$status" == 0 ]] || { cat "$test_root/output"; exit 1; }
    cmp "$test_root/assets/boxen" "$test_root/installed"
    if [[ "$TEST_UID" == 0 ]]; then
      [[ ! -f "$test_root/sudo.log" ]]
    else
      [[ -f "$test_root/sudo.log" ]]
    fi
  else
    [[ "$status" != 0 && ! -f "$test_root/install.log" && ! -f "$test_root/installed" ]]
  fi
  [[ ! -d "$test_root/download" ]]
}

for TEST_OS in Linux Darwin; do
  case "$TEST_OS" in Linux) os=linux ;; Darwin) os=darwin ;; esac
  for TEST_ARCH in x86_64 amd64 aarch64 arm64; do
    case "$TEST_ARCH" in x86_64|amd64) arch=amd64 ;; *) arch=arm64 ;; esac
    check_install success "$os" "$arch"
  done
done
TEST_UID=0 TEST_TAG=v0.0.4-rc.1 BOXEN_TAG=v0.0.4-rc.1 check_install success darwin arm64
if command -v shasum >/dev/null 2>&1; then
  rm -f "$test_root/bin/sha256sum"
  check_install success darwin arm64
fi
for TEST_FAILURE in checksum missing-checksum download extraction; do
  check_install failure darwin arm64
done
TEST_FAILURE= TEST_OS=FreeBSD check_install failure darwin arm64
TEST_FAILURE= TEST_ARCH=riscv64 check_install failure darwin arm64
echo 'Installer checks passed (platforms, pinned release, checksums, failures, privileges, cleanup).'
