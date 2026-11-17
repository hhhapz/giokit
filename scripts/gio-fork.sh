#!/bin/bash
# giokit's Gio fork as a patch series against upstream.
#
#   scripts/gio-fork.sh diff         regenerate third_party/patches from the
#                                    fork (after editing third_party/gioui.org)
#   scripts/gio-fork.sh apply [ver]  rebuild third_party/gioui.org from
#                                    upstream gioui.org@ver (default: the
#                                    base below) plus the patches, in order:
#                                    bumping Gio is "apply vX.Y.Z", then fix
#                                    any patch that no longer applies
#   scripts/gio-fork.sh check        apply to a temporary copy and verify it
#                                    reproduces third_party/gioui.org exactly
#
# Each patch is one concern; its files belong to no other patch.
set -euo pipefail
base=v0.10.3
root=$(cd "$(dirname "$0")/.." && pwd)
fork=$root/third_party/gioui.org
patches=$root/third_party/patches

series=(
	"0001-vulkan-opt-in:app/vulkan.go app/vulkan_wayland.go app/vulkan_x11.go gpu/headless/headless_vulkan.go gpu/internal/vulkan/vulkan.go"
	"0002-wayland-cursor-wheel-trace:app/os_wayland.go app/window.go app/giokit_scroll.go app/giokit_scroll_test.go app/giokit_trace.go"
	"0003-label-clips-to-text:widget/label.go"
	"0004-source-bounds-focus:io/input/giokit_bounds.go"
	"0005-semantic-image:io/semantic/semantic.go"
	"0006-editor-keys:widget/editor.go widget/editor_test.go widget/text.go"
	"0007-paint-stream-dmabuf:gpu/gpu.go gpu/internal/driver/driver.go gpu/internal/opengl/opengl.go gpu/internal/opengl/dmabuf_linux.go internal/stream op/paint/stream.go"
)

upstream() { # the pristine module for version $1
	local v=$1 dir
	dir=$(go env GOMODCACHE)/gioui.org@$v
	[ -d "$dir" ] || (cd "$root" && GOFLAGS=-mod=mod go mod download gioui.org@"$v")
	echo "$dir"
}

files() { # the files of a series entry, directories expanded
	local f
	for f in ${1#*:}; do
		if [ -d "$fork/$f" ]; then (cd "$fork" && find "$f" -type f | sort); else echo "$f"; fi
	done
}

case ${1:-} in
diff)
	up=$(upstream $base)
	rm -rf "$patches" && mkdir -p "$patches"
	for e in "${series[@]}"; do
		out=$patches/${e%%:*}.patch
		for f in $(files "$e"); do
			a=$up/$f
			[ -e "$a" ] || a=/dev/null
			diff -uN --label "a/$f" --label "b/$f" "$a" "$fork/$f" >>"$out" || true
		done
		echo "$out: $(grep -c '^[+-][^+-]' "$out") lines"
	done
	;;
apply)
	up=$(upstream "${2:-$base}")
	dest=${DEST:-$fork}
	rm -rf "$dest" && mkdir -p "$dest"
	cp -r "$up"/. "$dest" && chmod -R u+w "$dest"
	for p in "$patches"/*.patch; do
		echo "applying $(basename "$p")"
		(cd "$dest" && patch -p1 --no-backup-if-mismatch -s <"$p")
	done
	;;
check)
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	DEST=$tmp/gioui.org "$0" apply >/dev/null
	diff -r "$tmp/gioui.org" "$fork" && echo "patch series reproduces third_party/gioui.org"
	;;
*)
	sed -n '2,16p' "$0"
	exit 2
	;;
esac
