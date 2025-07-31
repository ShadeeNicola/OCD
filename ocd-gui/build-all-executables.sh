#!/bin/bash
echo "Building OCD for all platforms..."

# Windows
go build -ldflags="-s -w" -o OCD.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o OCD-Tool-Linux-x64 .
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o OCD-Tool-Linux-x86 .
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o OCD-Tool-Linux-ARM64 .
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o OCD-Tool-Linux-ARM .

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o OCD-Tool-macOS-Intel .
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o OCD-Tool-macOS-AppleSilicon .

echo "Build complete!"