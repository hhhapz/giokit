package kittest

import (
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/semantic"

	"github.com/hhhapz/giokit/kit"
)

// ProblemKind names the rule a Problem breaks. Its value is the short name
// used in failure messages.
type ProblemKind string

// Strict-mode kinds, found while the harness settles after input.
const (
	// ProblemStaleFrame means a frame drawn with no new input differs from
	// the settled one, so the settled frame showed stale state.
	ProblemStaleFrame ProblemKind = "stale-frame"
	// ProblemNoSettle means the UI kept requesting immediate redraws.
	ProblemNoSettle ProblemKind = "no-settle"
)

// Audit kinds, found by Audit in the current frame; Audit describes each.
const (
	ProblemUnlabeledClickable ProblemKind = "unlabeled-clickable"
	ProblemTinyTarget         ProblemKind = "tiny-target"
	ProblemOccluded           ProblemKind = "occluded"
	ProblemDuplicateID        ProblemKind = "duplicate-id"
	ProblemMissingGlyph       ProblemKind = "missing-glyph"
	ProblemLowContrast        ProblemKind = "low-contrast"
)

// Problem is one audit or strict-mode finding.
type Problem struct {
	Kind   ProblemKind     // what kind of finding this is
	ID     string          // the widget's id, when it has one
	Bounds image.Rectangle // the widget's bounds in pixels, when known
	Msg    string          // what is wrong, for a human
}

// String formats p as "kind id at bounds: msg", leaving out what is unknown.
func (p Problem) String() string {
	s := string(p.Kind)
	if p.ID != "" {
		s += " " + p.ID
	}
	if !p.Bounds.Empty() {
		s += " at " + p.Bounds.String()
	}
	return s + ": " + p.Msg
}

// Audit inspects the current frame for structural problems:
//   - unlabeled-clickable: clickable without a semantic id (untestable, mute to screen readers)
//   - tiny-target: clickable smaller than MinTarget (often a collapsed Stack child)
//   - occluded: another interactive area sits on top of a clickable's center
//   - duplicate-id: two widgets share an id
//   - missing-glyph: a label uses runes the theme's fonts can't draw (tofu)
//   - low-contrast: an enabled label's text is under MinContrast against its
//     background, measured from pixels (off while MinContrast is 0)
func (h *Harness) Audit() []Problem {
	h.tb.Helper()
	var out []Problem
	t := h.semantics()
	seen := map[string]bool{}
	minPx := h.Metric.Dp(h.MinTarget)
	layers := t.openLayers()
	for _, n := range t.nodes {
		d := n.Desc
		if d.Description != "" {
			if seen[d.Description] {
				out = append(out, Problem{Kind: ProblemDuplicateID, ID: d.Description, Bounds: d.Bounds, Msg: "id used more than once"})
			}
			seen[d.Description] = true
		}
		if h.Theme != nil && d.Label != "" {
			if m := h.Theme.MissingGlyphs(d.Label); len(m) > 0 {
				out = append(out, Problem{Kind: ProblemMissingGlyph, Bounds: d.Bounds, Msg: fmt.Sprintf("%q can't draw %q", d.Label, string(m))})
			}
		}
		if d.Gestures&input.ClickGesture == 0 || d.Disabled {
			continue
		}
		if d.Description == "" && !t.layered(n) && !(d.Class == semantic.Editor && t.labeledAncestor(n)) {
			out = append(out, Problem{Kind: ProblemUnlabeledClickable, Bounds: d.Bounds, Msg: "clickable has no id (use kit.Tag / Press)"})
		}
		if sz := d.Bounds.Size(); (sz.X < minPx || sz.Y < minPx) && !t.bigClickableAncestor(n, minPx) {
			out = append(out, Problem{Kind: ProblemTinyTarget, ID: d.Description, Bounds: d.Bounds, Msg: fmt.Sprintf("%v px is under %v dp", sz, h.MinTarget)})
		}
		// The part of the widget its scroll containers let through: a row
		// half scrolled under a header is clipped there, not covered by it.
		vis := t.visible(n, image.Rectangle{Max: h.Size})
		if _, backdrop := backdropOf(d.Description); vis.Empty() || backdrop {
			continue
		}
		c := Center(vis)
		owner := n.ID // a layered handler belongs to its parent widget
		if t.layered(n) {
			owner = n.ParentID
		}
		if hit, ok := h.router.SemanticAt(f32.Pt(float32(c.X), float32(c.Y))); ok && hit != n.ID && hit != owner && !t.within(hit, owner) {
			// An open layer (a popup's panel and its backdrop) covers the page
			// on purpose: a press outside the panel closes it and goes nowhere
			// else.
			if top := t.byID[hit]; top.Desc.Gestures != 0 && !t.inLayer(layers, top) {
				out = append(out, Problem{Kind: ProblemOccluded, ID: d.Description, Bounds: d.Bounds, Msg: fmt.Sprintf("center covered by %q", top.Desc.Description)})
			}
		}
	}
	return append(out, h.lowContrast()...)
}

