// Package media decodes compressed video (H.264, H.265, AV1, VP8, VP9) into
// frames for a paint.Stream, using FFmpeg's libavcodec through cgo.
//
// It is a separate module so that giokit itself stays free of cgo: only
// apps that show video pay for FFmpeg.
//
// Decoding prefers the GPU (VA-API on Linux) and falls back to software.
// A hardware frame can leave the decoder two ways: copied to RGBA in system
// memory (works everywhere), or converted to RGBA on the GPU and exported as
// a DMA-BUF (Linux), which a renderer that can import it draws with no copy.
//
//	d, _ := media.NewDecoder(media.H264, media.Options{})
//	d.Decode(pkt, func(f media.Frame) { stream.Submit(f.StreamFrame()) })
package media

/*
#cgo pkg-config: libavcodec libavformat libavutil libswscale
#cgo linux pkg-config: libva
#include <stdlib.h>
#include "av.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"io"
	"sync"
	"unsafe"

	"gioui.org/op/paint"
)

// Codec is a video codec.
type Codec int

const (
	H264 Codec = C.GK_H264
	H265 Codec = C.GK_H265
	AV1  Codec = C.GK_AV1
	VP8  Codec = C.GK_VP8
	VP9  Codec = C.GK_VP9
)

func (c Codec) String() string {
	switch c {
	case H264:
		return "h264"
	case H265:
		return "h265"
	case AV1:
		return "av1"
	case VP8:
		return "vp8"
	case VP9:
		return "vp9"
	}
	return fmt.Sprintf("codec(%d)", int(c))
}

// Hardware selects the decoder.
type Hardware int

const (
	// HardwareAuto decodes on the GPU when frames can stay there
	// (Options.DMABuf), else in software: copying a decoded frame back from
	// video memory costs more than decoding it on the CPU (2960×1848 H.264
	// on an RX 6700 XT: 17.8 ms/frame through VA-API and a copy, 5.7 ms in
	// software, 3.0 ms through VA-API and a DMA-BUF).
	HardwareAuto  Hardware = C.GK_HW_AUTO
	HardwareOff   Hardware = C.GK_HW_OFF   // always software
	HardwareVAAPI Hardware = C.GK_HW_VAAPI // VA-API or fail, whatever the output
)

// Options configure a Decoder. The zero value is right for live streams.
type Options struct {
	Hardware Hardware
	// Device is the VA-API render node; "" picks the default.
	Device string
	// Buffered allows frame threading and reordering: more throughput, more
	// latency. Leave it off for live video.
	Buffered bool
	// DMABuf asks for hardware frames as DMA-BUFs (GPU conversion, no copy)
	// instead of RGBA in memory. Frames fall back to RGBA whenever export is
	// not possible; check Frame.DMABuf.
	DMABuf bool
}

// Packet is one compressed frame, or codec configuration.
type Packet struct {
	Data []byte
	PTS  int64 // microseconds
	Key  bool  // a key frame
	// Config marks codec configuration (H.264/H.265 parameter sets, the AV1
	// sequence header) sent ahead of the frames, as Android encoders and
	// scrcpy do. The decoder prepends it to the next packet.
	Config bool
}

// Frame is one decoded picture. Exactly one of Image and DMABuf is set.
// Call Release when done with it (a paint.Stream does, via StreamFrame).
type Frame struct {
	Width, Height int
	PTS           int64 // microseconds
	Hardware      bool  // decoded on the GPU
	Image         *image.RGBA
	DMABuf        *DMABuf
	release       func()
}

// Release returns the frame's buffer. Safe to call from any goroutine, once.
func (f Frame) Release() {
	if f.release != nil {
		f.release()
	}
}

// DMABuf describes a single-plane RGBA image in GPU memory (Linux). The
// file descriptor belongs to the decoder: do not close it, and stop using it
// once the frame is released.
type DMABuf struct {
	ID            uint64 // the buffer's identity: the decoder cycles a few
	FD            int
	Offset, Pitch uint32
	Fourcc        uint32 // DRM format, e.g. 'AB24' (ABGR8888: bytes R,G,B,A)
	Modifier      uint64 // DRM format modifier (tiling)
	Width, Height int
}

// Error is an FFmpeg error.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return "media: " + e.Msg }

func averr(code C.int) error {
	if code >= 0 {
		return nil
	}
	if code == C.gk_eof() {
		return io.EOF
	}
	var buf [128]C.char
	C.gk_err(code, &buf[0], C.int(len(buf)))
	return &Error{Code: int(code), Msg: C.GoString(&buf[0])}
}

// Decoder decodes one stream. Its methods must be called from one goroutine
// at a time; frames may be released from any goroutine.
type Decoder struct {
	c      *C.gk_dec
	codec  Codec
	opt    Options
	config []byte
	last   []byte // the most recent configuration, for reopening

	softwareNext bool // reopen in software at the next key frame (Auto, export failed)

	mu   sync.Mutex
	pool []*image.RGBA // released RGBA buffers of the current size
	size image.Point

	dmaFailed error // why DMA-BUF export stopped, if it did
}

// NewDecoder opens a decoder for codec.
func NewDecoder(codec Codec, opt Options) (*Decoder, error) {
	d := &Decoder{codec: codec, opt: opt}
	hw := opt.Hardware
	if hw == HardwareAuto && !opt.DMABuf {
		hw = HardwareOff
	}
	if err := d.open(hw); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Decoder) open(hw Hardware) error {
	opt := d.opt
	var dev *C.char
	if opt.Device != "" {
		dev = C.CString(opt.Device)
		defer C.free(unsafe.Pointer(dev))
	}
	lowDelay := C.int(1)
	if opt.Buffered {
		lowDelay = 0
	}
	var e C.int
	c := C.gk_dec_open(C.int(d.codec), C.int(hw), dev, lowDelay, &e)
	if c == nil {
		return fmt.Errorf("open %v decoder: %w", d.codec, averr(e))
	}
	d.c = c
	return nil
}

// Codec is the decoder's codec.
func (d *Decoder) Codec() Codec { return d.codec }

// Hardware reports whether the decoder runs on the GPU. A hardware decoder
// may still hand out software frames for a stream the GPU cannot decode.
func (d *Decoder) Hardware() bool { return C.gk_dec_hw(d.c) != 0 }

// DisableDMABuf stops DMA-BUF output, for a renderer that reported it
// cannot import (paint.ImportUnsupported). Under HardwareAuto the decoder
// switches to software at the next key frame.
func (d *Decoder) DisableDMABuf() {
	if d.dmaFailed == nil {
		d.dmaFailed = errors.New("media: DMA-BUF output disabled: the renderer cannot import")
		d.softwareNext = d.opt.Hardware == HardwareAuto && d.Hardware()
	}
}

// DMABufError reports why DMA-BUF export was turned off, or nil.
func (d *Decoder) DMABufError() error { return d.dmaFailed }

// Decode sends one packet and calls emit for every frame it completes (in
// low-delay mode, normally one). emit runs on the calling goroutine.
func (d *Decoder) Decode(p Packet, emit func(Frame)) error {
	if p.Config {
		d.config = append(d.config[:0], p.Data...)
		d.last = append(d.last[:0], p.Data...)
		return nil
	}
	if d.softwareNext && p.Key {
		// Export failed under HardwareAuto: a key frame is a clean point to
		// switch to the faster software path.
		d.softwareNext = false
		old := d.c
		if err := d.open(HardwareOff); err == nil {
			C.gk_dec_close(old)
			if len(d.config) == 0 {
				d.config = append(d.config, d.last...)
			}
		} else {
			d.c = old
		}
	}
	data := p.Data
	if len(d.config) > 0 {
		data = append(append(make([]byte, 0, len(d.config)+len(p.Data)), d.config...), p.Data...)
		d.config = d.config[:0]
	}
	if len(data) == 0 {
		return nil
	}
	key := C.int(0)
	if p.Key {
		key = 1
	}
	r := C.gk_dec_send(d.c, (*C.uint8_t)(unsafe.Pointer(&data[0])), C.int(len(data)), C.int64_t(p.PTS), key)
	if r < 0 && r != C.gk_eagain() {
		return averr(r)
	}
	return d.drain(emit)
}

// Flush drains the frames still inside the decoder at the end of a stream.
func (d *Decoder) Flush(emit func(Frame)) error {
	if r := C.gk_dec_flush(d.c); r < 0 && r != C.gk_eof() {
		return averr(r)
	}
	err := d.drain(emit)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (d *Decoder) drain(emit func(Frame)) error {
	for {
		r := C.gk_dec_receive(d.c)
		if r == C.gk_eagain() {
			return nil
		}
		if r < 0 {
			return averr(r)
		}
		f, err := d.frame()
		if err != nil {
			return err
		}
		emit(f)
	}
}

func (d *Decoder) frame() (Frame, error) {
	var w, h, hw C.int
	var pts C.int64_t
	C.gk_dec_frame_info(d.c, &w, &h, &pts, &hw)
	f := Frame{Width: int(w), Height: int(h), PTS: int64(pts), Hardware: hw != 0}
	if f.Hardware && d.opt.DMABuf && d.dmaFailed == nil {
		var desc C.gk_dmabuf
		var e C.int
		if hnd := C.gk_dec_frame_dmabuf(d.c, &desc, &e); hnd != nil {
			f.DMABuf = &DMABuf{
				ID: uint64(desc.id), FD: int(desc.fd), Offset: uint32(desc.offset), Pitch: uint32(desc.pitch),
				Fourcc: uint32(desc.fourcc), Modifier: uint64(desc.modifier),
				Width: int(desc.width), Height: int(desc.height),
			}
			var once sync.Once
			f.release = func() { once.Do(func() { C.gk_dmabuf_release(hnd) }) }
			return f, nil
		} else if e != C.gk_eagain() {
			// Not available on this GPU or driver: stop trying, use RGBA.
			d.dmaFailed = averr(e)
			d.softwareNext = d.opt.Hardware == HardwareAuto
		}
	}
	img := d.buffer(image.Pt(f.Width, f.Height))
	if r := C.gk_dec_frame_rgba(d.c, (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])), C.int(img.Stride)); r < 0 {
		d.put(img)
		return Frame{}, averr(r)
	}
	f.Image = img
	var once sync.Once
	f.release = func() { once.Do(func() { d.put(img) }) }
	return f, nil
}

// buffer returns a pooled RGBA image of size sz.
func (d *Decoder) buffer(sz image.Point) *image.RGBA {
	d.mu.Lock()
	defer d.mu.Unlock()
	if sz != d.size {
		d.pool, d.size = nil, sz
	}
	if n := len(d.pool); n > 0 {
		img := d.pool[n-1]
		d.pool = d.pool[:n-1]
		return img
	}
	return image.NewRGBA(image.Rectangle{Max: sz})
}

func (d *Decoder) put(img *image.RGBA) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if img.Rect.Size() == d.size && len(d.pool) < 4 {
		d.pool = append(d.pool, img)
	}
}

// Close frees the decoder. Frames already handed out stay valid until
// released.
func (d *Decoder) Close() {
	if d.c != nil {
		C.gk_dec_close(d.c)
		d.c = nil
	}
}

// StreamFrame converts f for a paint.Stream, which releases it when done.
func (f Frame) StreamFrame() paint.StreamFrame {
	sf := paint.StreamFrame{Image: f.Image, Release: f.release}
	if b := f.DMABuf; b != nil {
		sf.DMABuf = &paint.DMABuf{ID: b.ID, FD: b.FD, Offset: b.Offset, Pitch: b.Pitch,
			Fourcc: b.Fourcc, Modifier: b.Modifier, Width: b.Width, Height: b.Height}
	}
	return sf
}
