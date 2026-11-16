package kit

import "slices"

// AnchoredList keeps the first visible stable ID at the same screen offset
// when items are inserted, removed, or reordered. It is useful for chat
// history and live feeds where index-based scroll positions would jump.
type AnchoredList struct {
	List   List
	ids    []string
	anchor string
	seen   bool
}

func (t *Theme) AnchoredList(gtx C, a *AnchoredList, id string, ids []string, item func(C, int) D) D {
	if a.seen && !slices.Equal(a.ids, ids) && a.anchor != "" {
		if next := slices.Index(ids, a.anchor); next >= 0 {
			a.List.Position.First = next
			a.List.Position.BeforeEnd = true
		}
	}
	d := t.List(gtx, &a.List, id, len(ids), item)
	a.ids = append(a.ids[:0], ids...)
	a.seen = true
	if i := a.List.Position.First; i >= 0 && i < len(ids) {
		a.anchor = ids[i]
	} else {
		a.anchor = ""
	}
	return d
}
