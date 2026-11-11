#!/usr/bin/env bash
set -euo pipefail

# Single owner of the golangci-lint gate: CI (.github/workflows/ci.yml) and
# local developers run this same script so the pinned version, the toolchain
# selection, and the lint invocation cannot drift apart.
GOLANGCI_LINT_VERSION="v2.12.2"

ROOT="${ISSUEOPS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

usage() {
  cat <<'EOF'
Usage: scripts/lint-local.sh [--print-version]

Run the CI golangci-lint gate locally: `golangci-lint run ./...` built with the
Go toolchain selected from go.mod. Off Linux it also runs the
GOOS=linux CGO_ENABLED=0 variant, so platform-specific findings (for example
syscall.Stat_t field types) surface before CI does. Off Windows it also runs the
GOOS=windows CGO_ENABLED=0 variant, so findings in *_windows.go files surface
on the Linux CI runner and on macOS.

golangci-lint is a developer and CI gate only. This script never installs it;
install the pinned version with the official path:
  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>

Options:
  --print-version  Print the pinned golangci-lint version and exit.
EOF
}

case "${1:-}" in
  --print-version)
    printf '%s\n' "$GOLANGCI_LINT_VERSION"
    exit 0
    ;;
  -h|--help)
    usage
    exit 0
    ;;
  "")
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "golangci-lint not found on PATH; install ${GOLANGCI_LINT_VERSION}:" >&2
  echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" >&2
  exit 1
fi

# `golangci-lint version` prints "... has version <v>[ built with ...". Release
# binaries omit the leading "v" that `go install` builds carry, so compare
# without it.
version_line="$(golangci-lint version 2>&1 | head -n 1)"
installed="$(sed -n 's/.* has version v\{0,1\}\([^ ]*\).*/\1/p' <<<"$version_line")"
if [ "$installed" != "${GOLANGCI_LINT_VERSION#v}" ]; then
  echo "golangci-lint version mismatch: CI pins ${GOLANGCI_LINT_VERSION}, found '${installed:-unknown}' (${version_line})" >&2
  echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" >&2
  exit 1
fi

cd "$ROOT"

# Build with the toolchain selected from go.mod so a newer ambient GOROOT
# cannot break the analyzers.
GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')"
export GOTOOLCHAIN

status=0
echo "==> golangci-lint run ./... (GOOS=$(go env GOOS))"
golangci-lint run ./... || status=1

if [ "$(go env GOOS)" != "linux" ]; then
  # The cross run only type-checks linux sources. When CGO_ENABLED=1 is set
  # explicitly in the environment, the linux-only cgo files of the standard
  # library (net, os/user) cannot be preprocessed for another GOOS and
  # typecheck fails. This repository imports no "C", so the cross run disables
  # cgo.
  echo "==> golangci-lint run ./... (GOOS=linux)"
  GOOS=linux CGO_ENABLED=0 golangci-lint run ./... || status=1
fi

if [ "$(go env GOOS)" != "windows" ]; then
  # CI lints on Linux, so *_windows.go files are never analyzed there without
  # this run. A Windows host already analyzes them in the first step. cgo stays
  # disabled for the same reason as the linux cross run above.
  echo "==> golangci-lint run ./... (GOOS=windows)"
  GOOS=windows CGO_ENABLED=0 golangci-lint run ./... || status=1
fi

exit "$status"
