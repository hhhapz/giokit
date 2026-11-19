package conform

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// Composite is a control whose activation opens a layer and moves focus
// into it: a menu, a select. Its contract: the trigger opens the layer with
// a click and with Enter, a key moves to an item, Enter picks it (once),
// Escape dismisses, and focus returns to the trigger either way; the open
// layer is clean under every preset, scale and extreme style.
type Composite struct {
	Name    string // subtest name
	Trigger string // the opener's id
	Panel   string // the open layer's id
	// Item is the id focused after opening and pressing Down once.
	Item string
	// Build makes the control; its Activations count picks.
	Build func(th *kit.Theme) Control
	Size  image.Point // window in dp; default 420×360
}

var compositeSize = image.Pt(420, 360) // default Composite window, dp

// RunComposite runs each composite's contract as subtests.
func RunComposite(t *testing.T, cases ...Composite) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { c.run(t) })
	}
}

func (c Composite) size() image.Point {
	if c.Size == (image.Point{}) {
		return compositeSize
	}
	return c.Size
}

func (c Composite) harness(t *testing.T, th *kit.Theme, scale float32) (*kittest.Harness, *int) {
	t.Helper()
	return c.harnessOn(t, th, stage{size: c.size(), scale: scale, bg: th.Bg})
}

func (c Composite) harnessOn(t *testing.T, th *kit.Theme, s stage) (*kittest.Harness, *int) {
	t.Helper()
	ctl := c.Build(th)
	return s.open(t, th, ctl), ctl.Activations
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
		h, n := c.harnessOn(t, th, stage{size: c.size(), scale: 1, bg: th.Bg, stretch: true})
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
