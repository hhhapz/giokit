package conform

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// Composite is a control whose activation opens a layer and moves focus
// into it: a menu, a select. Its contract: the trigger opens the layer with
// a click and with Enter, a key moves to an item, Enter picks it (once),
// Escape dismisses, and focus returns to the trigger either way; the open
// layer is clean under every preset, scale and extreme style.
type Composite struct {
	Name    string
	Trigger string // the opener's id
	Panel   string // the open layer's id
	// Item is the id focused after opening and pressing Down once.
	Item string
	// Build returns the frame and a counter of picks.
	Build func(th *kit.Theme) (frame func(gtx kit.C), picks *int)
	Size  image.Point // window in dp; default 420×360
}

// RunComposite runs each composite's contract as subtests.
func RunComposite(t *testing.T, cases ...Composite) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { c.run(t) })
	}
}

func (c Composite) size() image.Point {
	if c.Size == (image.Point{}) {
		return image.Pt(420, 360)
	}
	return c.Size
}

func (c Composite) harness(t *testing.T, th *kit.Theme, scale float32) (*kittest.Harness, *int) {
	return c.harnessMin(t, th, scale, false)
}

// harnessMin is harness; with stretch the control gets the window's whole
// size as its minimum, as an app's root does.
func (c Composite) harnessMin(t *testing.T, th *kit.Theme, scale float32, stretch bool) (*kittest.Harness, *int) {
	t.Helper()
	frame, n := c.Build(th)
	h := kittest.NewTAt(t, c.size().X, c.size().Y, scale, func(gtx layout.Context) {
		paint.Fill(gtx.Ops, th.Bg)
		layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !stretch {
				gtx.Constraints.Min = image.Point{}
			}
			frame(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
	h.Theme = th
	return h, n
}

func (c Composite) run(t *testing.T) {
	th := kit.Dark()
	t.Run("contract", func(t *testing.T) { c.contract(t, th) })
	open := func(t *testing.T, th *kit.Theme, scale float32) {
		h, _ := c.harness(t, th, scale)
		h.ClickID(c.Trigger)
		if _, ok := h.Find(c.Panel); !ok {
			t.Fatalf("click did not open %q", c.Panel)
		}
		h.ExpectClean()
	}
	for _, p := range kit.Presets() {
		t.Run("preset/"+p.Name, func(t *testing.T) { open(t, p, 1) })
	}
	t.Run("scale/2", func(t *testing.T) { open(t, th, 2) })
	// A minimum size from the caller (a window's) must not stretch the
	// trigger and leave the layer no room for its items.
	t.Run("stretched", func(t *testing.T) {
		h, n := c.harnessMin(t, th, 1, true)
		h.ClickID(c.Trigger)
		if _, ok := h.Find(c.Panel); !ok {
			t.Fatalf("click did not open %q", c.Panel)
		}
		*n = 0
		h.ClickID(c.Item) // fails if the item is laid out out of view
		if *n != 1 {
			t.Errorf("clicking %q under a minimum size picked %d times, want 1", c.Item, *n)
		}
	})
	for _, s := range styles() {
		t.Run("style/"+s.Name, func(t *testing.T) { open(t, s, 1) })
	}
}

func (c Composite) contract(t *testing.T, th *kit.Theme) {
	h, n := c.harness(t, th, 1)
	h.ExpectClean()
	if _, ok := h.Find(c.Panel); ok {
		t.Fatalf("%q is open before the trigger was used", c.Panel)
	}

	// Pointer: open, key into the items, pick with Enter.
	h.ClickID(c.Trigger)
	if _, ok := h.Find(c.Panel); !ok {
		t.Fatalf("click did not open %q", c.Panel)
	}
	h.Key(key.NameDownArrow, 0)
	if f := h.FocusedID(); f != c.Item {
		t.Fatalf("after Down: focus %q, want %q", f, c.Item)
	}
	*n = 0
	h.Key(key.NameReturn, 0)
	if *n != 1 {
		t.Errorf("Enter picked %d times, want 1", *n)
	}
	if _, ok := h.Find(c.Panel); ok {
		t.Errorf("a pick left %q open", c.Panel)
	}
	if f := h.FocusedID(); f != c.Trigger {
		t.Errorf("after a pick: focus %q, want the trigger %q", f, c.Trigger)
	}

	// Keyboard: Enter on the trigger opens, Escape dismisses onto it.
	h.Key(key.NameReturn, 0)
	if _, ok := h.Find(c.Panel); !ok {
		t.Fatalf("Enter on the trigger did not open %q", c.Panel)
	}
	h.Key(key.NameEscape, 0)
	if _, ok := h.Find(c.Panel); ok {
		t.Errorf("Escape left %q open", c.Panel)
	}
	if f := h.FocusedID(); f != c.Trigger {
		t.Errorf("after Escape: focus %q, want the trigger %q", f, c.Trigger)
	}
	if *n != 1 {
		t.Errorf("dismissing picked something (%d picks)", *n)
	}
	h.ExpectClean()
}
