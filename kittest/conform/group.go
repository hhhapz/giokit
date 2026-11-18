package conform

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"gioui.org/io/key"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// Group is a focus-group collection (a list, tree or table): one Tab stop
// whose active row is the selection. Its contract: Tab lands on the selected
// row (First, selected initially), Down selects Second and reports one
// pick, a click on First selects it again (one pick), the rows are tagged
// and labelled, disabled blocks Tab, keys and clicks, and the page is clean
// under every preset, scale 2 and the extreme styles.
type Group struct {
	Name          string // subtest name
	First, Second string // row ids; First starts selected, Second is next
	// Build makes the collection; its Activations count picks reported by
	// Update.
	Build Builder
	Size  image.Point // window in dp; default 420×300
}

var groupSize = image.Pt(420, 300) // default Group window, dp

// groupTabs is how many Tab presses the disabled check makes looking for a
// way into the collection.
const groupTabs = 6

// RunGroup runs each group's contract as subtests.
func RunGroup(t *testing.T, cases ...Group) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { c.run(t) })
	}
}

// stage is the window a group opens in. A collection fills the space it
// is given, so it keeps the window's minimum size.
func (c Group) stage(bg color.NRGBA, scale float32) stage {
	size := c.Size
	if size == (image.Point{}) {
		size = groupSize
	}
	return stage{size: size, scale: scale, bg: bg, stretch: true}
}

func (c Group) harness(t *testing.T, th *kit.Theme, disabled bool, scale float32) (*kittest.Harness, *int) {
	t.Helper()
	return c.harnessOn(t, th, c.stage(th.Bg, scale), disabled)
}

func (c Group) harnessOn(t *testing.T, th *kit.Theme, s stage, disabled bool) (*kittest.Harness, *int) {
	t.Helper()
	ctl := c.Build(th, disabled)
	return s.open(t, th, ctl), ctl.Activations
}

func (c Group) run(t *testing.T) {
	th := kit.Dark()
	t.Run("contract", func(t *testing.T) {
		h, n := c.harness(t, th, false, 1)
		for _, id := range []string{c.First, c.Second} {
			node, ok := find(h.A11yTree(), id)
			if !ok {
				t.Fatalf("no row %q; ids: %v", id, h.IDs())
			}
			if node.Label == "" || node.Label == id {
				t.Errorf("row %q spoken as %q: needs a human label", id, node.Label)
			}
			if r := h.Role(id); r != kit.RoleOption && r != kit.RoleTreeItem && r != kit.RoleRow {
				t.Errorf("row %q has role %q, want option, treeitem or row", id, r)
			}
		}
		h.ExpectClean()
		h.ExpectTabReachable()
		if !focus(h, c.First) {
			t.Fatalf("Tab never landed on the selected row %q", c.First)
		}
		*n = 0
		h.Key(key.NameDownArrow, 0)
		if *n != 1 {
			t.Errorf("Down reported %d picks, want 1", *n)
		}
		if f := h.FocusedID(); f != c.Second {
			t.Errorf("after Down: focus %q, want %q", f, c.Second)
		}
		*n = 0
		h.ClickID(c.First)
		if *n != 1 {
			t.Errorf("a click reported %d picks, want 1", *n)
		}
		h.ExpectClean()
	})
	// Row hover shows on a card (Panel) too.
	t.Run("hover-on-card", func(t *testing.T) {
		h, _ := c.harnessOn(t, th, c.stage(th.Panel, 1), false)
		r := h.ExpectID(c.Second)
		img := func() []byte { return crop(h, r) }
		before := img()
		h.Hover(c.Second)
		if bytes.Equal(before, img()) {
			t.Error("row hover changes nothing on a card (Panel background)")
		}
	})
	t.Run("disabled", func(t *testing.T) {
		h, n := c.harness(t, th, true, 1)
		*n = 0
		h.ClickID(c.Second)
		for range groupTabs {
			h.Tab(false)
			if f := h.FocusedID(); f == c.First || f == c.Second {
				t.Fatalf("Tab reached disabled row %q", f)
			}
		}
		h.Key(key.NameDownArrow, 0)
		if *n != 0 {
			t.Errorf("disabled collection reported %d picks", *n)
		}
	})
	clean := func(t *testing.T, th *kit.Theme, scale float32) {
		h, _ := c.harness(t, th, false, scale)
		h.ExpectClean()
	}
	for _, p := range kit.Presets() {
		t.Run("preset/"+p.Name, func(t *testing.T) { clean(t, p, 1) })
	}
	t.Run("scale/2", func(t *testing.T) { clean(t, th, 2) })
	for _, s := range styles() {
		t.Run("style/"+s.Name, func(t *testing.T) { clean(t, s, 1) })
	}
}
