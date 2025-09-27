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

VERSION_TAG="dev"
if [[ -n "${CI_COMMIT_TAG:-}" ]]; then
    VERSION_TAG="${CI_COMMIT_TAG}"
elif [[ -n "${GITHUB_REF_NAME:-}" ]]; then
    VERSION_TAG="${GITHUB_REF_NAME}"
elif [[ -n "${GITHUB_REF:-}" ]]; then
    VERSION_TAG="${GITHUB_REF##*/}"
fi
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "local")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Cross-compilation relies on pure Go builds; disable CGO unless explicitly enabled.
CGO_ENABLED=${CGO_ENABLED:-0}
export CGO_ENABLED

LDFLAGS="-s -w -X app/internal/version.Version=$VERSION_TAG -X app/internal/version.Commit=$COMMIT_SHA -X app/internal/version.Date=$BUILD_DATE"

echo "Building OCD for all platforms..."
echo "Module root: $MOD_ROOT"

echo "Building Windows executable..."
# resource.syso is committed in app/ and picked up automatically by Go on Windows targets
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD.exe" ./cmd/ocd-gui

echo "Building Linux executables..."
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-Linux-x64" ./cmd/ocd-gui
GOOS=linux GOARCH=386 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-Linux-x86" ./cmd/ocd-gui
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-Linux-ARM64" ./cmd/ocd-gui
GOOS=linux GOARCH=arm GOARM=${GOARM:-7} go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-Linux-ARM" ./cmd/ocd-gui

echo "Building macOS executables..."
GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-macOS-Intel" ./cmd/ocd-gui
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o "$DIST_DIR/OCD-Tool-macOS-AppleSilicon" ./cmd/ocd-gui


echo "Build complete!"
echo "Generated executables in repo '$DIST_DIR_REL' ($DIST_DIR)"

popd >/dev/null
