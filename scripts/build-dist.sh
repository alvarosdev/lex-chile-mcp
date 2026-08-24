#!/usr/bin/env bash
# build-dist.sh — builds the cross-platform distributions of the server.
#
# Usage: scripts/build-dist.sh <version>   (e.g. 1.2.0)
#
# Compiles the 6-target matrix (linux/darwin/windows × amd64/arm64),
# each binary is self-contained via go:embed (endpoints and prompts baked
# into the binary, no external config), generates SHA256SUMS.txt and packs
# everything into dist.zip.
#
# This is the SAME script the CI workflow runs: local = CI.
set -euo pipefail

VERSION="${1:?usage: build-dist.sh <version>}"

command -v sha256sum >/dev/null || { echo "✗ 'sha256sum' is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "✗ 'python3' is required (used for zipping)" >&2; exit 1; }

cd "$(dirname "$0")/.."
DIST="dist"
BINARY="lex-chile-mcp"

rm -rf "$DIST"
mkdir -p "$DIST"

# target matrix: os/arch — darwin/amd64 is Intel (pre-Apple Silicon); Go
# calls x86-64 "amd64" (x64 is the same ISA under a different name).
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

echo "Building version $VERSION for ${#TARGETS[@]} targets..."
for t in "${TARGETS[@]}"; do
  os="${t%/*}"
  arch="${t#*/}"
  out="$DIST/$os/$arch/$BINARY"
  [[ "$os" == "windows" ]] && out="$out.exe"
  mkdir -p "$(dirname "$out")"

  echo "  → $os/$arch"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="-s -w -X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=${VERSION#v}" \
    -o "$out" ./cmd/lex-chile-mcp

done

echo "Generating SHA256SUMS.txt..."
(
  cd "$DIST"
  find . -type f -name "$BINARY*" | sort | xargs sha256sum > SHA256SUMS.txt
)

echo "Packing dist.zip..."
rm -f dist.zip
python3 - "$DIST" <<'PYEOF'
import sys, zipfile, os
dist = sys.argv[1]
with zipfile.ZipFile("dist.zip", "w", zipfile.ZIP_DEFLATED) as zf:
    for root, _, files in os.walk(dist):
        for name in sorted(files):
            path = os.path.join(root, name)
            zf.write(path, path)
PYEOF

echo "✓ dist.zip ready ($(du -h dist.zip | cut -f1))"
