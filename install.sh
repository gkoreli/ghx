#!/usr/bin/env bash
# ghx installer — downloads prebuilt binary or builds from source
set -euo pipefail

REPO="gkoreli/ghx"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TMPDIR_CLEANUP=""

cleanup() { [[ -n "$TMPDIR_CLEANUP" ]] && rm -rf "$TMPDIR_CLEANUP"; }
trap cleanup EXIT

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
  curl -sfL --head "$url" >/dev/null 2>&1 || return 1

  echo "Downloading ghx ($os/$arch)..."
  TMPDIR_CLEANUP="$(mktemp -d)"
  curl -sfL "$url" -o "$TMPDIR_CLEANUP/ghx.tar.gz"
  tar xzf "$TMPDIR_CLEANUP/ghx.tar.gz" -C "$TMPDIR_CLEANUP"
  cp "$TMPDIR_CLEANUP/ghx" "$INSTALL_DIR/ghx"
  chmod +x "$INSTALL_DIR/ghx"
  for f in SKILL.md MCP-SKILL.md; do
    [[ -f "$TMPDIR_CLEANUP/$f" ]] && cp "$TMPDIR_CLEANUP/$f" "$INSTALL_DIR/$f"
    [[ -f "$TMPDIR_CLEANUP/v2/$f" ]] && cp "$TMPDIR_CLEANUP/v2/$f" "$INSTALL_DIR/$f"
  done
  return 0
}

install_source() {
  command -v go &>/dev/null || { echo "❌ No prebuilt binary and Go not found. Install Go: https://go.dev/dl/"; exit 1; }
  echo "Building ghx from source..."
  TMPDIR_CLEANUP="$(mktemp -d)"
  git clone --depth 1 "https://github.com/$REPO.git" "$TMPDIR_CLEANUP/ghx"
  (cd "$TMPDIR_CLEANUP/ghx/v2" && go build -o "$INSTALL_DIR/ghx" .)
}

echo "Installing ghx to $INSTALL_DIR..."
install_binary || install_source

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo "⚠️  $INSTALL_DIR is not in PATH. Add it:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "✅ ghx installed. Run: ghx --help"
fi
