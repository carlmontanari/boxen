#!/bin/sh
(
  set -eu
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  repo=https://github.com/carlmontanari/boxen
  tag=${BOXEN_TAG:-$(curl --fail --silent --show-error --location --head \
    --output /dev/null --write-out '%{url_effective}' "$repo/releases/latest")}
  tag=${tag##*/}
  version=${tag#v}
  archive="boxen_${version}_${os}_${arch}.tar.gz"
  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' 0
  trap 'exit 1' HUP INT TERM

  curl --fail --silent --show-error --location \
    "$repo/releases/download/$tag/$archive" --output "$tmp_dir/$archive"
  curl --fail --silent --show-error --location \
    "$repo/releases/download/$tag/boxen_${version}_checksums.txt" \
    --output "$tmp_dir/checksums.txt"
  expected=$(awk -v archive="$archive" '$2 == archive {print $1; exit}' "$tmp_dir/checksums.txt")
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$tmp_dir/$archive")
  else
    actual=$(shasum -a 256 "$tmp_dir/$archive")
  fi
  if [ -z "$expected" ] || [ "${actual%% *}" != "$expected" ]; then
    echo "Checksum verification failed for $archive" >&2
    exit 1
  fi

  tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" boxen
  if [ "$(id -u)" -eq 0 ]; then
    install -d /usr/local/bin
    install -m 0755 "$tmp_dir/boxen" /usr/local/bin/boxen
  else
    sudo install -d /usr/local/bin
    sudo install -m 0755 "$tmp_dir/boxen" /usr/local/bin/boxen
  fi
  printf 'Installed boxen %s to /usr/local/bin/boxen\n' "$version"
)
