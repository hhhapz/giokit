package kittest

import (
	"image"
	"iter"

	"github.com/hhhapz/giokit/gio/io/input"
)

// semanticRoot returns the root of the router's semantic tree as a
// one-element slice, ready to walk through Children. Router.AppendSemantics
// returns every node of the tree exactly once, root first; the nodes after
// the root are the same nodes its Children slices reach, so only the root
// starts a walk. TestSemanticTreeShape checks this on the Gio in use.
func semanticRoot(r *input.Router) []input.SemanticNode {
	all := r.AppendSemantics(nil)
	if len(all) == 0 {
		return nil
	}
	return all[:1]
}

// preorder yields every node under roots, each parent before its children.
func preorder(roots []input.SemanticNode) iter.Seq[input.SemanticNode] {
	return func(yield func(input.SemanticNode) bool) {
		var walk func([]input.SemanticNode) bool
		walk = func(ns []input.SemanticNode) bool {
			for _, n := range ns {
				if !yield(n) || !walk(n.Children) {
					return false
				}
			}
			return true
		}
		walk(roots)
	}
}

// semanticTree is one frame's semantic nodes in pre-order (draw order),
// indexed by id for walks towards the root.
type semanticTree struct {
	nodes []input.SemanticNode
	byID  map[input.SemanticID]input.SemanticNode
}

// semantics snapshots the semantic tree of the last frame.
func (h *Harness) semantics() semanticTree {
	t := semanticTree{byID: map[input.SemanticID]input.SemanticNode{}}
	for n := range preorder(semanticRoot(&h.router)) {
		t.nodes = append(t.nodes, n)
		t.byID[n.ID] = n
	}
	return t
}

// ancestors yields n's ancestors, nearest first, ending with the root.
func (t semanticTree) ancestors(n input.SemanticNode) iter.Seq[input.SemanticNode] {
	return func(yield func(input.SemanticNode) bool) {
		for n.ParentID != 0 {
			p, ok := t.byID[n.ParentID]
			if !ok || !yield(p) {
				return
			}
			n = p
		}
	}
}

// within reports whether node id is a strict descendant of ancestor.
func (t semanticTree) within(id, ancestor input.SemanticID) bool {
	n, ok := t.byID[id]
	if !ok {
		return false
	}
	for p := range t.ancestors(n) {
		if p.ID == ancestor {
			return true
		}
	}
	return false
}

// disabled reports whether n or an ancestor is disabled (kit's
// Theme.Disable marks the subtree's root).
func (t semanticTree) disabled(n input.SemanticNode) bool {
	if n.Desc.Disabled {
		return true
	}
	for p := range t.ancestors(n) {
		if p.Desc.Disabled {
			return true
		}
	}
	return false
}

// visible clips n's bounds to its ancestors' (a scrolled list's rows report
// bounds outside its viewport) and to the screen. Ancestors with empty
// bounds (pure grouping nodes) do not clip.
func (t semanticTree) visible(n input.SemanticNode, screen image.Rectangle) image.Rectangle {
	r := n.Desc.Bounds.Intersect(screen)
	for p := range t.ancestors(n) {
		if b := p.Desc.Bounds; !b.Empty() {
			r = r.Intersect(b)
		}
	}
	return r
}