// backdropSuffix ends the id of an open popup's backdrop: kit.Popup
// documents the backdrop of panel X as kit.Part(X, "backdrop").
var backdropSuffix = kit.Part("", "backdrop")

// backdropOf reports whether id names a backdrop and, if so, the id of the
// panel it belongs to.
func backdropOf(id string) (panel string, ok bool) {
	return strings.CutSuffix(id, backdropSuffix)
}

// openLayers returns the semantic ids of open floating panels and their
// backdrops.
func (t semanticTree) openLayers() map[input.SemanticID]bool {
	panels := map[string]bool{}
	for _, n := range t.nodes {
		if panel, ok := backdropOf(n.Desc.Description); ok {
			panels[panel] = true
		}
	}
	out := map[input.SemanticID]bool{}
	for _, n := range t.nodes {
		_, backdrop := backdropOf(n.Desc.Description)
		if backdrop || panels[n.Desc.Description] {
			out[n.ID] = true
		}
	}
	return out
}

// inLayer reports whether n is an open layer's panel or backdrop, or inside one.
func (t semanticTree) inLayer(layers map[input.SemanticID]bool, n input.SemanticNode) bool {
	if layers[n.ID] {
		return true
	}
	for p := range t.ancestors(n) {
		if layers[p.ID] {
			return true
		}
	}
	return false
}

// layered reports whether n is an extra handler layer of its parent widget:
// same bounds as a parent that has an id (e.g. a scrollbar's track-click
// layer over its drag layer).
func (t semanticTree) layered(n input.SemanticNode) bool {
	p, ok := t.byID[n.ParentID]
	return ok && p.Desc.Description != "" && p.Desc.Bounds == n.Desc.Bounds
}

// labeledAncestor reports whether some ancestor of n carries an id. Editors
// draw their own hit area inside the field, so the field container's id
// (kit.TextField) names them.
func (t semanticTree) labeledAncestor(n input.SemanticNode) bool {
	for p := range t.ancestors(n) {
		if p.Desc.Description != "" {
			return true
		}
	}
	return false
}

// bigClickableAncestor reports whether n sits inside an enabled clickable
// ancestor at least minPx a side: then the ancestor is the target (a text
// field's padding focuses its editor), and a small inner hit area is no
// problem.
func (t semanticTree) bigClickableAncestor(n input.SemanticNode, minPx int) bool {
	for p := range t.ancestors(n) {
		d := p.Desc
		if d.Gestures&input.ClickGesture != 0 && !d.Disabled && d.Bounds.Dx() >= minPx && d.Bounds.Dy() >= minPx && n.Desc.Bounds.In(d.Bounds) {
			return true
		}
	}
	return false
}

// Dump prints the semantic tree (ids, labels, bounds): the first thing to look
// at when a Find fails or an audit finding is unclear. Nodes with neither an
// id nor a label are skipped, and their children shown one level up.
func (h *Harness) Dump(w io.Writer) {
	var rec func([]input.SemanticNode, int)
	rec = func(ns []input.SemanticNode, depth int) {
		for _, n := range ns {
			d := n.Desc
			named := d.Description != "" || d.Label != ""
			if !named {
				rec(n.Children, depth)
				continue
			}
			marks := ""
			if d.Gestures&input.ClickGesture != 0 {
				marks = " [click]"
			}
			if d.Disabled {
				marks += " [disabled]"
			}
			fmt.Fprintf(w, "%s%q %q %v%s\n", strings.Repeat("  ", depth), d.Description, d.Label, d.Bounds, marks)
			rec(n.Children, depth+1)
		}
	}
	rec(semanticRoot(&h.router), 0)
}
