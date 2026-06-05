#!/bin/bash
set -euo pipefail

# Automatic build and test script for hydra-go
# This script builds and tests all three modules: base, core, cli

# Resolve symlinks to get the real script directory (works on Linux and macOS)
if command -v readlink >/dev/null 2>&1 && readlink -f / >/dev/null 2>&1; then
    # GNU readlink (Linux)
    SCRIPT_DIR="$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"
else
    # BSD readlink (macOS) or fallback
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
fi

cd "$SCRIPT_DIR"

# Use a writable cache location in restricted environments such as CI sandboxes.
export GOCACHE="${GOCACHE:-/tmp/codex-go-cache}"

# Install tools if not present
command -v gotest >/dev/null 2>&1 || go install github.com/rakyll/gotest@latest
command -v staticcheck >/dev/null 2>&1 || go install honnef.co/go/tools/cmd/staticcheck@latest

MODULES=("base" "core" "cli")

echo "=== Syncing workspace ==="
go work sync

for module in "${MODULES[@]}"; do
    echo ""
    echo "=== Module: $module ==="
    cd "$SCRIPT_DIR/$module"
    
    echo "Cleaning up go modules..."
    go mod tidy
    
    echo "Formatting Go files..."
    gofmt -w .
    
    echo "Running go vet..."
    go vet ./...
    
    echo "Running staticcheck..."
    XDG_CACHE_HOME="${XDG_CACHE_HOME:-/tmp/codex-cache}" staticcheck ./...
    
    echo "Running gopls check..."
    gopls check $(find . -name '*.go' -not -path './vendor/*') 2>/dev/null || true
    
    echo "Building..."
    go build ./...
    
    echo "Running tests..."
    gotest -v ./... "$@"
done

echo ""
echo "=== Building CLI binary ==="
cd "$SCRIPT_DIR/cli"
go build -o "$SCRIPT_DIR/hydra" .

echo ""
echo "=== All tests passed ==="
