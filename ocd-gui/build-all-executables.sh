#!/bin/bash
echo "Building OCD for all platforms..."

# Create executables directory if it doesn't exist
mkdir -p executables

# Detect current platform
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

echo "Current platform: $CURRENT_OS ($CURRENT_ARCH)"

# Windows
echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o executables/OCD.exe .

# Linux
echo "Building Linux executables..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o executables/OCD-Tool-Linux-x64 .
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o executables/OCD-Tool-Linux-x86 .
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o executables/OCD-Tool-Linux-ARM64 .
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o executables/OCD-Tool-Linux-ARM .

# macOS
echo "Building macOS executables..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o executables/OCD-Tool-macOS-Intel .
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o executables/OCD-Tool-macOS-AppleSilicon .

# Make Linux and macOS executables executable (if not on Windows)
if [[ "$CURRENT_OS" != "msys_nt"* ]] && [[ "$CURRENT_OS" != "cygwin"* ]]; then
    echo "Setting executable permissions for Linux and macOS binaries..."
    chmod +x executables/OCD-Tool-Linux*
    chmod +x executables/OCD-Tool-macOS*
fi

# Create a native executable for current platform if not Windows
if [[ "$CURRENT_OS" == "linux" ]]; then
    echo "Creating native Linux executable..."
    go build -ldflags="-s -w" -o executables/OCD .
    chmod +x executables/OCD
elif [[ "$CURRENT_OS" == "darwin" ]]; then
    echo "Creating native macOS executable..."
    go build -ldflags="-s -w" -o executables/OCD .
    chmod +x executables/OCD
fi

echo "Build complete!"
echo ""
echo "Generated executables in 'executables' folder:"
ls -la executables/OCD* | grep -E "(OCD\.exe|OCD-Tool-|OCD$)"