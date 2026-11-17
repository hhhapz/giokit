#!/bin/sh
# Build and test giokit against stock Gio (no replace onto third_party):
# kit, kit/ui, kit/x, kittest and examples/hello must not need the fork.
# The fork-only packages (kit/forktrace, kit/x/video) and the examples that
# import them are excluded. Run from the repository root.
set -eu
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
git ls-files -z | grep -zv '^third_party/' | xargs -0 cp --parents -t "$dir"
cd "$dir"
sed -i '/^replace gioui.org/d' go.mod
export GOFLAGS=-mod=mod
# Stock Gio enables Vulkan on Linux, which needs its headers; novulkan keeps
# the build to OpenGL like the fork's default.
go build -tags novulkan ./kit/ ./kit/ui/ ./kit/x/ ./kit/fonts/ ./kittest/... ./examples/hello/
go test -tags novulkan ./kit/ ./kit/ui/ ./kit/x/ ./kit/fonts/ ./kittest/... ./internal/lint/ ./examples/hello/
