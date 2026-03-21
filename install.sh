#!/usr/bin/env bash
# ghx v2 installer — builds from source or downloads prebuilt binary
set -euo pipefail

REPO="gkoreli/ghx"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

mkdir -p "$INSTALL_DIR"

# Try prebuilt binary from GitHub releases first
install_binary() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac

  local url="https://github.com/$REPO/releases/latest/download/ghx_${os}_${arch}"
  if curl -sfL --head "$url" >/dev/null 2>&1; then
    echo "Downloading ghx binary ($os/$arch)..."
    curl -sfL "$url" -o "$INSTALL_DIR/ghx"
    chmod +x "$INSTALL_DIR/ghx"
    return 0
  fi
  return 1
}

# Fallback: build from source
install_source() {
  if ! command -v go &>/dev/null; then
    echo "❌ No prebuilt binary available and Go not found. Install Go first: https://go.dev/dl/"
    exit 1
  fi
  echo "Building ghx from source..."
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  git clone --depth 1 "https://github.com/$REPO.git" "$tmpdir/ghx"
  (cd "$tmpdir/ghx/v2" && go build -o "$INSTALL_DIR/ghx" .)
}

echo "Installing ghx to $INSTALL_DIR..."
install_binary || install_source

# Copy skill files next to binary
for f in SKILL.md MCP-SKILL.md; do
  curl -sfL "https://raw.githubusercontent.com/$REPO/mainline/v2/$f" -o "$INSTALL_DIR/$f" 2>/dev/null || true
done

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️  $INSTALL_DIR is not in PATH. Add it:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "✅ ghx installed. Run: ghx --help"
fi
