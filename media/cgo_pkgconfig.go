//go:build !ffmpeg_static

package media

// The default build links the system FFmpeg, found with pkg-config (and
// libva on Linux). Build with -tags ffmpeg_static to link the static
// FFmpeg from scripts/ffmpeg-static.sh instead (cgo_static.go).

/*
#cgo pkg-config: libavcodec libavformat libavutil libswscale
#cgo linux pkg-config: libva
*/
import "C"
