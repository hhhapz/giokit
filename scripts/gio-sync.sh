#!/bin/bash
# giokit's hard fork of Gio lives in gio/ as github.com/hhhapz/giokit/gio/...
# Pristine upstream enters history only as snapshot commits ("gio: upstream
# ..."), each changing nothing but gio/ and parented on the previous
# snapshot. The newest one reachable from HEAD is the base: bumping Gio is a
# merge of a new snapshot on top of it, and our changes are the commits
# after it. No branch is needed; never squash a snapshot commit away.
#
#   scripts/gio-sync.sh update [gio] [shader] [x] [cmd]
#        commit a snapshot of upstream at these versions (default: the
#        base's) on top of the base, and merge it into HEAD. Versions are
#        anything go mod download takes: v0.11.0, a commit hash.
#   scripts/gio-sync.sh pick <sha>
#        apply one upstream Gio commit to HEAD (git am, rewritten)
#   scripts/gio-sync.sh export [rev-range] [outdir]
#        format our gio/ commits (default: base..HEAD) as patches against
#        upstream paths, ready for git send-email to
#        ~eliasnaur/gio-patches@lists.sr.ht
#   scripts/gio-sync.sh check
#        verify the base's gio/ is exactly the snapshot of its versions, and
#        that no module requires upstream gioui.org
#
# The rewrite is one text substitution on Go files, "gioui.org/ -> "$path/
# (and gogio's -X gioui.org/ linker flags), so trees and patches rewrite
# identically. Doc comments keep upstream names.
set -euo pipefail
export LC_ALL=C
root=$(cd "$(dirname "$0")/.." && pwd)
path=github.com/hhhapz/giokit/gio
subject="gio: upstream" # snapshot commits' subject prefix

# What each upstream module contributes: module, destination under gio/,
# the paths kept (shader's piet compute shaders and most of x are unused),
# and tar patterns excluded from them. From cmd only gogio, the app
# packager, without its tests: they are end-to-end runs needing browsers,
# emulators and chromedp.
modules=(
	"gioui.org|.|*|"
	"gioui.org/shader|shader|LICENSE README.md shader.go gio cmd|"
	"gioui.org/x|x|LICENSE styledtext|"
	"gioui.org/cmd|cmd|LICENSE gogio|*_test.go gogio/internal"
)

rewrite() { # stdin -> stdout: upstream import paths to ours
	sed -e "s#\"gioui.org/#\"$path/#g" -e "s#\"gioui.org\"#\"$path\"#g" -e "s#-X gioui.org/#-X $path/#g"
}
unrewrite() {
	sed -e "s#\"$path/#\"gioui.org/#g" -e "s#\"$path\"#\"gioui.org\"#g" -e "s#-X $path/#-X gioui.org/#g"
}

base() { # the newest snapshot commit reachable from HEAD
	local b
	b=$(git -C "$root" log -1 --format=%H --grep="^$subject " HEAD)
	[ -n "$b" ] || { echo "no \"$subject ...\" snapshot commit in HEAD's history" >&2; exit 1; }
	echo "$b"
}

versions() { # "gio shader x cmd" from commit $1's gio/UPSTREAM
	git -C "$root" show "$1:gio/UPSTREAM" | awk '{print $2}' | paste -sd' '
}

download() { # module version -> its directory in the module cache
	(cd "$root" && GOFLAGS=-mod=mod go mod download -json "$1@$2") |
		sed -n 's/^\t"Dir": "\(.*\)",$/\1/p'
}

