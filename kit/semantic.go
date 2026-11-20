package kit

import (
	"image"

	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
)

// Tag attaches a semantic description to the current clip area: the stable id
// tests find widgets by, and what screen readers announce. Use "kind:recordID".
func Tag(gtx layout.Context, id string) {
	if id != "" {
		semantic.DescriptionOp(Path(gtx, id)).Add(gtx.Ops)
	}
}

// TagArea lays out w on a clip area of its own, as large as w, and gives
// that area the id: for content that has no area to Tag (a label, a
// drawing).
func TagArea(gtx layout.Context, id string, w layout.Widget) layout.Dimensions {
	return area(gtx, w, func() { Tag(gtx, id) })
}

// area lays out w on a clip area of its own, as large as w, adding ops
// (semantics) to that area first.
func area(gtx layout.Context, w layout.Widget, ops func()) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	ops()
	call.Add(gtx.Ops)
	return dims
}

// TagLabel gives an addressed control a human-readable semantic label. The id
// remains stable for automation; the label can change with localization or
// state. Use this for icon-only controls and custom-drawn controls.
func TagLabel(gtx layout.Context, id, label string) {
	Tag(gtx, id)
	if label != "" {
		semantic.LabelOp(label).Add(gtx.Ops)
	}
}

// TagImage names an image area: id for tests, label as its alternative
// text, role RoleImage. Audits treat the label as a description, not
// painted text.
func TagImage(gtx layout.Context, id, label string) {
	TagLabel(gtx, id, label)
	RoleImage.Add(gtx.Ops)
}

// Role is what a control is, in the WAI-ARIA vocabulary: richer than
// upstream Gio's five semantic classes, so a tree node, a table row and a
// menu item are told apart. It is the fork's semantic.ClassOp: add it to the
// control's clip area (role.Add(gtx.Ops)) beside its Tag, and it is part of
// the semantic tree that kittest (h.Role, h.ByRole) and the platform read.
// The zero value, semantic.Unknown, is no role.
type Role = semantic.ClassOp

// The roles kit's components record.
const (
	RoleButton    = semantic.Button
	RoleLink      = semantic.Link
	RoleCheckbox  = semantic.CheckBox
	RoleSwitch    = semantic.Switch
	RoleRadio     = semantic.RadioButton
	RoleTextbox   = semantic.Editor
	RoleSlider    = semantic.Slider
	RoleCombobox  = semantic.ComboBox
	RoleListbox   = semantic.ListBox
	RoleOption    = semantic.Option
	RoleList      = semantic.List
	RoleListItem  = semantic.ListItem
	RoleTree      = semantic.Tree
	RoleTreeItem  = semantic.TreeItem
	RoleGrid      = semantic.Grid
	RoleRow       = semantic.Row
	RoleGridCell  = semantic.GridCell
	RoleTabList   = semantic.TabList
	RoleTab       = semantic.Tab
	RoleMenu      = semantic.Menu
	RoleMenuItem  = semantic.MenuItem
	RoleDialog    = semantic.Dialog
	RoleImage     = semantic.Image
	RoleToolbar   = semantic.Toolbar
	RoleStatus    = semantic.Status // a toast, a live message
	RoleTooltip   = semantic.Tooltip
	RoleGroup     = semantic.Group
	RoleSeparator = semantic.Separator
)
