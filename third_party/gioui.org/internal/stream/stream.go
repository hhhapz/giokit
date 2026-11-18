// SPDX-License-Identifier: Unlicense OR MIT

// Package stream is the state shared by paint.Stream (producer and layout)
// and the GPU renderer (consumer). giokit patch.
package stream

import (
	"image"
	"sync"
	"sync/atomic"
)

// DMABuf describes a single-plane image in GPU memory (Linux DMA-BUF).
type DMABuf struct {
	// ID identifies the buffer across frames: a producer cycling through a
	// fixed set of buffers reuses IDs, and the renderer reuses its import.
	// It must change whenever the buffer behind it does.
	ID            uint64
	FD            int
	Offset, Pitch uint32
	Fourcc        uint32
	Modifier      uint64
	Width, Height int
}

// Frame is one image submitted to a stream: CPU pixels or a DMA-BUF.
type Frame struct {
	Img     *image.RGBA
	DMA     *DMABuf
	Release func() // called once the stream no longer needs the frame; may be nil
}

func (f Frame) size() image.Point {
	if f.DMA != nil {
		return image.Pt(f.DMA.Width, f.DMA.Height)
	}
	if f.Img != nil {
		return f.Img.Rect.Size()
	}
	return image.Point{}
}

// Import states, reported by the renderer.
const (
	ImportUnknown     = iota // no DMA-BUF frame drawn yet
	ImportSupported          // DMA-BUF frames are drawn without a copy
	ImportUnsupported        // this renderer cannot import: send CPU frames
)

// Stream holds the newest frame. The renderer uploads (or imports) it into
// a texture it keeps per stream, and only when Seq has moved on.
//
// Imported frames are sampled for as long as they are on screen, so a
// renderer holds the frame it has bound: such a frame is released when the
// last renderer holding it lets go, not when a newer frame replaces it.
// Several renderers (one per window) may draw the same stream; holds are
// counted, and each Hold must be balanced by one Unhold of the same
// sequence number.
//
// A frame's Release function runs exactly once, without the stream's lock
// held, on the goroutine that ends the stream's need for the frame: the
// producer's in Submit or Close, or a renderer's in Use or Unhold. It must
// therefore be safe to call from any goroutine and should not block.
//
// The zero Stream is empty and ready to use.
type Stream struct {
	mu     sync.Mutex
	cur    Frame
	seq    uint64
	size   image.Point
	held   map[uint64]*heldFrame // frames bound by renderers, by sequence number
	closed bool

	importState atomic.Int32
}

// heldFrame is a frame bound by one or more renderers.
type heldFrame struct {
	frame   Frame
	holders int
}

// Submit makes f the current frame and releases the previous one unless a
// renderer holds it. It waits for an upload in progress, so a released
// frame is never mid-copy. Submitting an empty Frame releases the current
// frame and leaves the stream drawing nothing.
//
// After Close, Submit releases f immediately and reports false; otherwise
// it reports true.
func (s *Stream) Submit(f Frame) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		release(f)
		return false
	}
	old := s.replace(f)
	s.mu.Unlock()
	release(old)
	return true
}

// Close empties the stream for good: the current frame is released (once
// no renderer holds it) and later frames are refused. Frames still held are
// released when their renderers Unhold them, which happens at the latest
// when a renderer drops the stream's texture after a frame that does not
// draw it. Close may be called more than once and from any goroutine.
func (s *Stream) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	old := s.replace(Frame{})
	s.mu.Unlock()
	release(old)
}

// replace makes f the current frame and returns the previous one if it
// must be released now, or the empty Frame if a renderer holds it. s.mu
// must be held.
func (s *Stream) replace(f Frame) Frame {
	old, oldSeq := s.cur, s.seq
	s.cur = f
	s.seq++
	s.size = f.size()
	if _, held := s.held[oldSeq]; held {
		return Frame{}
	}
	return old
}

func release(f Frame) {
	if f.Release != nil {
		f.Release()
	}
}

// Size is the current frame's size.
func (s *Stream) Size() image.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size
}

// ImportState reports whether the renderer imports DMA-BUF frames.
func (s *Stream) ImportState() int { return int(s.importState.Load()) }

// SetImportState records the renderer's capability.
func (s *Stream) SetImportState(v int) { s.importState.Store(int32(v)) }

// Use calls fn with the current frame and its sequence number, holding the
// stream's lock so the frame cannot be released during fn. Inside fn, the
// renderer may Hold the current frame and Unhold one it held before. Frames
// whose last hold ends in fn are released after the lock is dropped.
func (s *Stream) Use(fn func(u *Use)) {
	s.mu.Lock()
	u := Use{s: s, Frame: s.cur, Seq: s.seq}
	fn(&u)
	s.mu.Unlock()
	for _, f := range u.release {
		release(f)
	}
}

// Use is the renderer's view of the stream during Stream.Use. It is valid
// only inside the function passed to Use.
type Use struct {
	Frame   Frame
	Seq     uint64
	s       *Stream
	release []Frame
}

// Hold keeps the current frame from being released until a matching
// Unhold. Each renderer holds a given frame at most once.
func (u *Use) Hold() {
	if u.s.held == nil {
		u.s.held = make(map[uint64]*heldFrame)
	}
	h := u.s.held[u.Seq]
	if h == nil {
		h = &heldFrame{frame: u.Frame}
		u.s.held[u.Seq] = h
	}
	h.holders++
}

// Unhold ends one hold of the frame with sequence number seq. When its last
// hold ends, the frame is released (after Use returns) unless it is still
// the current frame. Unholding a frame that is not held does nothing.
func (u *Use) Unhold(seq uint64) {
	h, ok := u.s.held[seq]
	if !ok {
		return
	}
	h.holders--
	if h.holders > 0 {
		return
	}
	delete(u.s.held, seq)
	if seq != u.s.seq {
		u.release = append(u.release, h.frame)
	}
}

// Unhold is Use.Unhold outside a Use: for a renderer dropping its texture.
func (s *Stream) Unhold(seq uint64) {
	s.Use(func(u *Use) { u.Unhold(seq) })
}
