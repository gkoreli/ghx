#!/usr/bin/env bash
# Publishes platform npm packages from GoReleaser artifacts
set -euo pipefail

REPO="gkoreli/ghx"
VERSION="${1:?Usage: publish-npm.sh <version>}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# GoReleaser archive name → npm package dir
declare -A PLATFORMS=(
  ["ghx_darwin_arm64"]="ghx-darwin-arm64"
  ["ghx_darwin_amd64"]="ghx-darwin-x64"
  ["ghx_linux_arm64"]="ghx-linux-arm64"
  ["ghx_linux_amd64"]="ghx-linux-x64"
  ["ghx_windows_arm64"]="ghx-win32-arm64"
  ["ghx_windows_amd64"]="ghx-win32-x64"
)

# Binary name per platform
bin_name() { [[ "$1" == *win32* ]] && echo "ghx.exe" || echo "ghx"; }

echo "Publishing @gkoreli/ghx platform packages v${VERSION}"

for archive in "${!PLATFORMS[@]}"; do
  pkg="${PLATFORMS[$archive]}"
  ext="tar.gz"; [[ "$archive" == *windows* ]] && ext="zip"
  url="https://github.com/$REPO/releases/download/v${VERSION}/${archive}.${ext}"
  bin=$(bin_name "$pkg")

  echo "  → @gkoreli/$pkg"
  curl -sfL "$url" -o "$TMPDIR/archive"
  if [[ "$ext" == "tar.gz" ]]; then
    tar xzf "$TMPDIR/archive" -C "$TMPDIR" "$bin"
  else
    unzip -o "$TMPDIR/archive" "$bin" -d "$TMPDIR" >/dev/null
  fi

  # Copy binary into package dir and set version
  cp "$TMPDIR/$bin" "npm/$pkg/$bin"
  chmod +x "npm/$pkg/$bin"
  node -e "const p=require('./npm/$pkg/package.json'); p.version='$VERSION'; require('fs').writeFileSync('./npm/$pkg/package.json', JSON.stringify(p,null,2)+'\n')"

  # Publish from repo root so .npmrc auth is found
  npm publish "npm/$pkg" --access public --provenance

  rm -f "$TMPDIR/archive" "$TMPDIR/$bin"
done

# Update and publish main package
echo "  → @gkoreli/ghx"
node -e "
const p=require('./package.json');
p.version='$VERSION';
for (const [k] of Object.entries(p.optionalDependencies||{})) p.optionalDependencies[k]='$VERSION';
require('fs').writeFileSync('./package.json', JSON.stringify(p,null,2)+'\n');
"
npm publish --access public --provenance

echo "Done — @gkoreli/ghx@${VERSION} published with all platform packages"
