#!/usr/bin/env bash
# ggcode installer — downloads the script to ~/.local/bin
set -euo pipefail

REPO="gkoreli/ggcode"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

mkdir -p "$INSTALL_DIR"

echo "Installing ggcode to $INSTALL_DIR..."

if command -v gh &>/dev/null; then
  # Use gh for authenticated download (handles private repos too)
  gh api "repos/$REPO/contents/ggcode" -H "Accept: application/vnd.github.raw+json" > "$INSTALL_DIR/ggcode"
else
  curl -sfL "https://raw.githubusercontent.com/$REPO/main/ggcode" -o "$INSTALL_DIR/ggcode"
fi

chmod +x "$INSTALL_DIR/ggcode"

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️  $INSTALL_DIR is not in PATH. Add it:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "✅ ggcode installed. Run: ggcode --help"
fi
