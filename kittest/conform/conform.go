// Package conform is the conformance suite every interactive giokit control
// must pass. A control is accepted when it has a row in a table of Cases and
// that row is green:
//
//	conform.Run(t, conform.Case{
//		Name: "button", ID: "save", Class: semantic.Button, Role: kit.RoleButton,
//		Build: func(th *kit.Theme, disabled bool) conform.Control { ... },
//	})
//
// Per case it checks: an id, the semantic class, the kit role, and a spoken
// label that is not the id; hover changes pixels; one click fires once; Tab
// reaches it; Enter and Space activate it; keyboard focus draws a ring;
// disabled blocks clicks, keys and Tab and says so; and the structural audit
// is clean under every theme preset, at scale 1, 1.5 and 2, at density 0.8
// and 1.2, and in a narrow window. Composite (controls that open a layer)
// and Group (focus-group collections) have contracts of their own.
//
// Keeping the conformance suite in a separate package lets screenshot tools
// use kittest without importing testing.
package conform

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kittest"
)

// Control is a control under test, as a builder returns it.
type Control struct {
	// Frame draws one frame. It handles the control's input first (read its
	// events, then lay it out) and counts every activation in *Activations.
	Frame func(gtx layout.Context)
	// Activations counts the control's activations: clicks, key presses,
	// picks, as its events report them. The builder allocates it; the suite
	// resets it to 0 before an input and reads it after.
	Activations *int
}

// Builder makes a fresh control laid out with th. When disabled, the frame
// wraps the control in th.Disable. Each call returns independent state: the
// suite builds one control per harness.
type Builder func(th *kit.Theme, disabled bool) Control

// Case is one interactive control.
type Case struct {
	Name  string           // subtest name
	ID    string           // the control's id as tests address it
	Class semantic.ClassOp // its Gio semantic class (semantic.Unknown for none)
	// Role is the role the control must report (Harness.Role: as recorded
	// with kit.SetRole, else derived from Class). Required.
	Role kit.Role
	// Label is the expected spoken label; empty accepts any non-empty label
	// other than the id.
	Label string
	// Build makes the control. Clickable adapts the common case.
	Build Builder
	// NoHover says the control has no hover feedback (rare: say why in the
	// table).
	NoHover bool
	// NoKeys says Enter and Space do not activate it (a slider moves
	// instead).
	NoKeys bool
	// Size is the window in dp; default 360×140. The control sits 24dp in
	// from the top left, so the pointer can rest outside it.
	Size image.Point
}

// Window layout shared by every contract.
const (
	// inset is how far the control sits from the window's top left, in dp.
	inset = 24
	// narrowWidth is the window width, in dp, of the narrow-window check.
	narrowWidth = 200
	// maxTabs is how many Tab presses focus may take to reach a control.
	maxTabs = 12
)

var (
	caseSize = image.Pt(360, 140) // default Case window, dp
	// rest is where the pointer waits between checks, in pixels: inside the
	// window, outside the inset control.
	rest = image.Pt(2, 2)
)

// Focus ring visibility: a ring is visible when a real outline changes
// clearly (even a 2dp ring round an 18dp checkbox is over 100 pixels), not
// when a few antialiased corner pixels do.
const (
	ringMargin       = 4  // px around the control's bounds where a ring may sit
	ringChannelDelta = 40 // per-channel change that makes a pixel clearly different
	ringMinPixels    = 40 // clearly changed pixels a visible ring needs
)

// stage is the window a contract opens a control in.
type stage struct {
	size  image.Point // dp
	scale float32     // pixels per dp
	bg    color.NRGBA // the window (Bg) or a card (Panel)
	// stretch passes the window's minimum size to the control, as an app's
	// root does; otherwise the control gets no minimum.
	stretch bool
}

// open builds the control and lays it out in a harness on s.
func (s stage) open(t *testing.T, th *kit.Theme, c Control) *kittest.Harness {
	t.Helper()
	h := kittest.NewTAt(t, s.size.X, s.size.Y, s.scale, func(gtx layout.Context) {
		paint.Fill(gtx.Ops, s.bg) // as an app's window (or a card) does
		layout.UniformInset(inset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !s.stretch {
				gtx.Constraints.Min = image.Point{}
			}
			c.Frame(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
	h.Theme = th
	return h
}

// Run checks every case as subtests of t.
func Run(t *testing.T, cases ...Case) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) { c.run(t) })
	}
}

