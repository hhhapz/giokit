#!/bin/sh
# Builds the gallery for the browser into this folder. Afterwards the folder
# is the whole site (index.html, wasm_exec.js, gallery.wasm): copy it to any
# static host. Preview locally with:
#
#   python3 -m http.server -d examples/gallery/web 8080
set -e
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o gallery.wasm ..
root=$(go env GOROOT)
cp "$root/lib/wasm/wasm_exec.js" . 2>/dev/null || cp "$root/misc/wasm/wasm_exec.js" .
echo "built $(pwd)"
