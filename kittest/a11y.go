package kittest

import (
	"image"
	"slices"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
)

// A11yNode is a stable, inspectable snapshot of one semantic node.
type A11yNode struct {
	ID, Label          string
	Class              semantic.ClassOp
	Bounds             image.Rectangle
	Disabled, Selected bool
	Children           []A11yNode
}

// A11yTree returns the semantic tree of the current frame. IDs are the stable
// automation addresses; labels are the text presented to users.
func (h *Harness) A11yTree() []A11yNode {
	var copyNodes func([]input.SemanticNode) []A11yNode
	copyNodes = func(nodes []input.SemanticNode) []A11yNode {
		out := make([]A11yNode, 0, len(nodes))
		for _, n := range nodes {
			d := n.Desc
			out = append(out, A11yNode{ID: d.Description, Label: d.Label, Class: d.Class,
				Bounds: d.Bounds, Disabled: d.Disabled, Selected: d.Selected, Children: copyNodes(n.Children)})
		}
		return out
	}
	all := h.router.AppendSemantics(nil)
	if len(all) == 0 {
		return nil
	}
	return copyNodes(all[:1])
}

// TabUnreachable visits the enabled, named controls in the current frame and
// returns those Tab cannot reach. Call it on a stable screen with no popup
// opening during traversal. It leaves focus at the final Tab stop.
func (h *Harness) TabUnreachable() []string {
	var want []string
	nodes := h.nodes()
	byID := make(map[input.SemanticID]input.SemanticNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for _, n := range nodes {
		d := n.Desc
		if d.Description == "" || disabled(byID, n) {
			continue
		}
		switch d.Class {
		case semantic.Button, semantic.CheckBox, semantic.RadioButton, semantic.Switch, semantic.Editor:
			want = append(want, d.Description)
		}
	}
	seen := map[string]bool{}
	if id := h.FocusedID(); id != "" {
		seen[id] = true
	}
	// A screen may contain focusable regions that have no semantic node at all.
	// Leave room for those stops and ignore repeated empty IDs while traversing.
	for i := 0; i < max(64, len(nodes)*16); i++ {
		h.Tab(false)
		id := h.FocusedID()
		if id == "" {
			continue
		}
		if seen[id] {
			break
		}
		seen[id] = true
	}
	// A focus group (list, tree, toolbar) is one Tab stop; its members are
	// reached with arrow keys once Tab lands on the group.
	for group, members := range h.kwin.GroupMembers() {
		if seen[group] || slices.ContainsFunc(members, func(m string) bool { return seen[m] }) {
			for _, m := range members {
				seen[m] = true
			}
		}
	}
	return slices.DeleteFunc(want, func(id string) bool { return seen[id] })
}

// GroupMembers maps each focus group laid out in the last frame to the ids
// of its members. A group is one Tab stop; arrows move inside it.
func (h *Harness) GroupMembers() map[string][]string { return h.kwin.GroupMembers() }

// ExpectTabReachable reports every enabled control that Tab skipped.
func (h *Harness) ExpectTabReachable() {
	h.t()
	for _, id := range h.TabUnreachable() {
		h.tb.Errorf("Tab cannot reach %q", id)
	}
}