snapshot() { # dest gio shader x cmd: write the rewritten upstream tree to dest
	local dest=$1 m mod sub keep skip dir v f p
	shift
	rm -rf "$dest" && mkdir -p "$dest"
	: >"$dest/UPSTREAM"
	for m in "${modules[@]}"; do
		IFS='|' read -r mod sub keep skip <<<"$m"
		local excludes=()
		for p in $skip; do excludes+=("--exclude=$p"); done
		v=${1:-}
		[ $# -gt 0 ] && shift
		[ -n "$v" ] || continue # not in this snapshot (an older base)
		dir=$(download "$mod" "$v")
		[ -d "$dir" ] || { echo "cannot download $mod@$v" >&2; exit 1; }
		# The resolved version: a commit hash becomes its pseudo-version.
		v=$(basename "$dir")
		v=${v#*@}
		echo "$mod $v" >>"$dest/UPSTREAM"
		mkdir -p "$dest/$sub"
		if [ "$keep" = "*" ]; then
			(cd "$dir" && tar -c --exclude=go.mod --exclude=go.sum --exclude='flake.*' --exclude=.builds --exclude=.gitattributes .) | tar -x -C "$dest/$sub"
		else
			(cd "$dir" && tar -c "${excludes[@]}" $keep) | tar -x -C "$dest/$sub"
		fi
		# The module cache is read-only, and tar keeps its modes.
		chmod -R u+w "$dest"
	done
	find "$dest" -name '*.go' -print0 | while IFS= read -r -d '' f; do
		rewrite <"$f" >"$f.tmp" && mv "$f.tmp" "$f"
	done
	# Upstream's module subdirectories (shader, x) must not keep go.mod files
	# either, or they would split back into modules.
	find "$dest" -mindepth 2 \( -name go.mod -o -name go.sum \) -delete
}

case ${1:-} in
update)
	shift
	if [ -n "$(git -C "$root" status --porcelain)" ]; then
		echo "commit or stash your changes first: update merges into HEAD" >&2
		exit 1
	fi
	b=$(base)
	read -r g s x c <<<"$(versions "$b")"
	g=${1:-$g} s=${2:-$s} x=${3:-$x} c=${4:-$c}
	wt=$(mktemp -d)
	trap 'git -C "$root" worktree remove --force "$wt" 2>/dev/null || true; rm -rf "$wt"' EXIT
	git -C "$root" worktree add -q --detach "$wt" "$b"
	snapshot "$wt/gio" "$g" "$s" "$x" "$c"
	git -C "$wt" add -A gio
	if git -C "$wt" diff --cached --quiet; then
		echo "the base already is upstream $g, shader $s, x $x, cmd $c"
		exit 0
	fi
	what=$(awk '{print $1 "@" $2}' "$wt/gio/UPSTREAM" | paste -sd' ')
	git -C "$wt" commit -qm "$subject $what"
	snap=$(git -C "$wt" rev-parse HEAD)
	# On conflicts, git stops here: resolve them in gio/, keeping our
	# changes where upstream did not take them, and commit.
	git -C "$root" merge --no-ff -m "gio: merge $what" "$snap"
	;;
pick)
	sha=${2:?usage: pick <sha>}
	curl -fsSL "https://git.sr.ht/~eliasnaur/gio/commit/$sha.patch" | rewrite |
		git -C "$root" am --directory=gio --exclude=go.mod --exclude=go.sum
	;;
export)
	range=${2:-$(base)..HEAD}
	out=${3:-$root/gio-patches}
	mkdir -p "$out"
	for p in $(git -C "$root" format-patch --relative=gio -o "$out" "$range" -- gio/); do
		unrewrite <"$p" >"$p.tmp" && mv "$p.tmp" "$p"
		echo "$p"
	done
	;;
check)
	b=$(base)
	read -r g s x c <<<"$(versions "$b")"
	if [ "$(versions HEAD)" != "$(versions "$b")" ]; then
		echo "gio/UPSTREAM says $(versions HEAD), but the base $b is $(versions "$b")" >&2
		exit 1
	fi
	if [ -n "$(git -C "$root" diff --name-only "$b^" "$b" -- . ':!gio/')" ]; then
		echo "the base $b changes files outside gio/" >&2
		exit 1
	fi
	tmp=$(mktemp -d)
	trap 'chmod -R u+w "$tmp"; rm -rf "$tmp"' EXIT
	snapshot "$tmp/gio" "$g" "$s" "$x" "$c"
	mkdir "$tmp/base"
	git -C "$root" archive "$b" gio | tar -x -C "$tmp/base"
	diff -r "$tmp/gio" "$tmp/base/gio" >/dev/null || {
		echo "the base $b is not the snapshot of $(versions "$b")" >&2
		diff -rq "$tmp/gio" "$tmp/base/gio" >&2 || true
		exit 1
	}
	if grep -n 'gioui\.org' "$root/go.mod" "$root/media/go.mod"; then
		echo "a module requires upstream Gio: two Gios cannot link into one binary" >&2
		exit 1
	fi
	echo "ok: base $(git -C "$root" rev-parse --short "$b") is upstream $(versions "$b")"
	;;
*)
	sed -n '2,/^set -euo/p' "$0" | sed '$d; s/^# \{0,1\}//'
	exit 2
	;;
esac
