#!/bin/bash
# giokit's Gio fork as a patch series against upstream.
#
#   scripts/gio-fork.sh diff         regenerate third_party/patches from the
#                                    fork (after editing third_party/gioui.org);
#                                    the old series is replaced only once every
#                                    patch was written
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
# The C locale keeps file order, and so the patches, the same on every machine.
export LC_ALL=C
base=v0.10.3
root=$(cd "$(dirname "$0")/.." && pwd)
fork=$root/third_party/gioui.org
patches=$root/third_party/patches

series=(
	"0001-vulkan:internal/vk/vulkan.go gpu/headless/headless.go"
	"0002-wayland-cursor-wheel-trace:app/os_wayland.go app/window.go app/giokit_scroll.go app/giokit_scroll_test.go app/giokit_trace.go"
	"0003-label-clips-to-text:widget/label.go"
	"0004-source-bounds-focus:io/input/giokit_bounds.go"
	"0005-semantic-image:io/semantic/semantic.go"
	"0006-editor-keys:widget/editor.go widget/editor_test.go widget/text.go"
	"0007-paint-stream-dmabuf:gpu/gpu.go gpu/stream_test.go gpu/internal/driver/driver.go gpu/internal/driver/api.go gpu/internal/opengl/opengl.go gpu/internal/opengl/dmabuf_linux.go gpu/internal/vulkan/vulkan.go gpu/internal/vulkan/dmabuf.go internal/vk/giokit_dmabuf.go app/vulkan.go gpu/headless/headless_vulkan.go internal/stream op/paint/stream.go"
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

udiff() { # the unified diff from $1 to $2, labelled $3; fails only on trouble
	local status=0
	diff -uN --label "a/$3" --label "b/$3" "$1" "$2" || status=$?
	# diff exits 1 when the files differ and 2 on trouble (a missing file).
	[ "$status" -le 1 ]
}

case ${1:-} in
diff)
	up=$(upstream $base)
	tmp=$(mktemp -d "$root/third_party/.patches.XXXXXX")
	trap 'rm -rf "$tmp"' EXIT
	for e in "${series[@]}"; do
		name=${e%%:*}.patch
		out=$tmp/$name
		: >"$out"
		for f in $(files "$e"); do
			a=$up/$f
			[ -e "$a" ] || a=/dev/null
			udiff "$a" "$fork/$f" "$f" >>"$out"
		done
		[ -s "$out" ] || echo "warning: $name is empty; drop it from the series" >&2
		echo "$name: $(grep -c '^[+-][^+-]' "$out" || true) lines"
	done
	chmod 755 "$tmp"
	rm -rf "$patches"
	mv "$tmp" "$patches"
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
