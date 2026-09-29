#!/usr/bin/env bash
set -euo pipefail

check_empty() {
  local output
  output=$("$@")
  if [[ -n "$output" ]]; then
    printf '%s\n' "$output"
    return 1
  fi
}

buf format . --diff --exit-code
check_empty gofumpt -l .
check_empty gci list --skip-generated .
check_empty golines --base-formatter=gofmt --dry-run .
