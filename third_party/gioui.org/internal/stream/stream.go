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
// Imported frames are sampled for as long as they are on screen, so the
// renderer holds the frame it has bound: such a frame is released when the
// renderer lets go of it, not when a newer frame replaces it.
type Stream struct {
	mu   sync.Mutex
	cur  Frame
	seq  uint64
	size image.Point
	held map[uint64]Frame // frames bound by a renderer, by sequence number

	importState atomic.Int32
}

// Submit makes f the current frame and releases the previous one unless a
// renderer holds it. It waits for an upload in progress, so a released
// frame is never mid-copy.
func (s *Stream) Submit(f Frame) {
	s.mu.Lock()
	old, oldSeq := s.cur, s.seq
	s.cur = f
	s.seq++
	s.size = f.size()
	_, held := s.held[oldSeq]
	s.mu.Unlock()
	if !held && old.Release != nil {
		old.Release()
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
// stream so the frame cannot be released during fn. Inside fn, the
// renderer may Hold the current frame and Unhold one it held before.
func (s *Stream) Use(fn func(u *Use)) {
	s.mu.Lock()
	u := Use{s: s, Frame: s.cur, Seq: s.seq}
	fn(&u)
	s.mu.Unlock()
	for _, r := range u.release {
		r()
	}
}

// Use is the renderer's view of the stream during Stream.Use.
type Use struct {
	Frame   Frame
	Seq     uint64
	s       *Stream
	release []func()
}

// Hold keeps the current frame from being released until Unhold.
func (u *Use) Hold() {
	if u.s.held == nil {
		u.s.held = make(map[uint64]Frame)
	}
	u.s.held[u.Seq] = u.Frame
}

// Unhold lets go of a frame held earlier; it is released (after Use returns)
// unless it is still the current frame.
func (u *Use) Unhold(seq uint64) {
	f, ok := u.s.held[seq]
	if !ok {
		return
	}
	delete(u.s.held, seq)
	if seq != u.s.seq && f.Release != nil {
		u.release = append(u.release, f.Release)
	}
}

// Unhold is Use.Unhold outside a Use: for a renderer dropping its texture.
func (s *Stream) Unhold(seq uint64) {
	s.Use(func(u *Use) { u.Unhold(seq) })
}
