// SPDX-License-Identifier: Unlicense OR MIT

package paint

import (
	"image"

	"gioui.org/internal/ops"
	"gioui.org/internal/stream"
	"gioui.org/op"
)

// Stream is an image whose contents change over time: video, a camera, a
// remote screen. A producer on any goroutine submits frames; the renderer
// keeps one texture per stream and uploads only frames it has not seen.
// Unlike creating an ImageOp per frame, nothing is reallocated and no
// mipmaps are built. giokit patch.
//
// Frames are latest-wins: a frame submitted before the previous one was
// drawn replaces it, so a slow UI drops frames instead of adding latency.
// Any number of windows may draw the same stream.
//
// A producer that is done calls Close, which releases the current frame
// and refuses later ones. Submitting an empty StreamFrame also releases the
// current frame but leaves the stream open.
type Stream struct {
	// Filter is the scaling filter, as for ImageOp.
	Filter ImageFilter
	// OnFrame, if set, is called on the submitting goroutine after each
	// Submit. Use it to invalidate the window. Set it before the first Submit.
	OnFrame func()

	s     stream.Stream
	shape *image.RGBA // size-only placeholder carried by the op
}

// StreamFrame is one frame: RGBA pixels (alpha is respected) or a DMA-BUF,
// and a release function called once the stream has finished with it, for
// pooling. The stream owns the frame's pixels or buffer from Submit until
// Release; the producer must not modify or reuse them before then.
//
// Release runs exactly once per submitted frame. An Image frame is released
// when a newer frame replaces it or the stream is closed; the renderer
// copies it under the stream's lock, so replacement waits for the copy. A
// DMA-BUF frame is sampled in place for as long as it is on screen, so it
// is released only when every window drawing it has moved to a newer frame
// or stopped drawing the stream. Release may therefore run on the
// goroutine calling Submit or Close, or on a window's rendering goroutine.
// It must be safe for concurrent use with the producer and should not
// block; it is never called with the stream's lock held.
type StreamFrame struct {
	Image   *image.RGBA
	DMABuf  *DMABuf
	Release func()
}

// DMABuf describes a single-plane RGBA image in GPU memory (Linux), such as
// a hardware video decoder's output. Renderers that can import it draw it
// with no copy; check Stream.ImportState and send Image frames instead when
// it reports ImportUnsupported. giokit patch.
type DMABuf struct {
	// ID names the buffer across frames. Reuse it when a producer cycles
	// through a fixed set of buffers (the renderer keeps its import), and
	// never reuse it for a different buffer.
	ID            uint64
	FD            int
	Offset, Pitch uint32
	Fourcc        uint32 // DRM fourcc, e.g. ABGR8888 for R,G,B,A bytes
	Modifier      uint64 // DRM format modifier; 0x00ffffffffffffff for none
	Width, Height int
}

// ImportState says whether the renderer draws DMA-BUF frames.
type ImportState int

const (
	ImportUnknown     ImportState = iota // no DMA-BUF frame drawn yet
	ImportSupported                      // drawn in place, no copy
	ImportUnsupported                    // send Image frames instead
)

// NewStream returns an empty stream. It draws nothing until the first Submit.
func NewStream() *Stream { return new(Stream) }

// Submit replaces the current frame. It is safe to call from any goroutine.
// After Close, Submit releases f at once and does not call OnFrame.
func (s *Stream) Submit(f StreamFrame) {
	var d *stream.DMABuf
	if b := f.DMABuf; b != nil {
		d = &stream.DMABuf{ID: b.ID, FD: b.FD, Offset: b.Offset, Pitch: b.Pitch, Fourcc: b.Fourcc,
			Modifier: b.Modifier, Width: b.Width, Height: b.Height}
	}
	if !s.s.Submit(stream.Frame{Img: f.Image, DMA: d, Release: f.Release}) {
		return
	}
	if s.OnFrame != nil {
		s.OnFrame()
	}
}

// Close ends the stream: the current frame is released, once no window
// holds it, and later frames are released as soon as they are submitted.
// The stream draws nothing afterwards. A window still holding a DMA-BUF
// frame releases it when it next renders without drawing the stream. Close
// is safe to call from any goroutine and more than once; it does not call
// OnFrame, so invalidate the window if the stream is on screen.
func (s *Stream) Close() { s.s.Close() }

// ImportState reports whether the renderer drawing this stream imports
// DMA-BUF frames. It stays ImportUnknown until a DMA-BUF frame is drawn.
func (s *Stream) ImportState() ImportState { return ImportState(s.s.ImportState()) }

// Size is the current frame's size, zero before the first frame.
func (s *Stream) Size() image.Point { return s.s.Size() }

// Add sets the brush to the stream's current frame, like ImageOp.Add. Follow
// it with a PaintOp. The image covers (0,0)-Size().
func (s *Stream) Add(o *op.Ops) {
	sz := s.s.Size()
	if sz.X <= 0 || sz.Y <= 0 {
		return
	}
	if s.shape == nil || s.shape.Rect.Size() != sz {
		s.shape = &image.RGBA{Rect: image.Rectangle{Max: sz}}
	}
	data := ops.Write2(&o.Internal, ops.TypeImageLen, s.shape, &s.s)
	data[0] = byte(ops.TypeImage)
	data[1] = byte(s.Filter)
}
