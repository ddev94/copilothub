#!/bin/bash
# Spec Designer Installer for macOS and Linux
# This script downloads and installs copilothub from GitHub releases

set -e

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="copilothub"
GITHUB_REPO="azoom-nguyen-thanh-duong/copilothub"
VERSION="${1:-latest}"

echo "🚀 Spec Designer Installer"
echo "================="
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
   echo "❌ Please don't run this script as root (no sudo needed here)"
   exit 1
fi

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Darwin*)
        if [ "$ARCH" = "arm64" ]; then
            BINARY_URL_SUFFIX="copilothub-darwin-arm64"
        else
            BINARY_URL_SUFFIX="copilothub-darwin-amd64"
        fi
        ;;
    Linux*)
        if [ "$ARCH" = "x86_64" ]; then
            BINARY_URL_SUFFIX="copilothub-linux-amd64"
        else
            echo "❌ Unsupported architecture: $ARCH"
            exit 1
        fi
        ;;
    *)
        echo "❌ Unsupported operating system: $OS"
        exit 1
        ;;
esac

echo "🖥️  Detected: $OS $ARCH"
echo ""

# Determine download URL
if [ "$VERSION" = "latest" ]; then
    echo "📡 Fetching latest release..."
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/latest/download/$BINARY_URL_SUFFIX"
else
    echo "📡 Fetching version $VERSION..."
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/v$VERSION/$BINARY_URL_SUFFIX"
fi
echo $DOWNLOAD_URL
# Download binary to temp location
TEMP_DIR=$(mktemp -d)
TEMP_BINARY="$TEMP_DIR/$BINARY_NAME"

echo "⬇️  Downloading from GitHub releases..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_BINARY"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$DOWNLOAD_URL" -O "$TEMP_BINARY"
else
    echo "❌ Neither curl nor wget found. Please install one of them."
    rm -rf "$TEMP_DIR"
    exit 1
fi

if [ ! -f "$TEMP_BINARY" ]; then
    echo "❌ Download failed"
    rm -rf "$TEMP_DIR"
    exit 1
fi

echo "✓ Downloaded successfully"
echo ""

# Remove quarantine attribute if present (macOS)
if [ "$OS" = "Darwin" ]; then
    echo "🔓 Removing quarantine attribute (if present)..."
    xattr -d com.apple.quarantine "$TEMP_BINARY" 2>/dev/null || echo "   No quarantine attribute found (that's fine)"
fi

# Make executable
chmod +x "$TEMP_BINARY"

SPEC_DESIGNER_BINARY="$TEMP_BINARY"

# Check if already installed
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    CURRENT_VERSION=$("$INSTALL_DIR/$BINARY_NAME" --version 2>/dev/null || echo "unknown")
    NEW_VERSION=$("$SPEC_DESIGNER_BINARY" --version 2>/dev/null || echo "unknown")
    
    echo "⚠️  Spec Designer is already installed: $CURRENT_VERSION"
    echo "   New version: $NEW_VERSION"
    echo ""
    read -p "Do you want to replace it? [y/N] " -n 1 -r < /dev/tty
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ Installation cancelled"
        exit 0
    fi
    echo ""
fi

# Install
echo "📥 Installing to $INSTALL_DIR/$BINARY_NAME ..."
sudo cp "$SPEC_DESIGNER_BINARY" "$INSTALL_DIR/$BINARY_NAME"
sudo chmod 755 "$INSTALL_DIR/$BINARY_NAME"

# Clean up temp files
rm -rf "$TEMP_DIR"

echo ""
echo "✅ Installation complete!"
echo ""

# Verify
if command -v copilothub >/dev/null 2>&1; then
    INSTALLED_VERSION=$(copilothub --version)
    echo "🎉 $INSTALLED_VERSION"
    echo ""
    echo "Try it:"
    echo "  copilothub --help"
    echo "  copilothub open"
else
    echo "⚠️  Installation succeeded but 'copilothub' command not found in PATH"
    echo ""
    echo "Add this to your ~/.zshrc or ~/.bashrc:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "Then run: source ~/.zshrc"
fi

echo ""
echo "📚 Documentation: https://github.com/$GITHUB_REPO"
