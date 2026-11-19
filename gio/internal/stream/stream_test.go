// SPDX-License-Identifier: Unlicense OR MIT

package stream

import (
	"image"
	"testing"
)

// counted returns a frame whose Release increments *n.
func counted(n *int) Frame {
	return Frame{Img: image.NewRGBA(image.Rect(0, 0, 2, 1)), Release: func() { *n++ }}
}

// hold holds the current frame of s and returns its sequence number.
func hold(s *Stream) uint64 {
	var seq uint64
	s.Use(func(u *Use) {
		seq = u.Seq
		u.Hold()
	})
	return seq
}

func TestReplaceReleases(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	if released != 0 {
		t.Fatalf("current frame released: %d", released)
	}
	s.Submit(Frame{})
	if released != 1 {
		t.Fatalf("replaced frame released %d times, want 1", released)
	}
	if got := s.Size(); got != (image.Point{}) {
		t.Errorf("empty frame size: %v", got)
	}
}

func TestHoldDefersRelease(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	seq := hold(&s)
	s.Submit(Frame{})
	if released != 0 {
		t.Fatalf("held frame released on replace: %d", released)
	}
	s.Unhold(seq)
	if released != 1 {
		t.Fatalf("held frame released %d times after Unhold, want 1", released)
	}
	s.Unhold(seq)
	if released != 1 {
		t.Fatalf("second Unhold released again: %d", released)
	}
}

func TestUnholdCurrentKeepsFrame(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	seq := hold(&s)
	s.Unhold(seq)
	if released != 0 {
		t.Fatalf("current frame released on Unhold: %d", released)
	}
	s.Submit(Frame{})
	if released != 1 {
		t.Fatalf("frame released %d times after replace, want 1", released)
	}
}

// Two renderers (windows) bind the same frame: it stays alive until both
// let go.
func TestTwoHolders(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	first := hold(&s)
	second := hold(&s)
	s.Submit(Frame{})
	s.Unhold(first)
	if released != 0 {
		t.Fatalf("frame released with second holder still active: released=%d", released)
	}
	s.Unhold(second)
	if released != 1 {
		t.Fatalf("frame released %d times after both Unholds, want 1", released)
	}
}

func TestReleaseOutsideLock(t *testing.T) {
	var s Stream
	// A Release that re-enters the stream deadlocks if it runs under the lock.
	s.Submit(Frame{Release: func() { s.Size() }})
	seq := hold(&s)
	s.Submit(Frame{Release: func() { s.Size() }})
	s.Unhold(seq)
	s.Close()
}

func TestClose(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	s.Close()
	if released != 1 {
		t.Fatalf("Close released current frame %d times, want 1", released)
	}
	if got := s.Size(); got != (image.Point{}) {
		t.Errorf("size after Close: %v", got)
	}
	late := 0
	if s.Submit(counted(&late)) {
		t.Error("Submit after Close accepted the frame")
	}
	if late != 1 {
		t.Errorf("frame submitted after Close released %d times, want 1", late)
	}
	s.Use(func(u *Use) {
		if u.Frame.Img != nil || u.Frame.DMA != nil {
			t.Error("closed stream has a current frame")
		}
	})
	s.Close()
	if released != 1 || late != 1 {
		t.Errorf("second Close released again: %d, %d", released, late)
	}
}

func TestCloseWhileHeld(t *testing.T) {
	var s Stream
	released := 0
	s.Submit(counted(&released))
	seq := hold(&s)
	s.Close()
	if released != 0 {
		t.Fatalf("held frame released on Close: %d", released)
	}
	s.Unhold(seq)
	if released != 1 {
		t.Fatalf("held frame released %d times after Unhold, want 1", released)
	}
}
