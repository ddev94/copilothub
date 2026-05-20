#!/bin/bash

# Release script for pock
set -e

# Get version from version.go
VERSION=$(grep 'const Version' pkg/version/version.go | sed 's/.*"\(.*\)".*/\1/')

echo "========================================="
echo "Creating GitHub Release v${VERSION}"
echo "========================================="
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed."
    echo ""
    echo "Install it with:"
    echo "  brew install gh"
    echo ""
    exit 1
fi

# Check if user is authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub."
    echo ""
    echo "Run: gh auth login"
    echo ""
    exit 1
fi

# Check if tag already exists
if git rev-parse "v${VERSION}" >/dev/null 2>&1; then
    echo "⚠️  Tag v${VERSION} already exists."
    read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "v${VERSION}"
        git push origin ":refs/tags/v${VERSION}" 2>/dev/null || true
    else
        echo "Aborted."
        exit 1
    fi
fi

# Build release artifacts
echo "▶ Building release artifacts..."
make release

echo ""
echo "▶ Creating git tag v${VERSION}..."
git tag -a "v${VERSION}" -m "Release v${VERSION}"

echo "▶ Pushing tag to GitHub..."
git push origin "v${VERSION}"

echo ""
echo "▶ Creating GitHub release..."
gh release create "v${VERSION}" \
    bin/copilothub-darwin-amd64 \
    bin/copilothub-darwin-arm64 \
    bin/copilothub-linux-amd64 \
    bin/copilothub-windows-amd64.exe \
    --title "v${VERSION}" \
    --notes "Release v${VERSION}

## Quick Install

### macOS/Linux
\`\`\`bash
curl -o- https://raw.githubusercontent.com/azoom-nguyen-thanh-duong/spec-designer/main/install.sh | bash
\`\`\`

Or with wget:
\`\`\`bash
wget -qO- https://raw.githubusercontent.com/azoom-nguyen-thanh-duong/spec-designer/main/install.sh | bash
\`\`\`

## Manual Download

Download the binary for your platform:
- \`copilothub-darwin-amd64\` - macOS Intel
- \`copilothub-darwin-arm64\` - macOS Apple Silicon
- \`copilothub-linux-amd64\` - Linux 64-bit
- \`copilothub-windows-amd64.exe\` - Windows 64-bit

Make it executable and move to your PATH:
\`\`\`bash
chmod +x copilothub-*
sudo mv copilothub-* /usr/local/bin/copilothub
\`\`\`

## What's Changed
- Bug fixes and improvements"

echo ""
echo "========================================="
echo "✓ Release v${VERSION} published!"
echo "========================================="
echo ""
echo "View at: $(gh release view "v${VERSION}" --json url -q .url)"
