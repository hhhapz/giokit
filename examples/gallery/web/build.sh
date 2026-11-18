#!/bin/sh
# Builds the gallery for the browser into this folder. Afterwards the folder
# is the whole site (index.html, wasm_exec.js, gallery.wasm, plus .br copies
# for hosts that serve precompressed files with Content-Encoding: br; the
# originals are the fallback): copy it to any static host. Preview locally with:
#
#   python3 -m http.server -d examples/gallery/web 8080
set -e
cd "$(dirname "$0")"
command -v brotli >/dev/null || { echo "brotli is needed to compress the build" >&2; exit 1; }
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o gallery.wasm ..
# Guard against a host build slipping through under the wasm name.
head -c 4 gallery.wasm | od -An -c | grep -q 'a   s   m' || { echo "gallery.wasm is not WebAssembly" >&2; exit 1; }
root=$(go env GOROOT)
cp "$root/lib/wasm/wasm_exec.js" . 2>/dev/null || cp "$root/misc/wasm/wasm_exec.js" .
brotli --best --keep --force gallery.wasm wasm_exec.js index.html
echo "built $(pwd)"
