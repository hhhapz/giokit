package kit

import (
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// TestTweenFromKeepsEasing: replaying a transition with From keeps the
// easing the caller chose, rather than switching to EaseOut.
func TestTweenFromKeepsEasing(t *testing.T) {
	now := time.Unix(0, 0)
	gtx := layout.Context{Ops: new(op.Ops), Now: now}
	d := 100 * time.Millisecond
	var tw Tween
	tw.ToEase(gtx, 1, d, Linear)
	tw.From(gtx, 0, d)
	gtx.Now = now.Add(d / 2)
	if v := tw.ToEase(gtx, 1, d, Linear); v != 0.5 {
		t.Errorf("halfway through a linear replay: %v, want 0.5", v)
	}

	var def Tween // From before any To: EaseOut
	gtx.Now = now
	def.From(gtx, 0, d)
	def.To(gtx, 1, d)
	gtx.Now = now.Add(d / 2)
	if v, want := def.Value(gtx), EaseOut(0.5); v != want {
		t.Errorf("default easing halfway: %v, want %v", v, want)
	}
}
