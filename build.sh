#!/bin/bash

# Build script for File Transfer application
# Builds for Windows, Linux, and Android

set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=$VERSION"

echo "Building File Transfer v$VERSION..."
echo ""

# Create output directory
mkdir -p dist

# Linux AMD64
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "dist/filetransfer-linux-amd64" .

# Linux ARM64
echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o "dist/filetransfer-linux-arm64" .

# Windows AMD64
echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "dist/filetransfer-windows-amd64.exe" .

# Windows ARM64
echo "Building for Windows ARM64..."
GOOS=windows GOARCH=arm64 go build -ldflags="$LDFLAGS" -o "dist/filetransfer-windows-arm64.exe" .

# Linux ARM (Raspberry Pi, etc.)
echo "Building for Linux ARM..."
GOOS=linux GOARCH=arm go build -ldflags="$LDFLAGS" -o "dist/filetransfer-linux-arm" .

echo ""
echo "✅ All builds complete! Binaries in ./dist/"
ls -lh dist/

echo ""
echo "Android build instructions:"
echo "  1. Install gomobile: go install golang.org/x/mobile/cmd/gomobile@latest"
echo "  2. Initialize: gomobile init"
echo "  3. Build: gomobile build -target=android -o dist/filetransfer.apk ."
