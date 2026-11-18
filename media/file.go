package media

/*
#include <stdlib.h>
#include "av.h"
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

// File reads the video packets of a media file (MP4, MKV, WebM, raw
// streams...). H.264 and H.265 come out in Annex B form with the parameter
// sets as a Config packet first: the same shape a live encoder sends, so a
// file can stand in for a device in tests and demos.
//
// A File has one owner: call Read from one goroutine at a time, and Close
// it when done.
type File struct {
	// Stream metadata, fixed when the file opens.
	Codec         Codec
	Width, Height int     // coded picture size in pixels
	FPS           float64 // 0 if the container does not say

	m      *C.gk_demux // nil after Close
	config []byte
}

// OpenFile opens path and reads its first video stream's metadata.
func OpenFile(path string) (*File, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var e C.int
	m := C.gk_demux_open(cp, &e)
	if m == nil {
		return nil, averr("open "+path, e)
	}
	f := &File{m: m}
	var codec, w, h, num, den C.int
	C.gk_demux_info(m, &codec, &w, &h, &num, &den)
	f.Codec, f.Width, f.Height = Codec(codec), int(w), int(h)
	if den > 0 {
		f.FPS = float64(num) / float64(den)
	}
	if f.Codec == 0 {
		f.Close()
		return nil, fmt.Errorf("media: open %s: unsupported video codec", path)
	}
	n := C.gk_demux_config(m, nil, 0)
	if n > 0 {
		f.config = make([]byte, n)
		C.gk_demux_config(m, (*C.uint8_t)(unsafe.Pointer(&f.config[0])), n)
	}
	return f, nil
}

// Config is the codec configuration packet, or nil if the codec carries it
// in band. The slice is shared: do not modify it. It stays valid after
// Close.
func (f *File) Config() []byte { return f.config }

// Read returns the next video packet, io.EOF at the end, or ErrClosed after
// Close. The packet's Data is a fresh copy owned by the caller.
func (f *File) Read() (Packet, error) {
	if f.m == nil {
		return Packet{}, ErrClosed
	}
	var data *C.uint8_t
	var pts C.int64_t
	var key C.int
	n := C.gk_demux_read(f.m, &data, &pts, &key)
	if n < 0 {
		return Packet{}, averr("read", n)
	}
	if n == 0 {
		return Packet{}, io.EOF
	}
	return Packet{
		Data: C.GoBytes(unsafe.Pointer(data), n),
		PTS:  int64(pts),
		Key:  key != 0,
	}, nil
}

// Close closes the file. Calling it again does nothing.
func (f *File) Close() {
	if f.m != nil {
		C.gk_demux_close(f.m)
		f.m = nil
	}
}
