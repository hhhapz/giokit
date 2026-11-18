package ui_test

import (
	"math"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Turning Password off shows the text again (the mask used to stay).
func TestTextFieldPasswordOff(t *testing.T) {
	th := kit.Dark()
	var e widget.Editor
	password := true
	h := kittest.NewT(t, 300, 100, func(gtx layout.Context) {
		ui.TextField{ID: "pw", Label: "Password", Password: password}.Layout(gtx, th, &e)
	})
	if e.Mask == 0 {
		t.Fatal("Password field not masked")
	}
	password = false
	h.Settle()
	if e.Mask != 0 {
		t.Fatalf("mask %q stayed after Password turned off", e.Mask)
	}
}

// Fractional steps show the decimal the user expects, not the binary
// floating-point sum (0.1+0.1+0.1 printed 0.30000000000000004).
func TestNumberFieldFractionalSteps(t *testing.T) {
	th := kit.Dark()
	var s ui.NumberFieldState
	var v float64
	h := kittest.NewT(t, 300, 100, func(gtx layout.Context) {
		if nv, ok := s.Update(gtx); ok {
			v = nv
		}
		ui.NumberField{ID: "gain", Placeholder: "Gain", Step: 0.1}.Layout(gtx, th, &s)
	})
	for range 3 {
		h.ClickID("gain/plus")
	}
	if got := s.Editor.Text(); got != "0.3" {
		t.Errorf("text after three steps of 0.1: %q, want 0.3", got)
	}
	if v != 0.3 {
		t.Errorf("value after three steps of 0.1: %v, want 0.3", v)
	}
}

// A one-sided bound clamps on its side only. ("Bounded when Min or Max is
// non-zero" clamped every value to Min when only Min was set.)
func TestNumberFieldOneSidedBound(t *testing.T) {
	th := kit.Dark()
	var s ui.NumberFieldState
	h := kittest.NewT(t, 300, 100, func(gtx layout.Context) {
		s.Update(gtx)
		ui.NumberField{ID: "n", Label: "Count", Min: 1, Max: math.Inf(1)}.Layout(gtx, th, &s)
	})
	s.SetValue(1)
	h.ClickID("n/minus")
	if s.Value() != 1 {
		t.Errorf("below Min: %v", s.Value())
	}
	h.ClickID("n/plus")
	h.ClickID("n/plus")
	if s.Value() != 3 {
		t.Errorf("two steps up from 1: %v", s.Value())
	}
	if got := label(h, "n/plus"); got != "Increase Count" {
		t.Errorf("plus spoken as %q", got)
	}
}
