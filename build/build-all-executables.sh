#!/bin/bash
set -euo pipefail

# Root-level script that builds all platform binaries into repo-level dist/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MOD_ROOT="$REPO_ROOT/app"

# Build from inside the module to avoid Go module discovery issues on Windows
pushd "$MOD_ROOT" >/dev/null

DIST_DIR_REL=${DIST_DIR:-dist}
DIST_DIR="$REPO_ROOT/$DIST_DIR_REL"
mkdir -p "$DIST_DIR"

CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

echo "Building OCD for all platforms..."
echo "Module root: $MOD_ROOT"

echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD.exe" ./cmd/ocd-gui

echo "Building Linux executables..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-x64" ./cmd/ocd-gui
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-x86" ./cmd/ocd-gui
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-ARM64" ./cmd/ocd-gui
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-ARM" ./cmd/ocd-gui

echo "Building macOS executables..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-macOS-Intel" ./cmd/ocd-gui
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-macOS-AppleSilicon" ./cmd/ocd-gui

echo "Build complete!"
echo "Generated executables in repo '$DIST_DIR_REL' ($DIST_DIR)"

popd >/dev/null
