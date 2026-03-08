#!/usr/bin/env bash
# ghx installer — downloads the script to ~/.local/bin
set -euo pipefail

REPO="gkoreli/ghx"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

mkdir -p "$INSTALL_DIR"

echo "Installing ghx to $INSTALL_DIR..."

if command -v gh &>/dev/null; then
  # Use gh for authenticated download (handles private repos too)
  gh api "repos/$REPO/contents/ghx" -H "Accept: application/vnd.github.raw+json" > "$INSTALL_DIR/ghx"
else
  curl -sfL "https://raw.githubusercontent.com/$REPO/main/ghx" -o "$INSTALL_DIR/ghx"
fi

chmod +x "$INSTALL_DIR/ghx"

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️  $INSTALL_DIR is not in PATH. Add it:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "✅ ghx installed. Run: ghx --help"
fi
