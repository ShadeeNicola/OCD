#!/bin/bash
set -euo pipefail

echo "Building OCD for all platforms..."

# Resolve script directory and project root so the script works from any CWD
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Output directory (configurable via DIST_DIR env var)
DIST_DIR_REL=${DIST_DIR:-dist}
DIST_DIR="$PROJECT_ROOT/$DIST_DIR_REL"

# Create output directory if it doesn't exist
mkdir -p "$DIST_DIR"

# Detect current platform
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

echo "Current platform: $CURRENT_OS ($CURRENT_ARCH)"

# Windows
echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD.exe" "$PROJECT_ROOT/cmd/ocd-gui"

# Linux
echo "Building Linux executables..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-x64" "$PROJECT_ROOT/cmd/ocd-gui"
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-x86" "$PROJECT_ROOT/cmd/ocd-gui"
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-ARM64" "$PROJECT_ROOT/cmd/ocd-gui"
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-Linux-ARM" "$PROJECT_ROOT/cmd/ocd-gui"

# macOS
echo "Building macOS executables..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-macOS-Intel" "$PROJECT_ROOT/cmd/ocd-gui"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$DIST_DIR/OCD-Tool-macOS-AppleSilicon" "$PROJECT_ROOT/cmd/ocd-gui"

# Make Linux and macOS executables executable (if not on Windows)
if [[ "$CURRENT_OS" != msys_nt* && "$CURRENT_OS" != cygwin* ]]; then
    echo "Setting executable permissions for Linux and macOS binaries..."
    chmod +x "$DIST_DIR"/OCD-Tool-Linux* || true
    chmod +x "$DIST_DIR"/OCD-Tool-macOS* || true
fi

# Create a native executable for current platform if not Windows
if [[ "$CURRENT_OS" == "linux" ]]; then
    echo "Creating native Linux executable..."
    go build -ldflags="-s -w" -o "$DIST_DIR/OCD" "$PROJECT_ROOT/cmd/ocd-gui"
    chmod +x "$DIST_DIR/OCD"
elif [[ "$CURRENT_OS" == "darwin" ]]; then
    echo "Creating native macOS executable..."
    go build -ldflags="-s -w" -o "$DIST_DIR/OCD" "$PROJECT_ROOT/cmd/ocd-gui"
    chmod +x "$DIST_DIR/OCD"
fi

echo "Build complete!"
echo ""
echo "Generated executables in '$DIST_DIR_REL' folder (at $DIST_DIR):"
ls -la "$DIST_DIR"/OCD* 2>/dev/null | grep -E "(OCD\.exe|OCD-Tool-|OCD$)" || echo "No executables generated. Check above logs for errors."


