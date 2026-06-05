#!/bin/bash
set -euo pipefail

# Automatic build and run script for hydra
# This script builds the Go binary and then runs it with the provided arguments

# Check if Go is installed
if ! command -v go &>/dev/null; then
    echo "[ERROR] Go is not installed" >&2
    echo "" >&2
    echo "To install Go, you can use brew:" >&2
    echo "  brew install go" >&2
    echo "" >&2
    echo "For more information, visit: https://golang.org/doc/install" >&2
    exit 1
fi

# Resolve symlinks to get the real script directory (works on Linux and macOS)
if command -v readlink >/dev/null 2>&1 && readlink -f / >/dev/null 2>&1; then
    # GNU readlink (Linux)
    SCRIPT_DIR="$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"
else
    # BSD readlink (macOS) or fallback
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
fi

BINARY_NAME="hydra"
CLI_DIR="$SCRIPT_DIR/cli"
BINARY_PATH="${SCRIPT_DIR}/${BINARY_NAME}"

echo "[BUILD] Building hydra Go binary..." >&2

# Build the Go binary from cli module
if ! go build -C "$CLI_DIR" -o "${BINARY_PATH}" .; then
    echo "[ERROR] Failed to build Go binary" >&2
    exit 1
fi

echo "[BUILD] Build successful, running hydra..." >&2

exec "$BINARY_PATH" "$@"
