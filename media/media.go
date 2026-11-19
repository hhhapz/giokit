// Package media decodes compressed video (H.264, H.265, AV1, VP8, VP9) into
// frames for a paint.Stream, using FFmpeg's libavcodec through cgo.
//
// It is a separate module so that giokit does not depend on FFmpeg: only
// apps that show video link it. (This is about FFmpeg, not cgo in general:
// Gio's desktop backends may use cgo in any giokit app.)
//
// Decoding can run on the GPU through VA-API (Linux), D3D11VA (Windows) or
// VideoToolbox (macOS), or in software. A hardware frame can leave the
// decoder two ways: copied back to RGBA in system memory (works with every
// API), or converted to RGBA on the GPU and exported as a DMA-BUF (VA-API on
// Linux), which a renderer that can import it draws with no copy. Frame.Path
// says which of the three paths produced a frame.
//
//	d, err := media.NewDecoder(media.H264, media.Options{})
//	if err != nil {
//		return err
//	}
//	defer d.Close()
//	err = d.Decode(pkt, func(f media.Frame) {
//		stream.Submit(f.StreamFrame()) // the stream releases f
//	})
//
// # Ownership
//
// A Decoder or File has one owner: call its methods from one goroutine at a
// time, and do not use it after Close, which reports ErrClosed. Packet data
// is borrowed: Decode copies what it keeps, so the caller may reuse the
// bytes once it returns.
//
// Every Frame passed to an emit callback holds a decoder buffer until
// released. Call Release exactly when done with it, from any goroutine, or
// hand it to a paint.Stream with StreamFrame, which releases it (on the
// renderer's goroutine) once it has drawn a newer frame. Frames remain
// valid after their decoder is closed.
//
// # GIOKIT_VIDEO
//
// The environment variable GIOKIT_VIDEO, read once per process, steers
// decoders whose Options.Hardware is HardwareAuto (an explicit Hardware in
// code wins), so that users and tests can force a path:
//
//	auto              the default policy (as if unset)
//	off, software     decode in software
//	readback          the platform's GPU API, frames copied back to RGBA
//	                  even when Options.DMABuf asks for DMA-BUFs
//	zerocopy          the platform's GPU API with DMA-BUF output
//	vaapi, d3d11va,   that API (output as Options.DMABuf says)
//	videotoolbox
//
// Forced GPU modes make NewDecoder fail where the API is missing, rather
// than silently decoding in software. Other values are ignored.
package media

