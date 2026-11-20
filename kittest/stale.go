package kittest

import (
	"crypto/sha256"
	"fmt"
	"image"
	"maps"
	"slices"
	"strings"

	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/semantic"
)

// Stale draws one extra frame with no input and reports what changed, or "".
// A settled UI must be a fixed point: if another frame changes anything, the
// previous frame showed stale state and a real window would sit on it until
// the user moved the mouse. Compared are the semantic tree (ids, labels,
// classes (roles), selection, disabled state, gestures and bounds), focus
// groups and focused id, and every pixel.
func (h *Harness) Stale() string {
	before := h.snapshot()
	h.frame()
	after := h.snapshot()
	if d := before.sem.diff(after.sem); d != "" {
		return "semantics changed: " + d
	}
	if before.sum != after.sum {
		return fmt.Sprintf("pixels changed in %v", diffRect(before.img, after.img))
	}
	return ""
}

// frameSnapshot is what a user, a screen reader or a test can observe of
// one frame.
type frameSnapshot struct {
	sem semanticSnapshot
	sum [sha256.Size]byte
	img *image.RGBA
}

func (h *Harness) snapshot() frameSnapshot {
	img := h.Image()
	return frameSnapshot{sem: h.semanticSnapshot(), sum: sha256.Sum256(img.Pix), img: img}
}

// semanticSnapshot is the observable semantic state of one frame. Nodes
// are identified by their position in draw order, never by the router's
// semantic ids, which are numbering details of one frame.
type semanticSnapshot struct {
	nodes   []semanticNode
	focused string
	groups  map[string][]string
	active  map[string]string
}

// semanticNode is the observable part of one semantic node.
type semanticNode struct {
	depth    int
	id       string
	label    string
	class    semantic.ClassOp
	selected bool
	disabled bool
	gestures input.SemanticGestures
	bounds   image.Rectangle
}

func (h *Harness) semanticSnapshot() semanticSnapshot {
	s := semanticSnapshot{
		focused: h.FocusedID(),
		groups:  map[string][]string{},
		active:  maps.Clone(h.kwin.GroupActive()),
	}
	for g, members := range h.kwin.GroupMembers() {
		s.groups[g] = slices.Clone(members)
	}
	var walk func(ns []input.SemanticNode, depth int)
	walk = func(ns []input.SemanticNode, depth int) {
		for _, n := range ns {
			d := n.Desc
			s.nodes = append(s.nodes, semanticNode{
				depth:    depth,
				id:       d.Description,
				label:    d.Label,
				class:    d.Class,
				selected: d.Selected,
				disabled: d.Disabled,
				gestures: d.Gestures,
				bounds:   d.Bounds,
			})
			walk(n.Children, depth+1)
		}
	}
	walk(semanticRoot(&h.router), 0)
	return s
}

// maxDiffs bounds how many differences a stale-frame message lists.
const maxDiffs = 4

// diff describes how b differs from a, or returns "" when they match.
func (a semanticSnapshot) diff(b semanticSnapshot) string {
	var out []string
	if len(a.nodes) == len(b.nodes) {
		for i := range a.nodes {
			if d := a.nodes[i].diff(b.nodes[i]); d != "" {
				out = append(out, d)
			}
		}
	} else {
		out = append(out, diffNodeSets(a.nodes, b.nodes)...)
	}
	if a.focused != b.focused {
		out = append(out, fmt.Sprintf("focus %q → %q", a.focused, b.focused))
	}
	for _, g := range changedKeys(a.groups, b.groups, slices.Equal[[]string]) {
		out = append(out, fmt.Sprintf("members of group %q %q → %q", g, a.groups[g], b.groups[g]))
	}
	for _, g := range changedKeys(a.active, b.active, func(x, y string) bool { return x == y }) {
		out = append(out, fmt.Sprintf("active member of group %q %q → %q", g, a.active[g], b.active[g]))
	}
	if len(out) > maxDiffs {
		out = append(out[:maxDiffs], fmt.Sprintf("and %d more", len(out)-maxDiffs))
	}
	return strings.Join(out, "; ")
}

// diff describes the fields of the same node that changed from a to b.
func (a semanticNode) diff(b semanticNode) string {
	var fields []string
	if a.depth != b.depth {
		fields = append(fields, fmt.Sprintf("depth %d → %d", a.depth, b.depth))
	}
	if a.id != b.id {
		fields = append(fields, fmt.Sprintf("id %q → %q", a.id, b.id))
	}
	if a.label != b.label {
		fields = append(fields, fmt.Sprintf("label %q → %q", a.label, b.label))
	}
	if a.class != b.class {
		fields = append(fields, fmt.Sprintf("class %v → %v", a.class, b.class))
	}
	if a.selected != b.selected {
		fields = append(fields, fmt.Sprintf("selected %v → %v", a.selected, b.selected))
	}
	if a.disabled != b.disabled {
		fields = append(fields, fmt.Sprintf("disabled %v → %v", a.disabled, b.disabled))
	}
	if a.gestures != b.gestures {
		fields = append(fields, fmt.Sprintf("gestures %v → %v", a.gestures, b.gestures))
	}
	if a.bounds != b.bounds {
		fields = append(fields, fmt.Sprintf("bounds %v → %v", a.bounds, b.bounds))
	}
	if len(fields) == 0 {
		return ""
	}
	return a.name() + ": " + strings.Join(fields, ", ")
}

// name is how a diagnostic refers to the node: by id, else by label.
func (n semanticNode) name() string {
	switch {
	case n.id != "":
		return fmt.Sprintf("%q", n.id)
	case n.label != "":
		return fmt.Sprintf("label %q", n.label)
	}
	return fmt.Sprintf("unnamed %v node", n.class)
}

func (n semanticNode) String() string {
	return fmt.Sprintf("%s %v selected=%v disabled=%v gestures=%v at %v", n.name(), n.class, n.selected, n.disabled, n.gestures, n.bounds)
}

// diffNodeSets lists the nodes only in a as removed and those only in b as
// added, for trees whose shapes differ.
func diffNodeSets(a, b []semanticNode) []string {
	count := map[semanticNode]int{}
	for _, n := range a {
		count[n]++
	}
	for _, n := range b {
		count[n]--
	}
	var out []string
	for _, n := range a {
		if count[n] > 0 {
			count[n]--
			out = append(out, "removed "+n.String())
		}
	}
	for _, n := range b {
		if count[n] < 0 {
			count[n]++
			out = append(out, "added "+n.String())
		}
	}
	return out
}

// changedKeys returns, sorted, the keys whose values differ between a and b
// or that only one of them has.
func changedKeys[V any](a, b map[string]V, equal func(V, V) bool) []string {
	var out []string
	for k, va := range a {
		if vb, ok := b[k]; !ok || !equal(va, vb) {
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// diffRect bounds the pixels, alpha included, that differ between a and b.
func diffRect(a, b *image.RGBA) image.Rectangle {
	var r image.Rectangle
	for y := 0; y < a.Rect.Dy(); y++ {
		for x := 0; x < a.Rect.Dx(); x++ {
			i := a.PixOffset(x, y)
			if [4]uint8(a.Pix[i:i+4]) != [4]uint8(b.Pix[i:i+4]) {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}
