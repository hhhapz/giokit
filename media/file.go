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
type File struct {
	m             *C.gk_demux
	Codec         Codec
	Width, Height int
	FPS           float64
	config        []byte
}

// OpenFile opens path.
func OpenFile(path string) (*File, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var e C.int
	m := C.gk_demux_open(cp, &e)
	if m == nil {
		return nil, fmt.Errorf("open %s: %w", path, averr(e))
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
		return nil, fmt.Errorf("open %s: unsupported video codec", path)
	}
	n := C.gk_demux_config(m, nil, 0)
	if n > 0 {
		f.config = make([]byte, n)
		C.gk_demux_config(m, (*C.uint8_t)(unsafe.Pointer(&f.config[0])), n)
	}
	return f, nil
}

// Config is the codec configuration packet, or nil if the codec carries it
// in band.
func (f *File) Config() []byte { return f.config }

// Read returns the next video packet, or io.EOF.
func (f *File) Read() (Packet, error) {
	var data *C.uint8_t
	var pts C.int64_t
	var key C.int
	n := C.gk_demux_read(f.m, &data, &pts, &key)
	if n < 0 {
		return Packet{}, averr(n)
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

// Close closes the file.
func (f *File) Close() {
	if f.m != nil {
		C.gk_demux_close(f.m)
		f.m = nil
	}
}
