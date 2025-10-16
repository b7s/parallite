#!/bin/bash

set -e

echo "=== Parallite Build Script ==="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check Go installation
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

echo -e "${BLUE}Go version:${NC}"
go version
echo ""

# Install dependencies
echo -e "${BLUE}Installing dependencies...${NC}"
go mod download
echo -e "${GREEN}✓ Dependencies installed${NC}"
echo ""

# Build for current platform
echo -e "${BLUE}Building for current platform...${NC}"
go build -o parallite main.go
echo -e "${GREEN}✓ Built: parallite${NC}"
echo ""

# Optional: Cross-compile for all platforms
if [ "$1" == "--cross-compile" ]; then
    echo -e "${BLUE}Cross-compiling for all platforms...${NC}"
    
    echo "Building for Linux (amd64)..."
    GOOS=linux GOARCH=amd64 go build -o parallite-linux-amd64 main.go
    echo -e "${GREEN}✓ Built: parallite-linux-amd64${NC}"
    
    echo "Building for Linux (arm64)..."
    GOOS=linux GOARCH=arm64 go build -o parallite-linux-arm64 main.go
    echo -e "${GREEN}✓ Built: parallite-linux-arm64${NC}"
    
    echo "Building for macOS (amd64)..."
    GOOS=darwin GOARCH=amd64 go build -o parallite-darwin-amd64 main.go
    echo -e "${GREEN}✓ Built: parallite-darwin-amd64${NC}"
    
    echo "Building for macOS (arm64)..."
    GOOS=darwin GOARCH=arm64 go build -o parallite-darwin-arm64 main.go
    echo -e "${GREEN}✓ Built: parallite-darwin-arm64${NC}"
    
    echo "Building for Windows (amd64)..."
    GOOS=windows GOARCH=amd64 go build -o parallite-windows-amd64.exe main.go
    echo -e "${GREEN}✓ Built: parallite-windows-amd64.exe${NC}"
    
    echo ""
    echo -e "${GREEN}All binaries built successfully!${NC}"
    ls -lh parallite-*
else
    echo "Tip: Run with --cross-compile to build for all platforms"
fi

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo ""
echo "To run the daemon:"
echo "  ./parallite"
echo ""
echo "To run with custom config:"
echo "  ./parallite --config parallite.json"
echo ""