/*
#include <stdlib.h>
#include "av.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/hhhapz/giokit/gio/op/paint"
)

// Codec is a video codec.
type Codec int

const (
	H264 Codec = C.GK_H264 // H.264 / AVC
	H265 Codec = C.GK_H265 // H.265 / HEVC
	AV1  Codec = C.GK_AV1  // AV1
	VP8  Codec = C.GK_VP8  // VP8
	VP9  Codec = C.GK_VP9  // VP9
)

// String returns the codec's lower-case short name ("h264", "av1"), or
// "codec(n)" for an unknown value.
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
	// (Options.DMABuf on a build with zero-copy export: VA-API on Linux),
	// else in software: copying a decoded frame back from video memory
	// costs more than decoding it on the CPU (2960×1848 H.264 on an RX 6700
	// XT: 17.8 ms/frame through VA-API and a copy, 5.7 ms in software,
	// 3.0 ms through VA-API and a DMA-BUF). If the GPU decoder cannot open,
	// it decodes in software. GIOKIT_VIDEO overrides it (see the package
	// doc).
	HardwareAuto Hardware = C.GK_HW_AUTO
	HardwareOff  Hardware = C.GK_HW_OFF // always software
	// HardwareGPU decodes through the platform's API (VA-API on Linux,
	// D3D11VA on Windows, VideoToolbox on macOS) or fails, whatever the
	// output. With Options.DMABuf off, frames take the readback path.
	HardwareGPU          Hardware = C.GK_HW_GPU
	HardwareVAAPI        Hardware = C.GK_HW_VAAPI        // VA-API or fail, whatever the output
	HardwareD3D11VA      Hardware = C.GK_HW_D3D11VA      // Direct3D 11 video or fail; readback only
	HardwareVideoToolbox Hardware = C.GK_HW_VIDEOTOOLBOX // VideoToolbox or fail; readback only
)

// String returns the mode's lower-case name ("auto", "vaapi"), the same
// names GIOKIT_VIDEO takes, or "hardware(n)" for an unknown value.
func (h Hardware) String() string {
	switch h {
	case HardwareAuto:
		return "auto"
	case HardwareOff:
		return "off"
	case HardwareGPU:
		return "gpu"
	case HardwareVAAPI:
		return "vaapi"
	case HardwareD3D11VA:
		return "d3d11va"
	case HardwareVideoToolbox:
		return "videotoolbox"
	}
	return fmt.Sprintf("hardware(%d)", int(h))
}

// PlatformHardware is the API HardwareAuto and HardwareGPU use on this
// build: HardwareD3D11VA on Windows, HardwareVideoToolbox on macOS,
// HardwareVAAPI elsewhere.
func PlatformHardware() Hardware { return Hardware(C.gk_hw_platform()) }

// Path is the way a frame left the decoder.
type Path int

const (
	PathSoftware Path = iota // decoded on the CPU, RGBA in memory
	PathReadback             // decoded on the GPU, copied back to RGBA in memory
	PathZeroCopy             // decoded on the GPU, exported as a DMA-BUF
)

// String returns "software", "hw-readback" or "zero-copy".
func (p Path) String() string {
	switch p {
	case PathSoftware:
		return "software"
	case PathReadback:
		return "hw-readback"
	case PathZeroCopy:
		return "zero-copy"
	}
	return fmt.Sprintf("path(%d)", int(p))
}

// Options configure a Decoder. The zero value is right for live streams.
type Options struct {
	Hardware Hardware // where to decode; HardwareAuto is the zero value
	// Device picks the GPU: the VA-API render node (/dev/dri/renderD129)
	// or the D3D11 adapter index ("1"); VideoToolbox ignores it. ""
	// picks the default.
	Device string
	// Buffered allows frame threading and reordering: more throughput, more
	// latency. Leave it off for live video.
	Buffered bool
	// DMABuf asks for hardware frames as DMA-BUFs (GPU conversion, no copy)
	// instead of RGBA in memory. Frames fall back to RGBA whenever export is
	// not possible (always, outside VA-API); check Frame.DMABuf. Leave it
	// off to force the readback path.
	DMABuf bool
}

// Packet is one compressed frame, or codec configuration.
type Packet struct {
	Data []byte // borrowed for the duration of Decode
	PTS  int64  // microseconds
	Key  bool   // a key frame
	// Config marks codec configuration (H.264/H.265 parameter sets, the AV1
	// sequence header) sent ahead of the frames, as Android encoders and
	// scrcpy do. The decoder prepends it to the next packet.
	Config bool
}

// Frame is one decoded picture. Exactly one of Image and DMABuf is set.
// Its buffer belongs to the decoder's pool until Release: call Release
// when done with it, or pass StreamFrame to a paint.Stream, which does.
type Frame struct {
	Width, Height int         // picture size in pixels
	PTS           int64       // microseconds
	Hardware      bool        // decoded on the GPU
	Image         *image.RGBA // the picture in memory, or nil
	DMABuf        *DMABuf     // the picture in GPU memory, or nil
	release       func()
}

// Path reports how the frame was produced: PathZeroCopy when DMABuf is set,
// else PathReadback for a GPU-decoded frame, else PathSoftware.
func (f Frame) Path() Path {
	switch {
	case f.DMABuf != nil:
		return PathZeroCopy
	case f.Hardware:
		return PathReadback
	}
	return PathSoftware
}

// Release returns the frame's buffer for reuse; stop using Image or DMABuf
// after it. It may run on any goroutine, also after the decoder is closed.
// Calls after the first do nothing.
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
	FD            int    // the dma-buf file descriptor, owned by the decoder
	Offset, Pitch uint32 // the plane's start and row stride, in bytes
	Fourcc        uint32 // DRM format, e.g. 'AB24' (ABGR8888: bytes R,G,B,A)
	Modifier      uint64 // DRM format modifier (tiling)
	Width, Height int    // image size in pixels
}

// zeroCopyBuild reports whether this build can export DMA-BUFs (for tests,
// which cannot use cgo).
func zeroCopyBuild() bool { return C.gk_zero_copy() != 0 }

// failHardwareDevices makes every hardware device creation fail while
// fail is set, for tests of the software fallback. Not for concurrent use
// with decoders being opened.
func failHardwareDevices(fail bool) {
	f := C.int(0)
	if fail {
		f = 1
	}
	C.gk_set_fail_hwdevice(f)
}

// videoEnv is GIOKIT_VIDEO, read once.
var videoEnv = sync.OnceValue(func() string { return os.Getenv("GIOKIT_VIDEO") })

// applyEnv returns opt as the GIOKIT_VIDEO value env changes it. Only
// HardwareAuto is overridden; unknown values change nothing.
func applyEnv(opt Options, env string) Options {
	if opt.Hardware != HardwareAuto {
		return opt
	}
	switch env {
	case "off", "software":
		opt.Hardware = HardwareOff
	case "readback":
		opt.Hardware, opt.DMABuf = HardwareGPU, false
	case "zerocopy":
		opt.Hardware, opt.DMABuf = HardwareGPU, true
	case "vaapi":
		opt.Hardware = HardwareVAAPI
	case "d3d11va":
		opt.Hardware = HardwareD3D11VA
	case "videotoolbox":
		opt.Hardware = HardwareVideoToolbox
	}
	return opt
}

// ErrClosed is returned by a Decoder or File used after Close.
var ErrClosed = errors.New("media: use after Close")

// rgbaPoolCap is how many released RGBA buffers a decoder keeps for reuse:
// enough for the frames a renderer typically holds (on screen, queued and
// in flight) without retaining much memory for a stalled consumer.
const rgbaPoolCap = 4

// Error is an FFmpeg error. Errors that callers branch on are separate:
// io.EOF for the end of input and ErrClosed for use after Close.
type Error struct {
	Op   string // what failed, such as "open clip.mp4" or "decode"
	Code int    // FFmpeg's negative AVERROR code
	Msg  string // FFmpeg's description of Code
}

// Error returns "media: Op: Msg".
func (e *Error) Error() string { return "media: " + e.Op + ": " + e.Msg }

// averr converts an FFmpeg return code from op to an error: nil for
// success, io.EOF at the end of input, and an *Error otherwise.
func averr(op string, code C.int) error {
	if code >= 0 {
		return nil
	}
	if code == C.gk_eof() {
		return io.EOF
	}
	var buf [128]C.char
	C.gk_err(code, &buf[0], C.int(len(buf)))
	return &Error{Op: op, Code: int(code), Msg: C.GoString(&buf[0])}
}

// Decoder decodes one stream. Its methods must be called from one goroutine
// at a time; frames may be released from any goroutine. Create it with
// NewDecoder and free it with Close.
type Decoder struct {
	codec Codec
	opt   Options

	c *C.gk_dec // nil after Close

	// config is configuration waiting to be prepended to the next packet;
	// last is the most recent configuration, kept to prime a reopened
	// decoder after config has been consumed.
	config []byte
	last   []byte

	softwareNext bool  // reopen in software at the next key frame (Auto, export failed)
	dmaFailed    error // why DMA-BUF export stopped, if it did

	// The RGBA pool is shared with Frame.Release, which may run on any
	// goroutine.
	mu   sync.Mutex
	pool []*image.RGBA // released RGBA buffers of size
	size image.Point
}

// NewDecoder opens a decoder for codec. Under HardwareAuto, GIOKIT_VIDEO
// may change opt (see the package doc).
func NewDecoder(codec Codec, opt Options) (*Decoder, error) {
	opt = applyEnv(opt, videoEnv())
	d := &Decoder{codec: codec, opt: opt}
	hw := opt.Hardware
	if hw == HardwareAuto && !(opt.DMABuf && zeroCopyBuild()) {
		hw = HardwareOff
	}
	if err := d.open(hw); err != nil {
		return nil, err
	}
	return d, nil
}

// open opens the codec context for hw. It sets d.c only on success.
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
		return averr(fmt.Sprintf("open %v decoder", d.codec), e)
	}
	d.c = c
	return nil
}

// Codec is the decoder's codec.
func (d *Decoder) Codec() Codec { return d.codec }

// Hardware reports whether the decoder runs on the GPU. A hardware decoder
// may still hand out software frames for a stream the GPU cannot decode.
// It is false after Close.
func (d *Decoder) Hardware() bool { return d.HardwareAPI() != HardwareOff }

// HardwareAPI is the GPU API the decoder opened (HardwareVAAPI,
// HardwareD3D11VA or HardwareVideoToolbox), or HardwareOff for a software
// decoder and after Close.
func (d *Decoder) HardwareAPI() Hardware {
	if d.c == nil {
		return HardwareOff
	}
	return Hardware(C.gk_dec_hw(d.c))
}

// DisableDMABuf stops DMA-BUF output, for a renderer that reported it
// cannot import (paint.ImportUnsupported). Under HardwareAuto the decoder
// switches to software at the next key frame. Calls after the first do
// nothing.
func (d *Decoder) DisableDMABuf() {
	if d.dmaFailed == nil {
		d.dmaFailed = errors.New("media: DMA-BUF output disabled: the renderer cannot import")
		d.softwareNext = d.opt.Hardware == HardwareAuto && d.Hardware()
	}
}

// DMABufError reports why DMA-BUF export was turned off, or nil while it
// is on (or was never requested). The reason is the first one seen: the
// FFmpeg or VA-API error from the export (an *Error), or DisableDMABuf's
// renderer report. Frames are RGBA from then on.
func (d *Decoder) DMABufError() error { return d.dmaFailed }

// Decode sends one packet and calls emit for every frame it completes (in
// low-delay mode, normally one). emit runs on the calling goroutine; each
// frame it receives must be released (see Frame). A Config packet is
// stored and sent ahead of the next frame packet.
func (d *Decoder) Decode(p Packet, emit func(Frame)) error {
	if d.c == nil {
		return ErrClosed
	}
	if p.Config {
		d.config = append(d.config[:0], p.Data...)
		d.last = append(d.last[:0], p.Data...)
		return nil
	}
	if d.softwareNext && p.Key {
		// Export failed under HardwareAuto: a key frame is a clean point to
		// switch to the faster software path. If the software decoder does
		// not open, open leaves d.c as it was and decoding carries on.
		d.softwareNext = false
		old := d.c
		if err := d.open(HardwareOff); err == nil {
			C.gk_dec_close(old)
			if len(d.config) == 0 {
				d.config = append(d.config, d.last...)
			}
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
		return averr("decode", r)
	}
	return d.drain(emit)
}

// Flush drains the frames still inside the decoder at the end of a stream,
// calling emit as Decode does.
func (d *Decoder) Flush(emit func(Frame)) error {
	if d.c == nil {
		return ErrClosed
	}
	if r := C.gk_dec_flush(d.c); r < 0 && r != C.gk_eof() {
		return averr("flush", r)
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
			return averr("decode", r)
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
			d.dmaFailed = averr("export DMA-BUF", e)
			d.softwareNext = d.opt.Hardware == HardwareAuto
		}
	}
	if f.Width <= 0 || f.Height <= 0 {
		// The conversion writes through &img.Pix[0]; an empty image has none.
		return Frame{}, fmt.Errorf("media: %v decoder produced a %dx%d frame", d.codec, f.Width, f.Height)
	}
	img := d.buffer(image.Pt(f.Width, f.Height))
	if r := C.gk_dec_frame_rgba(d.c, (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])), C.int(img.Stride)); r < 0 {
		d.put(img)
		return Frame{}, averr("convert to RGBA", r)
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
	if img.Rect.Size() == d.size && len(d.pool) < rgbaPoolCap {
		d.pool = append(d.pool, img)
	}
}

// Close frees the decoder and its buffer pool. Frames already handed out
// stay valid until released. Later calls to Decode and Flush return
// ErrClosed; calling Close again does nothing.
func (d *Decoder) Close() {
	if d.c == nil {
		return
	}
	C.gk_dec_close(d.c)
	d.c = nil
	d.mu.Lock()
	defer d.mu.Unlock()
	// A zero size matches no frame, so later releases are not pooled.
	d.pool, d.size = nil, image.Point{}
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
