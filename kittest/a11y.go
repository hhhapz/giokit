package kittest

import (
	"image"
	"slices"

	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/semantic"

	"github.com/hhhapz/giokit/kit"
)

// Tab traversal budget: ExpectTabReachable presses Tab at most
// tabStopsPerNode times per semantic node, and at least minTabPresses
// times, before giving up on reaching the first stop again. The slack
// covers focusable areas without a node of their own and unnamed stops.
const (
	minTabPresses   = 64
	tabStopsPerNode = 16
)

// tabBudget is how many Tab presses a screen of nodes semantic nodes gets.
func tabBudget(nodes int) int { return max(minTabPresses, nodes*tabStopsPerNode) }

// A11yNode is a stable, inspectable snapshot of one semantic node.
type A11yNode struct {
	ID, Label          string           // semantic id (description) and spoken label
	Class              semantic.ClassOp // Gio's semantic class
	Bounds             image.Rectangle  // window bounds in pixels
	Disabled, Selected bool             // Gio's enabled and selected state
	Children           []A11yNode       // nested nodes, in draw order
}

// A11yTree returns the semantic tree of the current frame. IDs are the stable
// automation addresses; labels are the text presented to users.
func (h *Harness) A11yTree() []A11yNode {
	h.tb.Helper()
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
	root := semanticRoot(&h.router)
	if root == nil {
		return nil
	}
	return copyNodes(root)
}

// focusableClasses are the Gio semantic classes of controls Tab must reach.
var focusableClasses = []semantic.ClassOp{semantic.Button, semantic.CheckBox, semantic.RadioButton, semantic.Switch, semantic.Editor}

// focusableRoles are the kit roles of controls Tab must reach, covering
// custom controls (a slider, a split handle) that have no Gio class. Members
// of a focus group count as reached when the group is.
var focusableRoles = []kit.Role{
	kit.RoleButton, kit.RoleLink, kit.RoleCheckbox, kit.RoleSwitch, kit.RoleRadio,
	kit.RoleTextbox, kit.RoleSlider, kit.RoleCombobox, kit.RoleTab,
}

// TabUnreachable visits the enabled, named controls in the current frame and
// returns those Tab cannot reach. A control is one by its Gio semantic class
// or its kit role. Call it on a stable screen with no popup opening during
// traversal. It leaves focus at the final Tab stop.
func (h *Harness) TabUnreachable() []string {
	h.tb.Helper()
	var want []string
	t := h.semantics()
	roles := h.kwin.Roles()
	for _, n := range t.nodes {
		d := n.Desc
		if d.Description == "" || t.disabled(n) {
			continue
		}
		if slices.Contains(focusableClasses, d.Class) || slices.Contains(focusableRoles, roles[d.Description]) {
			want = append(want, d.Description)
		}
	}
	seen := map[string]bool{}
	if id := h.FocusedID(); id != "" {
		seen[id] = true
	}
	// A screen may contain focusable regions that have no semantic node at all.
	// Leave room for those stops and ignore repeated empty IDs while traversing.
	for i := 0; i < tabBudget(len(t.nodes)); i++ {
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
// of its members. A group is one Tab stop; arrows move inside it. The map
// is borrowed from the harness: valid until the next frame, not to be
// modified.
func (h *Harness) GroupMembers() map[string][]string { return h.kwin.GroupMembers() }

// ExpectTabReachable reports every enabled control that Tab skipped.
func (h *Harness) ExpectTabReachable() {
	h.tb.Helper()
	h.requireTest("ExpectTabReachable")
	for _, id := range h.TabUnreachable() {
		h.tb.Errorf("Tab cannot reach %q", id)
	}
}
