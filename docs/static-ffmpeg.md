# Static FFmpeg

`media` links the system FFmpeg by default, found with `pkg-config` (`libavcodec libavformat libavutil libswscale`, plus `libva` on Linux). That is what distribution packages want.

To ship a binary that does not depend on the user's FFmpeg, build the minimal static FFmpeg and link it with the `ffmpeg_static` build tag:

```sh
./scripts/ffmpeg-static.sh                      # a few minutes; once per platform
cd media && go test -tags ffmpeg_static ./...
go build -tags ffmpeg_static ./yourapp
```

## What the script builds

`scripts/ffmpeg-static.sh` downloads pinned releases of FFmpeg and dav1d, checks their sha256 sums, and installs static libraries (`-fPIC`) into `build/ffmpeg/<goos>-<goarch>` (gitignored), or into `$FFMPEG_PREFIX`.

- FFmpeg is configured with `--disable-everything --disable-autodetect --disable-programs --disable-doc --disable-network`, then enables only what `media` uses: the `file` protocol, the `mov` (mp4) and `matroska` (mkv, webm) demuxers, the H.264, H.265, AV1, VP8 and VP9 parsers and decoders, the `h264_mp4toannexb` and `hevc_mp4toannexb` bitstream filters, and `swscale`.
- Hardware decoding per platform: VA-API on Linux, D3D11VA on Windows, VideoToolbox on macOS. The script fails if configure drops the platform's hwaccel.
- AV1: FFmpeg's native `av1` decoder decodes only through a hwaccel. Software AV1 comes from dav1d (BSD-2-Clause), built static alongside.
- Linux links `libva` and `libva-drm` dynamically: libva loads the VA drivers at run time, so it must stay a shared system library. With `--disable-autodetect` nothing else is pulled in (no zlib, xlib, or libdrm).

Native builds only: Linux (gcc or clang), macOS (Xcode clang), Windows from an MSYS2 MINGW64 or UCRT64 shell. The script needs `curl`, `make`, `pkg-config`, `meson`, `ninja`, and on x86 `nasm` (without it, it builds without assembly, which decodes much slower). CI runs it on all three (`ffmpeg-static` in `.github/workflows/ci.yml`).

## How the tag finds it

`media/cgo_static.go` (`//go:build ffmpeg_static`) adds `-I` and `-L` for `${SRCDIR}/../build/ffmpeg/<goos>-<goarch>`, then the libraries and the system libraries the fixed configure needs. It does not use `pkg-config`: Go runs `$PKG_CONFIG` without arguments, so `pkg-config --static` cannot be requested, and the static link line must not depend on the environment. The script prints `pkg-config --static --libs` at the end to compare against `cgo_static.go` after a version bump.

For a prefix elsewhere, or when `media` comes from the module cache (no `build/` beside it), put the prefix first. Go places `CGO_CFLAGS` and `CGO_LDFLAGS` before the file's flags:

```sh
FFMPEG_PREFIX=/opt/ffmpeg-static ./scripts/ffmpeg-static.sh
export CGO_CFLAGS=-I/opt/ffmpeg-static/include CGO_LDFLAGS=-L/opt/ffmpeg-static/lib
go build -tags ffmpeg_static ./yourapp
```

Go does not hash the static libraries into its build cache: after rebuilding them, run tests with `-count=1` and rebuild binaries with `-a` (or `go clean -cache`).

To check the result on Linux, `ldd` on the binary must list `libva.so` and no `libavcodec`; CI checks this on every platform (`ldd`, `objdump -p`, `otool -L`).

## LGPL obligations

The configure flags never include `--enable-gpl` or `--enable-nonfree` (keep it that way), so FFmpeg is LGPL 2.1 or later; the script aborts if `config.h` reports another licence. A binary that statically links it must let users relink against a modified FFmpeg. When you distribute one:

- Offer the exact FFmpeg source used (the pinned release tarball from ffmpeg.org) and `scripts/ffmpeg-static.sh` with its configure flags.
- Offer your application's object files or source, so it can be relinked; giokit itself is MIT.
- State in your notices that the binary uses FFmpeg under the LGPL, with its licence text (`COPYING.LGPLv2.1` in the tarball) and dav1d's BSD-2-Clause licence.

Dynamic linking (the default build) avoids the relinking requirement.
