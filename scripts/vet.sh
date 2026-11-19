#!/bin/bash
# go vet for the root module. gio/ is upstream Gio's code, which uses
# unkeyed struct literals throughout and syscall-style unsafe.Pointer
# conversions on Windows, so it is vetted without those two checks.
set -euo pipefail
cd "$(dirname "$0")/.."
go vet "$@" $(go list "$@" ./... | grep -v '/giokit/gio\(/\|$\)')
go vet -composites=false -unsafeptr=false "$@" ./gio/...
