#!/usr/bin/env bash
# ghx installer — downloads prebuilt binary or builds from source
set -euo pipefail

REPO="gkoreli/ghx"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

mkdir -p "$INSTALL_DIR"

install_binary() {
  local os arch ext="tar.gz"
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac
  [[ "$os" == "windows" ]] && ext="zip"

  local url="https://github.com/$REPO/releases/latest/download/ghx_${os}_${arch}.${ext}"
  if curl -sfL --head "$url" >/dev/null 2>&1; then
    echo "Downloading ghx ($os/$arch)..."
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT
    curl -sfL "$url" -o "$tmpdir/ghx.tar.gz"
    tar xzf "$tmpdir/ghx.tar.gz" -C "$tmpdir"
    cp "$tmpdir/ghx" "$INSTALL_DIR/ghx"
    chmod +x "$INSTALL_DIR/ghx"
    # Copy skill files if present in archive
    for f in SKILL.md MCP-SKILL.md; do
      [[ -f "$tmpdir/$f" ]] && cp "$tmpdir/$f" "$INSTALL_DIR/$f"
    done
    return 0
  fi
  return 1
}

install_source() {
  if ! command -v go &>/dev/null; then
    echo "❌ No prebuilt binary and Go not found. Install Go: https://go.dev/dl/"
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

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️  $INSTALL_DIR is not in PATH. Add it:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "✅ ghx installed. Run: ghx --help"
fi
