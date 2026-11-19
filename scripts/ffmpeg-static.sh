#!/usr/bin/env bash
# Build the minimal static FFmpeg (plus dav1d) that `go build -tags
# ffmpeg_static` links into the media module. See docs/static-ffmpeg.md.
#
#	./scripts/ffmpeg-static.sh            # into build/ffmpeg/<goos>-<goarch>
#	FFMPEG_PREFIX=/opt/ff ./scripts/ffmpeg-static.sh
#
# Native builds only: Linux (gcc/clang), macOS (Xcode clang), Windows from an
# MSYS2 MINGW64/UCRT64 shell. Needs curl, make, pkg-config, meson, ninja and,
# on x86, nasm. Sources are downloaded once into build/ffmpeg/src and checked
# against the pinned sha256 sums below.
#
# Licence: FFmpeg is built LGPL (never --enable-gpl or --enable-nonfree; the
# script refuses a GPL config). Whoever ships a binary linked with these libs
# must offer the FFmpeg source and this script.
set -euo pipefail

FFMPEG_VERSION=9.0.2
FFMPEG_SHA256=8c3850283eb25fa026482078a04051e0be17347b09ef81a0849bec15a96e002e
DAV1D_VERSION=1.5.5
DAV1D_SHA256=ca96b1d0ba3f01f55de7231d775892b64994ba061aaeeaaa4f662a390b3abf9e

root=$(cd "$(dirname "$0")/.." && pwd)
goos=$(go env GOOS)
goarch=$(go env GOARCH)
prefix=${FFMPEG_PREFIX:-$root/build/ffmpeg/$goos-$goarch}
src=${FFMPEG_SRC:-$root/build/ffmpeg/src}
jobs=$(nproc 2>/dev/null || sysctl -n hw.ncpu)

sha256() {
	if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1
}

# fetch URL SHA256: download into $src once, verify, extract.
fetch() {
	local url=$1 sum=$2 file
	file=$src/$(basename "$url")
	mkdir -p "$src"
	if [ ! -f "$file" ] || [ "$(sha256 "$file")" != "$sum" ]; then
		curl -fsSL -o "$file.part" "$url"
		mv "$file.part" "$file"
	fi
	if [ "$(sha256 "$file")" != "$sum" ]; then
		echo "ffmpeg-static: $file: sha256 mismatch" >&2
		exit 1
	fi
	tar -C "$src" -xf "$file"
}

asm=1
case $goarch in
amd64 | 386)
	if ! command -v nasm >/dev/null; then
		echo "ffmpeg-static: nasm not found: building without x86 assembly (slow decoding)" >&2
		asm=0
	fi
	;;
esac

mkdir -p "$prefix"
rm -rf "$src/dav1d-$DAV1D_VERSION" "$src/ffmpeg-$FFMPEG_VERSION"

# dav1d: the software AV1 decoder (BSD-2-Clause). FFmpeg's own "av1"
# decoder decodes only through a hwaccel.
fetch "https://downloads.videolan.org/pub/videolan/dav1d/$DAV1D_VERSION/dav1d-$DAV1D_VERSION.tar.xz" "$DAV1D_SHA256"
dav1d_asm=true
[ $asm = 1 ] || dav1d_asm=false
meson setup "$src/dav1d-$DAV1D_VERSION/build" "$src/dav1d-$DAV1D_VERSION" \
	--prefix="$prefix" --libdir=lib --buildtype=release \
	--default-library=static -Db_staticpic=true \
	-Denable_tools=false -Denable_tests=false -Denable_examples=false \
	-Denable_asm=$dav1d_asm
ninja -C "$src/dav1d-$DAV1D_VERSION/build" install

fetch "https://ffmpeg.org/releases/ffmpeg-$FFMPEG_VERSION.tar.xz" "$FFMPEG_SHA256"

# Only what media uses: mp4/mkv/webm files, H.264, H.265, AV1, VP8, VP9.
flags=(
	--prefix="$prefix" --libdir="$prefix/lib" --incdir="$prefix/include"
	--enable-static --disable-shared --enable-pic
	--disable-programs --disable-doc --disable-network --disable-autodetect
	--disable-everything --disable-avdevice --disable-avfilter --disable-swresample
	--pkg-config-flags=--static
	--enable-protocol=file
	--enable-demuxer=mov,matroska
	--enable-parser=h264,hevc,av1,vp8,vp9
	--enable-decoder=h264,hevc,av1,vp8,vp9,libdav1d
	--enable-bsf=h264_mp4toannexb,hevc_mp4toannexb
	--enable-libdav1d
)
[ $asm = 1 ] || flags+=(--disable-x86asm)
case $goos in
linux)
	# libva stays a shared system library: it loads the VA drivers at run
	# time, so it cannot be linked statically.
	flags+=(--enable-pthreads --enable-vaapi
		--enable-hwaccel=h264_vaapi,hevc_vaapi,av1_vaapi,vp8_vaapi,vp9_vaapi)
	hwaccel=CONFIG_H264_VAAPI_HWACCEL
	;;
windows)
	flags+=(--enable-w32threads --enable-d3d11va
		--enable-hwaccel=h264_d3d11va,h264_d3d11va2,hevc_d3d11va,hevc_d3d11va2,av1_d3d11va,av1_d3d11va2,vp9_d3d11va,vp9_d3d11va2)
	hwaccel=CONFIG_H264_D3D11VA2_HWACCEL
	;;
darwin)
	flags+=(--enable-pthreads --enable-videotoolbox
		--enable-hwaccel=h264_videotoolbox,hevc_videotoolbox,av1_videotoolbox,vp9_videotoolbox)
	hwaccel=CONFIG_H264_VIDEOTOOLBOX_HWACCEL
	;;
*)
	echo "ffmpeg-static: unsupported GOOS $goos" >&2
	exit 1
	;;
esac

cd "$src/ffmpeg-$FFMPEG_VERSION"
PKG_CONFIG_PATH="$prefix/lib/pkgconfig" ./configure "${flags[@]}"
if ! grep -q '^#define FFMPEG_LICENSE "LGPL' config.h; then
	echo "ffmpeg-static: configured licence is not LGPL:" >&2
	grep FFMPEG_LICENSE config.h >&2
	exit 1
fi
# configure drops a component whose dependencies are missing: fail instead.
for want in CONFIG_LIBDAV1D_DECODER CONFIG_H264_DECODER CONFIG_HEVC_DECODER $hwaccel; do
	grep -qs "^#define $want 1" config.h config_components.h || { echo "ffmpeg-static: $want disabled" >&2; exit 1; }
done
make -j"$jobs"
make install
cd "$root"
rm -rf "$src/dav1d-$DAV1D_VERSION" "$src/ffmpeg-$FFMPEG_VERSION"

echo
echo "Static FFmpeg $FFMPEG_VERSION installed in $prefix. Link it with:"
echo "  (cd media && go test -tags ffmpeg_static ./...)"
echo "Libraries it needs (for comparison with media/cgo_static.go):"
PKG_CONFIG_PATH="$prefix/lib/pkgconfig" pkg-config --static --libs libavformat libavcodec libswscale libavutil
if [ "$prefix" != "$root/build/ffmpeg/$goos-$goarch" ]; then
	echo "Outside the default prefix, also set:"
	echo "  export CGO_CFLAGS=\"-I$prefix/include\" CGO_LDFLAGS=\"-L$prefix/lib\""
fi
