// Package conform is the conformance suite every interactive giokit control
// must pass. A control is accepted when it has a row in a table of Cases and
// that row is green:
//
//	conform.Run(t, conform.Case{
//		Name: "button", ID: "save", Role: semantic.Button,
//		Build: func(th *kit.Theme, disabled bool) (func(kit.C), *int) { ... },
//	})
//
// Per case it checks: an id, the role, and a spoken label that is not the id;
// hover changes pixels; one click fires once; Tab reaches it; Enter and Space
// activate it; keyboard focus draws a ring; disabled blocks clicks, keys and
// Tab and says so; and the structural audit is clean under every theme
// preset, at scale 1, 1.5 and 2, at density 0.8 and 1.2, and in a narrow
// window.
//
// It lives apart from kittest because it needs package testing (subtests),
// which apps that use kittest for screenshots must not link.
package conform

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kittest"
)

// Case is one interactive control.
type Case struct {
	Name string           // subtest name
	ID   string           // the control's id as tests address it
	Role semantic.ClassOp // its semantic class
	// Label is the expected spoken label; empty accepts any non-empty label
	// other than the id.
	Label string
	// Build lays the control out with th, wrapped in th.Disable when
	// disabled. The frame must read the control's clicks first (rule 1) and
	// count each one in *activations.
	Build func(th *kit.Theme, disabled bool) (frame func(gtx kit.C), activations *int)
	// NoHover: the control has no hover feedback (rare: say why in the table).
	NoHover bool
	// NoKeys: Enter and Space do not activate it (a slider moves instead).
	NoKeys bool
	// Size is the window in dp; default 360×140. The control sits 24dp in
	// from the top left, so the pointer can rest outside it.
	Size image.Point
}

// Run checks every case as subtests of t.
func Run(t *testing.T, cases ...Case) {
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { c.run(t) })
	}
}

func (c Case) size() image.Point {
	if c.Size == (image.Point{}) {
		return image.Pt(360, 140)
	}
	return c.Size
}

