package kittest

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"io"
	"slices"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
)

// Problem is one audit or strict-mode finding.
type Problem struct {
	Kind   string // stale-frame, no-settle, unlabeled-clickable, tiny-target, occluded, duplicate-id, missing-glyph, low-contrast
	ID     string
	Bounds image.Rectangle
	Msg    string
}

func (p Problem) String() string {
	s := p.Kind
	if p.ID != "" {
		s += " " + p.ID
	}
	if !p.Bounds.Empty() {
		s += " at " + p.Bounds.String()
	}
	return s + ": " + p.Msg
}

// report sends a problem to the test (if attached) or records it for Problems().
func (h *Harness) report(p Problem) {
	if h.tb != nil {
		h.tb.Helper()
		h.tb.Error(p.String())
		return
	}
	h.problems = append(h.problems, p)
}

// Problems returns findings recorded when no test is attached.
func (h *Harness) Problems() []Problem { return h.problems }

func (h *Harness) fatal(msg string) {
	if h.tb != nil {
		h.tb.Helper()
		h.tb.Fatal(msg)
	}
	panic(msg)
}

// ---- staleness ----

type frameSig struct {
	sem string
	img [32]byte
	raw *image.RGBA
}

func (h *Harness) sig() frameSig {
	var b strings.Builder
	for _, n := range h.nodes() {
		fmt.Fprintf(&b, "%s|%s|%v;", n.Desc.Description, n.Desc.Label, n.Desc.Bounds)
	}
	img := h.Image()
	return frameSig{sem: b.String(), img: sha256.Sum256(img.Pix), raw: img}
}

// Stale draws one extra frame with no input and reports what changed, or "".
// A settled UI must be a fixed point: if another frame changes anything, the
// previous frame showed stale state and a real window would sit on it until
// the user moved the mouse.
func (h *Harness) Stale() string {
	before := h.sig()
	h.frame()
	after := h.sig()
	if before.sem != after.sem {
		return "semantics changed: " + diffTexts(before.sem, after.sem)
	}
	if before.img != after.img {
		return fmt.Sprintf("pixels changed in %v", diffRect(before.raw, after.raw))
	}
	return ""
}

func diffTexts(a, b string) string {
	as, bs := strings.Split(a, ";"), strings.Split(b, ";")
	var gone, added []string
	for _, s := range as {
		if !slices.Contains(bs, s) {
			gone = append(gone, s)
		}
	}
	for _, s := range bs {
		if !slices.Contains(as, s) {
			added = append(added, s)
		}
	}
	trim := func(x []string) []string { return x[:min(len(x), 4)] }
	return fmt.Sprintf("-%v +%v", trim(gone), trim(added))
}

