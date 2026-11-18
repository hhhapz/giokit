package kittest

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"

	"gioui.org/io/input"
	"gioui.org/io/semantic"

	"github.com/hhhapz/giokit/kit"
)

// Find returns the pixel bounds of the widget tagged with semantic id.
func (h *Harness) Find(id string) (image.Rectangle, bool) {
	n, ok := h.node(func(d input.SemanticDesc) bool { return d.Description == id })
	return n.Desc.Bounds, ok
}

// Disabled reports whether the widget tagged id is disabled (kit's
// Theme.Disable), itself or through an ancestor. It fails the test if the id
// isn't on screen.
func (h *Harness) Disabled(id string) bool {
	h.tb.Helper()
	h.must(id)
	t := h.semantics()
	for _, n := range t.nodes {
		if n.Desc.Description == id {
			return t.disabled(n)
		}
	}
	return false
}

// focusedTagID is the id of the control holding keyboard focus, as kit
// recorded it (kit.FocusID), or "" when nothing with an id has focus.
func (h *Harness) focusedTagID() string {
	return h.kwin.FocusedID(h.router.Source())
}

// FocusedID is the id of the focused widget. When focus is on a focus group
// (a list, tree, table or menu), it is the group's active item if that item
// is laid out, as a screen reader would announce it; otherwise the group.
func (h *Harness) FocusedID() string {
	id := h.focusedTagID()
	if a := h.kwin.GroupActive()[id]; a != "" {
		return a
	}
	return id
}

// Role is the role of the widget tagged id: as kit recorded it
// (kit.SetRole; every kit component does), else derived from its Gio
// semantic class, else "".
func (h *Harness) Role(id string) kit.Role {
	if r, ok := h.kwin.Roles()[id]; ok {
		return r
	}
	n, ok := h.node(func(d input.SemanticDesc) bool { return d.Description == id })
	if !ok {
		return ""
	}
	return classRole[n.Desc.Class]
}

var classRole = map[semantic.ClassOp]kit.Role{
	semantic.Button: kit.RoleButton, semantic.CheckBox: kit.RoleCheckbox,
	semantic.Editor: kit.RoleTextbox, semantic.RadioButton: kit.RoleRadio, semantic.Switch: kit.RoleSwitch,
}

// ByRole returns the id of the one control with role and spoken label (as a
// screen reader announces it: "the Save button", "the Editor tab"), and
// fails when there is none or several. Querying this way checks the
// accessibility on the way: an unlabelled control cannot be found.
func (h *Harness) ByRole(role kit.Role, label string) string {
	h.tb.Helper()
	var ids []string
	for _, n := range h.nodes() {
		if id := n.Desc.Description; id != "" && n.Desc.Label == label && h.Role(id) == role {
			ids = append(ids, id)
		}
	}
	switch len(ids) {
	case 1:
		return ids[0]
	case 0:
		h.fatal(fmt.Sprintf("no %s labelled %q on screen", role, label))
	default:
		h.fatal(fmt.Sprintf("%d controls are %s labelled %q: %v (disambiguate by id)", len(ids), role, label, ids))
	}
	return ""
}

// ClickRole clicks the control with role and spoken label (ByRole).
func (h *Harness) ClickRole(role kit.Role, label string) {
	h.tb.Helper()
	h.ClickID(h.ByRole(role, label))
}

// FindText returns the bounds of the first text label exactly equal to s.
func (h *Harness) FindText(s string) (image.Rectangle, bool) {
	n, ok := h.node(func(d input.SemanticDesc) bool { return d.Label == s })
	return n.Desc.Bounds, ok
}

// TextsIn returns the labels whose origin lies inside the widget tagged id.
// A label's semantic bounds are the constraints it was laid out in, often
// larger than its glyphs and its container, so containment would miss them.
func (h *Harness) TextsIn(id string) []string {
	h.tb.Helper()
	r := h.must(id)
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Label != "" && n.Desc.Bounds.Min.In(r) {
			out = append(out, n.Desc.Label)
		}
	}
	return out
}

// Texts returns every label on screen, in draw order.
func (h *Harness) Texts() []string {
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Label != "" {
			out = append(out, n.Desc.Label)
		}
	}
	return out
}

// IDs returns every semantic id on screen, in draw order.
func (h *Harness) IDs() []string {
	var out []string
	for _, n := range h.nodes() {
		if n.Desc.Description != "" {
			out = append(out, n.Desc.Description)
		}
	}
	return out
}

// mustVisible is the part of the widget tagged id that is on screen: its
// bounds clipped by its ancestors' (a scrolled list's viewport) and the
// window. Pointer helpers aim there; none visible fails the test.
func (h *Harness) mustVisible(id string) image.Rectangle {
	h.tb.Helper()
	h.must(id)
	t := h.semantics()
	screen := image.Rectangle{Max: h.Size}
	for _, n := range t.nodes {
		if n.Desc.Description != id {
			continue
		}
		if v := t.visible(n, screen); !v.Empty() {
			return v
		}
		h.fatal(fmt.Sprintf("widget %q is not visible: bounds %v are clipped out of view (scroll it into view first)", id, n.Desc.Bounds))
	}
	return h.must(id)
}

func (h *Harness) must(id string) image.Rectangle {
	h.tb.Helper()
	r, ok := h.Find(id)
	if !ok {
		h.fatal(fmt.Sprintf("no widget %q on screen; ids: %v", id, h.IDs()))
	}
	return r
}

// nodes flattens the semantic tree in pre-order (draw order).
func (h *Harness) nodes() []input.SemanticNode {
	return slices.Collect(preorder(semanticRoot(&h.router)))
}

// node returns the first node, in draw order, whose description matches.
func (h *Harness) node(match func(input.SemanticDesc) bool) (input.SemanticNode, bool) {
	for n := range preorder(semanticRoot(&h.router)) {
		if match(n.Desc) {
			return n, true
		}
	}
	return input.SemanticNode{}, false
}

// Image returns the current frame.
func (h *Harness) Image() *image.RGBA {
	h.tb.Helper()
	img := image.NewRGBA(image.Rectangle{Max: h.Size})
	if err := h.win.Screenshot(img); err != nil {
		h.fatal(fmt.Sprintf("kittest: read back frame %d: %v", h.Frames, err))
	}
	return img
}

// Pixel samples the current frame at p.
func (h *Harness) Pixel(p image.Point) color.NRGBA {
	h.tb.Helper()
	// Frames are opaque, so premultiplied and straight alpha agree.
	return color.NRGBA(h.Image().RGBAAt(p.X, p.Y))
}

// bgSampleInset is how far inside a widget's left edge BgColor samples, in
// pixels: past a border, before content.
const bgSampleInset = 3

// BgColor samples a widget's background 3px inside its left edge at mid
// height, where content rarely is. Use it to check hover/selection fills.
func (h *Harness) BgColor(id string) color.NRGBA {
	h.tb.Helper()
	r := h.must(id)
	return h.Pixel(image.Pt(r.Min.X+bgSampleInset, Center(r).Y))
}

// Save writes the current frame as a PNG.
func (h *Harness) Save(path string) error {
	h.tb.Helper()
	return writePNG(path, h.Image())
}

// writePNG encodes img to a new file at path, reporting write and close
// failures.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return f.Close()
}

// Center is the middle point of r.
func Center(r image.Rectangle) image.Point { return r.Min.Add(r.Size().Div(2)) }

// Near reports whether two colors match within tol per channel (GPU rounding).
func Near(a, b color.NRGBA, tol uint8) bool {
	d := func(x, y uint8) bool { return max(x, y)-min(x, y) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}
