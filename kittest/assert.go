package kittest

import (
	"fmt"
	"image"
	"image/color"
	"slices"

	"gioui.org/io/key"
)

// bgTolerance is how far, per channel, ExpectBg lets a sampled colour stray
// for GPU rounding.
const bgTolerance = 2

// ExpectClean fails the test for every Audit finding.
func (h *Harness) ExpectClean() {
	h.tb.Helper()
	h.requireTest("ExpectClean")
	for _, p := range h.Audit() {
		h.tb.Error(p.String())
	}
}

// ExpectID returns the bounds of the widget tagged id, stopping the test if
// it is not on screen.
func (h *Harness) ExpectID(id string) image.Rectangle {
	h.tb.Helper()
	h.requireTest("ExpectID")
	return h.must(id)
}

// ExpectNoID fails the test if a widget tagged id is on screen.
func (h *Harness) ExpectNoID(id string) {
	h.tb.Helper()
	h.requireTest("ExpectNoID")
	if _, ok := h.Find(id); ok {
		h.tb.Errorf("widget %q should not be on screen", id)
	}
}

// ExpectText fails the test unless some label on screen is exactly s.
func (h *Harness) ExpectText(s string) {
	h.tb.Helper()
	h.requireTest("ExpectText")
	if _, ok := h.FindText(s); !ok {
		h.tb.Errorf("text %q not on screen; texts: %q", s, h.Texts())
	}
}

// ExpectTextIn fails the test unless the widget tagged id shows the label s
// (see TextsIn).
func (h *Harness) ExpectTextIn(id, s string) {
	h.tb.Helper()
	h.requireTest("ExpectTextIn")
	if texts := h.TextsIn(id); !slices.Contains(texts, s) {
		h.tb.Errorf("%q shows %q, want %q", id, texts, s)
	}
}

// ExpectBg checks BgColor(id) within ±2 per channel.
func (h *Harness) ExpectBg(id string, want color.NRGBA) {
	h.tb.Helper()
	h.requireTest("ExpectBg")
	if got := h.BgColor(id); !Near(got, want, bgTolerance) {
		h.tb.Errorf("%q background = %v, want %v", id, got, want)
	}
}

// FocusLeak says how focus left a focus trap. Exactly one field is set.
type FocusLeak struct {
	To     string // the focused id outside the trap
	Lost   bool   // nothing with an id holds focus
	Closed bool   // the trap itself is no longer on screen
}

// String describes the leak for a test failure.
func (l FocusLeak) String() string {
	switch {
	case l.Closed:
		return "the trap closed"
	case l.Lost:
		return "focus was lost"
	}
	return fmt.Sprintf("focus reached %q", l.To)
}

// FocusEscape presses Tab n times, then Shift+Tab n times, and reports how
// focus first left the widget tagged id, or ok false if it stayed inside:
// the focus trap a modal layer must have.
func (h *Harness) FocusEscape(id string, n int) (leak FocusLeak, ok bool) {
	h.tb.Helper()
	for i := range 2 * n {
		h.Tab(i >= n)
		r, found := h.Find(id)
		if !found {
			return FocusLeak{Closed: true}, true
		}
		f := h.FocusedID()
		if f == "" {
			return FocusLeak{Lost: true}, true
		}
		if f == id {
			continue
		}
		if fr, found := h.Find(f); !found || !fr.In(r) {
			return FocusLeak{To: f}, true
		}
	}
	return FocusLeak{}, false
}

// ExpectFocusTrapped fails if Tab or Shift+Tab (n presses each) moves focus
// out of the widget tagged id.
func (h *Harness) ExpectFocusTrapped(id string, n int) {
	h.tb.Helper()
	h.requireTest("ExpectFocusTrapped")
	if leak, ok := h.FocusEscape(id, n); ok {
		h.tb.Errorf("focus left %q: %v", id, leak)
	}
}

// ExpectEscapeCloses presses Escape and fails unless the widget tagged id
// (a popup, menu or dialog) is gone.
func (h *Harness) ExpectEscapeCloses(id string) {
	h.tb.Helper()
	h.requireTest("ExpectEscapeCloses")
	h.must(id)
	h.Key(key.NameEscape, 0)
	if _, ok := h.Find(id); ok {
		h.tb.Errorf("Escape did not close %q", id)
	}
}