func diffRect(a, b *image.RGBA) image.Rectangle {
	var r image.Rectangle
	for y := 0; y < a.Rect.Dy(); y++ {
		for x := 0; x < a.Rect.Dx(); x++ {
			i := a.PixOffset(x, y)
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

// ---- audit ----

// Audit inspects the current frame for structural problems:
//   - unlabeled-clickable: clickable without a semantic id (untestable, mute to screen readers)
//   - tiny-target: clickable smaller than MinTarget (often a collapsed Stack child)
//   - occluded: another interactive area sits on top of a clickable's center
//   - duplicate-id: two widgets share an id
//   - missing-glyph: a label uses runes the theme's fonts can't draw (tofu)
//   - low-contrast: an enabled label's text is under MinContrast against its
//     background, measured from pixels (off while MinContrast is 0)
func (h *Harness) Audit() []Problem {
	var out []Problem
	nodes := h.nodes()
	byID := map[input.SemanticID]input.SemanticNode{}
	seen := map[string]bool{}
	minPx := h.Metric.Dp(h.MinTarget)
	for _, n := range nodes {
		byID[n.ID] = n
	}
	layers := openLayers(nodes)
	for _, n := range nodes {
		d := n.Desc
		if d.Description != "" {
			if seen[d.Description] {
				out = append(out, Problem{Kind: "duplicate-id", ID: d.Description, Bounds: d.Bounds, Msg: "id used more than once"})
			}
			seen[d.Description] = true
		}
		if h.Theme != nil && d.Label != "" {
			if m := h.Theme.MissingGlyphs(d.Label); len(m) > 0 {
				out = append(out, Problem{Kind: "missing-glyph", Bounds: d.Bounds, Msg: fmt.Sprintf("%q can't draw %q", d.Label, string(m))})
			}
		}
		if d.Gestures&input.ClickGesture == 0 || d.Disabled {
			continue
		}
		if d.Description == "" && !layered(byID, n) && !(d.Class == semantic.Editor && labeledAncestor(byID, n)) {
			out = append(out, Problem{Kind: "unlabeled-clickable", Bounds: d.Bounds, Msg: "clickable has no id (use kit.Tag / Press)"})
		}
		if sz := d.Bounds.Size(); (sz.X < minPx || sz.Y < minPx) && !bigClickableAncestor(byID, n, minPx) {
			out = append(out, Problem{Kind: "tiny-target", ID: d.Description, Bounds: d.Bounds, Msg: fmt.Sprintf("%v px is under %v dp", sz, h.MinTarget)})
		}
		// The part of the widget its scroll containers let through: a row
		// half scrolled under a header is clipped there, not covered by it.
		vis := visible(byID, n, image.Rectangle{Max: h.Size})
		if vis.Empty() || strings.HasSuffix(d.Description, "/backdrop") {
			continue
		}
		c := Center(vis)
		owner := n.ID // a layered handler belongs to its parent widget
		if layered(byID, n) {
			owner = n.ParentID
		}
		if hit, ok := h.router.SemanticAt(f32.Pt(float32(c.X), float32(c.Y))); ok && hit != n.ID && hit != owner && !within(byID, hit, owner) {
			// An open layer (a popup's panel and its backdrop) covers the page
			// on purpose: a press outside the panel closes it and goes nowhere
			// else.
			if top := byID[hit]; top.Desc.Gestures != 0 && !inLayer(byID, layers, top) {
				out = append(out, Problem{Kind: "occluded", ID: d.Description, Bounds: d.Bounds, Msg: fmt.Sprintf("center covered by %q", top.Desc.Description)})
			}
		}
	}
	return append(out, h.lowContrast()...)
}

// openLayers returns the semantic ids of open floating panels: kit tags a
// panel X's backdrop "X/backdrop".
func openLayers(nodes []input.SemanticNode) map[input.SemanticID]bool {
	names := map[string]bool{}
	for _, n := range nodes {
		if d := n.Desc.Description; strings.HasSuffix(d, "/backdrop") {
			names[strings.TrimSuffix(d, "/backdrop")] = true
		}
	}
	out := map[input.SemanticID]bool{}
	for _, n := range nodes {
		if names[n.Desc.Description] || strings.HasSuffix(n.Desc.Description, "/backdrop") {
			out[n.ID] = true
		}
	}
	return out
}

// inLayer reports whether n is an open layer's panel or backdrop, or inside one.
func inLayer(byID map[input.SemanticID]input.SemanticNode, layers map[input.SemanticID]bool, n input.SemanticNode) bool {
	for ok := true; ok; n, ok = byID[n.ParentID] {
		if layers[n.ID] {
			return true
		}
		if n.ParentID == 0 {
			break
		}
	}
	return false
}

// layered reports whether n is an extra handler layer of its parent widget:
// same bounds as a parent that has an id (e.g. a scrollbar's track-click
// layer over its drag layer).
func layered(byID map[input.SemanticID]input.SemanticNode, n input.SemanticNode) bool {
	p, ok := byID[n.ParentID]
	return ok && p.Desc.Description != "" && p.Desc.Bounds == n.Desc.Bounds
}

// labeledAncestor reports whether some ancestor of n carries an id. Editors
// draw their own hit area inside the field, so the field container's id
// (kit.TextField) names them.
func labeledAncestor(byID map[input.SemanticID]input.SemanticNode, n input.SemanticNode) bool {
	for p, ok := byID[n.ParentID]; ok; p, ok = byID[p.ParentID] {
		if p.Desc.Description != "" {
			return true
		}
		if p.ParentID == 0 {
			break
		}
	}
	return false
}

// within reports whether node id is a descendant of ancestor.
func within(byID map[input.SemanticID]input.SemanticNode, id, ancestor input.SemanticID) bool {
	for n, ok := byID[id]; ok && n.ParentID != 0; n, ok = byID[n.ParentID] {
		if n.ParentID == ancestor {
			return true
		}
	}
	return false
}

// Dump prints the semantic tree (ids, labels, bounds): the first thing to look
// at when a Find fails or an audit finding is unclear.
func (h *Harness) Dump(w io.Writer) {
	var rec func([]input.SemanticNode, int)
	rec = func(ns []input.SemanticNode, depth int) {
		for _, n := range ns {
			d := n.Desc
			if d.Description != "" || d.Label != "" {
				g := ""
				if d.Gestures&input.ClickGesture != 0 {
					g = " [click]"
				}
				if d.Disabled {
					g += " [disabled]"
				}
				fmt.Fprintf(w, "%s%q %q %v%s\n", strings.Repeat("  ", depth), d.Description, d.Label, d.Bounds, g)
				depth++
			}
			rec(n.Children, depth)
			if d.Description != "" || d.Label != "" {
				depth--
			}
		}
	}
	all := h.router.AppendSemantics(nil)
	if len(all) > 0 {
		rec(all[:1], 0)
	}
}

// ---- assertions (need NewT) ----

func (h *Harness) t() {
	if h.tb == nil {
		panic("kittest: Expect* needs a harness from NewT")
	}
	h.tb.Helper()
}

// ExpectClean fails the test for every Audit finding.
func (h *Harness) ExpectClean() {
	h.t()
	for _, p := range h.Audit() {
		h.tb.Error(p.String())
	}
}

func (h *Harness) ExpectID(id string) image.Rectangle {
	h.t()
	return h.must(id)
}

func (h *Harness) ExpectNoID(id string) {
	h.t()
	if _, ok := h.Find(id); ok {
		h.tb.Errorf("widget %q should not be on screen", id)
	}
}

func (h *Harness) ExpectText(s string) {
	h.t()
	if _, ok := h.FindText(s); !ok {
		h.tb.Errorf("text %q not on screen; texts: %q", s, h.Texts())
	}
}

func (h *Harness) ExpectTextIn(id, s string) {
	h.t()
	if texts := h.TextsIn(id); !slices.Contains(texts, s) {
		h.tb.Errorf("%q shows %q, want %q", id, texts, s)
	}
}

// ExpectBg checks BgColor(id) within ±2 per channel.
func (h *Harness) ExpectBg(id string, want color.NRGBA) {
	h.t()
	if got := h.BgColor(id); !Near(got, want, 2) {
		h.tb.Errorf("%q background = %v, want %v", id, got, want)
	}
}

// FocusEscape presses Tab n times, then Shift+Tab n times, and returns the
// first focused id outside the widget tagged id ("<none>" if focus is lost),
// or "" if focus stayed inside: the focus trap a modal layer must have.
func (h *Harness) FocusEscape(id string, n int) string {
	for i := 0; i < 2*n; i++ {
		h.Tab(i >= n)
		r, ok := h.Find(id)
		if !ok {
			return "<" + id + " closed>"
		}
		f := h.FocusedID()
		if f == "" {
			return "<none>"
		}
		if f == id {
			continue
		}
		if fr, ok := h.Find(f); !ok || !fr.In(r) {
			return f
		}
	}
	return ""
}

// ExpectFocusTrapped fails if Tab or Shift+Tab (n presses each) moves focus
// out of the widget tagged id.
func (h *Harness) ExpectFocusTrapped(id string, n int) {
	h.t()
	if out := h.FocusEscape(id, n); out != "" {
		h.tb.Errorf("focus left %q: reached %q", id, out)
	}
}

// ExpectEscapeCloses presses Escape and fails unless the widget tagged id
// (a popup, menu or dialog) is gone.
func (h *Harness) ExpectEscapeCloses(id string) {
	h.t()
	h.must(id)
	h.Key(key.NameEscape, 0)
	if _, ok := h.Find(id); ok {
		h.tb.Errorf("Escape did not close %q", id)
	}
}

// bigClickableAncestor reports whether n sits inside a clickable ancestor of
// at least min px a side: then the ancestor is the target (a text field's
// padding focuses its editor), and a small inner hit area is no problem.
func bigClickableAncestor(byID map[input.SemanticID]input.SemanticNode, n input.SemanticNode, min int) bool {
	for p, ok := byID[n.ParentID]; ok; p, ok = byID[p.ParentID] {
		d := p.Desc
		if d.Gestures&input.ClickGesture != 0 && !d.Disabled && d.Bounds.Dx() >= min && d.Bounds.Dy() >= min && n.Desc.Bounds.In(d.Bounds) {
			return true
		}
		if p.ParentID == 0 {
			break
		}
	}
	return false
}