func (c Case) size() image.Point {
	if c.Size == (image.Point{}) {
		return caseSize
	}
	return c.Size
}

// harness opens the control on th's window background at full width.
func (c Case) harness(t *testing.T, th *kit.Theme, disabled bool, scale float32) (*kittest.Harness, *int) {
	t.Helper()
	return c.harnessOn(t, th, stage{size: c.size(), scale: scale, bg: th.Bg}, disabled)
}

func (c Case) harnessOn(t *testing.T, th *kit.Theme, s stage, disabled bool) (*kittest.Harness, *int) {
	t.Helper()
	ctl := c.Build(th, disabled)
	return s.open(t, th, ctl), ctl.Activations
}

func (c Case) run(t *testing.T) {
	th := kit.Dark()
	t.Run("behaviour", func(t *testing.T) { c.behaviour(t, th) })
	t.Run("ring", func(t *testing.T) { c.ring(t, th) })
	if !c.NoHover {
		// On a card (Panel), hover must still change the control: a hover fill
		// equal to Panel vanishes there.
		t.Run("hover-on-card", func(t *testing.T) {
			h, _ := c.harnessOn(t, th, stage{size: c.size(), scale: 1, bg: th.Panel}, false)
			r, ok := h.Find(c.ID)
			if !ok {
				t.Fatalf("no widget %q", c.ID)
			}
			before := crop(h, r)
			h.Hover(c.ID)
			if bytes.Equal(before, crop(h, r)) {
				t.Error("hover changes nothing on a card (Panel background)")
			}
		})
	}
	t.Run("disabled", func(t *testing.T) { c.disabled(t, th) })
	for _, p := range kit.Presets() {
		t.Run("preset/"+p.Name, func(t *testing.T) {
			h, _ := c.harness(t, p, false, 1)
			h.ExpectClean()
		})
	}
	for _, s := range []float32{1.5, 2} {
		t.Run(fmt.Sprintf("scale/%g", s), func(t *testing.T) {
			h, _ := c.harness(t, th, false, s)
			h.ExpectClean()
		})
	}
	for _, d := range []float32{0.8, 1.2} {
		t.Run(fmt.Sprintf("density/%g", d), func(t *testing.T) {
			h, _ := c.harness(t, th.Derive(func(t *kit.Theme) { t.Density = d }), false, 1)
			h.ExpectClean()
		})
	}
	t.Run("narrow", func(t *testing.T) {
		h, _ := c.harnessOn(t, th, stage{size: image.Pt(narrowWidth, c.size().Y), scale: 1, bg: th.Bg}, false)
		h.ExpectClean()
	})
	// Shape, stroke, size, elevation, font and contrast at their extremes:
	// still clean, and the ring still visible.
	for _, s := range styles() {
		t.Run("style/"+s.Name, func(t *testing.T) {
			h, _ := c.harness(t, s, false, 1)
			h.ExpectClean()
		})
		t.Run("style/"+s.Name+"/ring", func(t *testing.T) { c.ring(t, s) })
	}
}

// styles are themes at the edges of the theme's knobs.
func styles() []*kit.Theme {
	pill := kit.Dark().Derive(func(t *kit.Theme) {
		t.Name = "pill"
		t.Radius = 999
		t.Stroke = 1.5
		t.Ring = 3
		t.Controls = kit.ControlLarge
	})
	pill.UseFonts(fonts.Inter())
	sharp := kit.Light().Derive(func(t *kit.Theme) {
		t.Name = "sharp"
		t.Radius = 0
		t.Elevation = 0
		t.Controls = kit.ControlSmall
		t.HeadingWeight = kit.SemiBold
	})
	sharp.UseFonts(fonts.Plex())
	legible := kit.Dark().Derive(func(t *kit.Theme) {
		t.Name = "high-contrast-atkinson"
		t.Palette = t.Palette.WithContrast(true, 1)
		t.Type = t.Type.Scaled(1.25)
	})
	legible.UseFonts(fonts.Atkinson())
	return []*kit.Theme{pill, sharp, legible}
}

