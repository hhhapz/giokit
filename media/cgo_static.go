//go:build ffmpeg_static

package media

// With -tags ffmpeg_static, media links the static FFmpeg and dav1d that
// scripts/ffmpeg-static.sh installs in build/ffmpeg/<goos>-<goarch> at the
// repository root. For a prefix elsewhere (or when media comes from the
// module cache), put its include and lib directories first with
// CGO_CFLAGS=-I<prefix>/include CGO_LDFLAGS=-L<prefix>/lib.
//
// The system libraries below are what the script's fixed configure needs
// (it disables autodetection); its last lines print pkg-config's list to
// compare against. libva stays shared on Linux: it loads drivers at run time.

/*
#cgo linux,amd64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/linux-amd64/include
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/linux-amd64/lib
#cgo linux,arm64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/linux-arm64/include
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/linux-arm64/lib
#cgo windows,amd64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/windows-amd64/include
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/windows-amd64/lib
#cgo windows,arm64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/windows-arm64/include
#cgo windows,arm64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/windows-arm64/lib
#cgo darwin,amd64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/darwin-amd64/include
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/darwin-amd64/lib
#cgo darwin,arm64 CFLAGS: -I${SRCDIR}/../build/ffmpeg/darwin-arm64/include
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../build/ffmpeg/darwin-arm64/lib

#cgo LDFLAGS: -lavformat -lavcodec -lswscale -lavutil -ldav1d
#cgo linux pkg-config: libva libva-drm
#cgo linux LDFLAGS: -ldl -lm -latomic -lpthread
#cgo windows LDFLAGS: -lbcrypt -lole32 -luser32
#cgo darwin LDFLAGS: -framework VideoToolbox -framework CoreMedia -framework CoreVideo -framework CoreFoundation -framework CoreServices -framework QuartzCore -lm
*/
import "C"
