#!/bin/bash
# Build and test giokit against stock Gio (no replace onto third_party):
# kit, kit/ui, kit/x, kittest and examples/hello must not need the fork.
# The fork-only packages (kit/forktrace, kit/x/video) and the examples that
# import them are excluded. Run from the repository root.
#
# The working tree is tested as it is: tracked files with their uncommitted
# changes, plus untracked files git does not ignore (new code is included),
# minus deleted files. GOFLAGS from the environment is kept; -mod=mod is
# added so go.sum can gain stock Gio's entries in the copy.
set -euo pipefail
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
git ls-files -z --cached --others --exclude-standard -- . ':!third_party/' |
	while IFS= read -r -d '' f; do
		if [ -e "$f" ]; then printf '%s\0' "$f"; fi
	done |
	xargs -0 cp --parents -t "$dir"
cd "$dir"
sed -i '/^replace gioui.org/d' go.mod
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-mod=mod"
# Stock Gio enables Vulkan on Linux, which needs its headers; novulkan keeps
# the build to OpenGL like the fork's default.
go build -tags novulkan ./kit/ ./kit/ui/ ./kit/x/ ./kit/fonts/ ./kittest/... ./examples/hello/
go test -tags novulkan ./kit/ ./kit/ui/ ./kit/x/ ./kit/fonts/ ./kittest/... ./internal/lint/ ./examples/hello/