func (c Case) harness(t *testing.T, th *kit.Theme, disabled bool, w int, scale float32) (*kittest.Harness, *int) {
	t.Helper()
	frame, n := c.Build(th, disabled)
	h := kittest.NewTAt(t, w, c.size().Y, scale, func(gtx layout.Context) {
		paint.Fill(gtx.Ops, th.Bg) // as an app's window does
		layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			frame(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
	h.Theme = th
	return h, n
}

func (c Case) run(t *testing.T) {
	th := kit.Dark()
	t.Run("behaviour", func(t *testing.T) { c.behaviour(t, th) })
	t.Run("ring", func(t *testing.T) { c.ring(t, th) })
	t.Run("disabled", func(t *testing.T) { c.disabled(t, th) })
	for _, p := range kit.Presets() {
		t.Run("preset/"+p.Name, func(t *testing.T) {
			h, _ := c.harness(t, p, false, c.size().X, 1)
			h.ExpectClean()
		})
	}
	for _, s := range []float32{1.5, 2} {
		t.Run(fmt.Sprintf("scale/%g", s), func(t *testing.T) {
			h, _ := c.harness(t, th, false, c.size().X, s)
			h.ExpectClean()
		})
	}
	for _, d := range []float32{0.8, 1.2} {
		t.Run(fmt.Sprintf("density/%g", d), func(t *testing.T) {
			h, _ := c.harness(t, th.Derive(func(t *kit.Theme) { t.Density = d }), false, c.size().X, 1)
			h.ExpectClean()
		})
	}
	t.Run("narrow", func(t *testing.T) {
		h, _ := c.harness(t, th, false, 200, 1)
		h.ExpectClean()
	})
	// Shape, stroke, size, elevation, font and contrast at their extremes:
	// still clean, and the ring still visible.
	for _, s := range styles() {
		t.Run("style/"+s.Name, func(t *testing.T) {
			h, _ := c.harness(t, s, false, c.size().X, 1)
			h.ExpectClean()
		})
		t.Run("style/"+s.Name+"/ring", func(t *testing.T) { c.ring(t, s) })
	}
}

// styles are themes at the edges of the theme's knobs.
func styles() []*kit.Theme {
	pill := kit.Dark().Derive(func(t *kit.Theme) {
		t.Name, t.Radius, t.Stroke, t.Ring, t.Controls = "pill", 999, 1.5, 3, kit.ControlLarge
	})
	pill.UseFonts(fonts.Inter())
	sharp := kit.Light().Derive(func(t *kit.Theme) {
		t.Name, t.Radius, t.Elevation, t.Controls, t.HeadingWeight = "sharp", 0, 0, kit.ControlSmall, kit.SemiBold
	})
	sharp.UseFonts(fonts.Plex())
	legible := kit.Dark().Derive(func(t *kit.Theme) {
		t.Name, t.Palette = "high-contrast-atkinson", t.Palette.WithContrast(true, 1)
		t.Type = t.Type.Scaled(1.25)
	})
	legible.UseFonts(fonts.Atkinson())
	return []*kit.Theme{pill, sharp, legible}
}

func (c Case) behaviour(t *testing.T, th *kit.Theme) {
	h, n := c.harness(t, th, false, c.size().X, 1)
	r, ok := h.Find(c.ID)
	if !ok {
		t.Fatalf("no widget %q; ids: %v", c.ID, h.IDs())
	}
	node, _ := find(h.A11yTree(), c.ID)
	switch {
	case node.Class != c.Role:
		t.Errorf("role %v, want %v", node.Class, c.Role)
	case node.Label == "" || node.Label == c.ID:
		t.Errorf("spoken label %q: needs a human label, never the id", node.Label)
	case c.Label != "" && node.Label != c.Label:
		t.Errorf("spoken label %q, want %q", node.Label, c.Label)
	}
	h.ExpectClean()

	rest := crop(h, r)
	if !c.NoHover {
		h.Hover(c.ID)
		if bytes.Equal(rest, crop(h, r)) {
			t.Error("hover changes nothing on screen")
		}
		h.Move(image.Pt(2, 2))
	}

	*n = 0
	h.ClickID(c.ID)
	if *n != 1 {
		t.Errorf("one click fired %d times", *n)
	}
	h.Move(image.Pt(2, 2))

	h.ExpectTabReachable()
	if !focus(h, c.ID) {
		t.Fatalf("Tab never focused %q", c.ID)
	}
	if !c.NoKeys {
		for _, k := range []key.Name{key.NameReturn, key.NameSpace} {
			*n = 0
			h.Key(k, 0)
			if *n != 1 {
				t.Errorf("%s fired %d times, want 1", k, *n)
			}
		}
	}
}

// ring: keyboard focus must be visible. Its own harness, so no activation
// has changed the control's state between the two pictures.
func (c Case) ring(t *testing.T, th *kit.Theme) {
	h, _ := c.harness(t, th, false, c.size().X, 1)
	r, ok := h.Find(c.ID)
	if !ok {
		t.Fatalf("no widget %q", c.ID)
	}
	blur := crop(h, r)
	if !focus(h, c.ID) {
		t.Fatalf("Tab never focused %q", c.ID)
	}
	// A ring is visible when a real outline changes clearly (even a 2dp ring
	// round an 18dp checkbox is over 100 pixels), not when a few anti-aliased
	// corner pixels do.
	changed, want := visiblyChanged(blur, crop(h, r)), 40
	if changed < want {
		t.Errorf("keyboard focus draws no visible ring: %d pixels changed clearly, want at least %d", changed, want)
	}
}

// visiblyChanged counts pixels whose colour moved by more than 40 in some
// channel between two crops of the same rectangle.
func visiblyChanged(a, b []byte) int {
	n := 0
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		for c := range 3 {
			d := int(a[i+c]) - int(b[i+c])
			if d > 40 || d < -40 {
				n++
				break
			}
		}
	}
	return n
}

func (c Case) disabled(t *testing.T, th *kit.Theme) {
	h, n := c.harness(t, th, true, c.size().X, 1)
	if _, ok := h.Find(c.ID); !ok {
		t.Fatalf("disabled control %q not on screen", c.ID)
	}
	if !h.Disabled(c.ID) {
		t.Error("semantics do not say disabled")
	}
	h.ClickID(c.ID)
	if *n != 0 {
		t.Errorf("disabled control fired %d times on click", *n)
	}
	if focus(h, c.ID) {
		t.Error("Tab focused a disabled control")
	}
	h.Key(key.NameReturn, 0)
	h.Key(key.NameSpace, 0)
	if *n != 0 {
		t.Errorf("disabled control fired %d times on keys", *n)
	}
}

// focus presses Tab until id has focus, up to a full cycle.
func focus(h *kittest.Harness, id string) bool {
	for range 12 {
		if h.FocusedID() == id {
			return true
		}
		h.Tab(false)
	}
	return h.FocusedID() == id
}

func crop(h *kittest.Harness, r image.Rectangle) []byte {
	img := h.Image()
	r = r.Inset(-4).Intersect(img.Bounds()) // rings may sit just outside
	var out []byte
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := img.PixOffset(r.Min.X, y)
		out = append(out, img.Pix[i:i+4*r.Dx()]...)
	}
	return out
}

func find(nodes []kittest.A11yNode, id string) (kittest.A11yNode, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
		if m, ok := find(n.Children, id); ok {
			return m, true
		}
	}
	return kittest.A11yNode{}, false
}

// Clickable builds a Case's Build around one Clickable: draw lays the
// control out, and every click it reports counts as an activation.
func Clickable(draw func(th *kit.Theme, gtx kit.C, c *kit.Clickable) kit.D) func(*kit.Theme, bool) (func(kit.C), *int) {
	return func(th *kit.Theme, disabled bool) (func(kit.C), *int) {
		var c kit.Clickable
		n := new(int)
		return func(gtx kit.C) {
			if c.Clicked(gtx) {
				*n++
			}
			th.Disable(gtx, disabled, func(gtx kit.C) kit.D { return draw(th, gtx, &c) })
		}, n
	}
}
