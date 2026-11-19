package kit

import (
	"slices"

	"github.com/hhhapz/giokit/gio/layout"
)

// AnchoredList keeps the first visible item, by stable id, at the same
// screen offset when items are inserted, removed, or reordered. It suits
// chat history and live feeds, where index-based scroll positions would
// jump. The zero value is ready to use. Its List identifies input areas by
// address, so an AnchoredList must not be copied after first use.
type AnchoredList struct {
	// List is the scroll state; read or set its Position as for any List.
	List   List
	anchor string // id of the first visible item last frame; "" for none
	index  int    // its index last frame
}

// AnchoredList lays out a.List with one item per id: item(gtx, i) draws
// ids[i]. ids is read only during the call and may be changed freely
// between frames. If the item first visible last frame has moved to another
// index, the list scrolls with it, keeping its offset. Checking costs O(1)
// per frame while that item stays put; finding it after it moves is O(n).
func (t *Theme) AnchoredList(gtx layout.Context, a *AnchoredList, id string, ids []string, item layout.ListElement) layout.Dimensions {
	if a.anchor != "" && (a.index >= len(ids) || ids[a.index] != a.anchor) {
		if next := slices.Index(ids, a.anchor); next >= 0 {
			a.List.Position.First = next
			a.List.Position.BeforeEnd = true
		}
	}
	d := t.List(gtx, &a.List, id, len(ids), item)
	a.anchor, a.index = "", 0
	if i := a.List.Position.First; i >= 0 && i < len(ids) {
		a.anchor, a.index = ids[i], i
	}
	return d
}