func (c Case) behaviour(t *testing.T, th *kit.Theme) {
	if c.Role == "" {
		t.Fatal("Case.Role is unset: name the kit role the control must record")
	}
	h, n := c.harness(t, th, false, 1)
	r, ok := h.Find(c.ID)
	if !ok {
		t.Fatalf("no widget %q; ids: %v", c.ID, h.IDs())
	}
	node, _ := find(h.A11yTree(), c.ID)
	switch {
	case node.Class != c.Class:
		t.Errorf("semantic class %v, want %v", node.Class, c.Class)
	case node.Label == "" || node.Label == c.ID:
		t.Errorf("spoken label %q: needs a human label, never the id", node.Label)
	case c.Label != "" && node.Label != c.Label:
		t.Errorf("spoken label %q, want %q", node.Label, c.Label)
	}
	switch role := h.Role(c.ID); role {
	case "":
		t.Errorf("no role: set one with kit.SetRole (want %s)", c.Role)
	case c.Role:
	default:
		t.Errorf("role %s, want %s", role, c.Role)
	}
	h.ExpectClean()

	before := crop(h, r)
	if !c.NoHover {
		h.Hover(c.ID)
		if bytes.Equal(before, crop(h, r)) {
			t.Error("hover changes nothing on screen")
		}
		h.Move(rest)
	}

	*n = 0
	h.ClickID(c.ID)
	if *n != 1 {
		t.Errorf("one click fired %d times", *n)
	}
	h.Move(rest)

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

// ring checks that keyboard focus is visible. It uses its own harness, so
// no activation has changed the control's state between the two pictures.
func (c Case) ring(t *testing.T, th *kit.Theme) {
	h, _ := c.harness(t, th, false, 1)
	r, ok := h.Find(c.ID)
	if !ok {
		t.Fatalf("no widget %q", c.ID)
	}
	blur := crop(h, r)
	if !focus(h, c.ID) {
		t.Fatalf("Tab never focused %q", c.ID)
	}
	if changed := visiblyChanged(blur, crop(h, r)); changed < ringMinPixels {
		t.Errorf("keyboard focus draws no visible ring: %d pixels changed clearly, want at least %d", changed, ringMinPixels)
	}
}

// visiblyChanged counts pixels whose colour moved by more than
// ringChannelDelta in some channel between two crops of the same rectangle.
func visiblyChanged(a, b []byte) int {
	n := 0
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		for c := range 3 {
			d := int(a[i+c]) - int(b[i+c])
			if d > ringChannelDelta || d < -ringChannelDelta {
				n++
				break
			}
		}
	}
	return n
}

func (c Case) disabled(t *testing.T, th *kit.Theme) {
	h, n := c.harness(t, th, true, 1)
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

// focus presses Tab until id has focus, up to maxTabs presses.
func focus(h *kittest.Harness, id string) bool {
	for range maxTabs {
		if h.FocusedID() == id {
			return true
		}
		h.Tab(false)
	}
	return h.FocusedID() == id
}

// crop copies the frame's pixels in r grown by ringMargin.
func crop(h *kittest.Harness, r image.Rectangle) []byte {
	img := h.Image()
	r = r.Inset(-ringMargin).Intersect(img.Bounds())
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

// Clickable adapts a control driven by one kit.Clickable into a Builder.
// The builder owns the Clickable: each frame reads its clicks first, counting
// each as an activation, then calls draw inside th.Disable(disabled) to lay
// the control out with that Clickable.
func Clickable(draw func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions) Builder {
	return func(th *kit.Theme, disabled bool) Control {
		var c kit.Clickable
		n := new(int)
		return Control{
			Frame: func(gtx layout.Context) {
				if c.Clicked(gtx) {
					*n++
				}
				th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions { return draw(th, gtx, &c) })
			},
			Activations: n,
		}
	}
}
