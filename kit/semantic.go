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

// TagArea lays out w and gives its whole area the id, on a nested clip area
// of its own that carries no event handler.
//
// It exists because of how Gio's router keeps semantics: an area with an
// event handler keeps its description only if that handler asked for
// pointer gestures this frame, that is, if its tag was polled with a
// pointer filter. A handler that is never polled that way (a FocusGroup
// container that reads only key and focus events, a keyboard-only canvas,
// a disabled control, a component whose Update was skipped this frame)
// asks for none, and Gio drops the area's description, so Tag on it would
// be lost. A handler-free area is always kept.
func TagArea(gtx layout.Context, id string, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	Tag(gtx, id)
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
// text. Audits treat the label as a description, not painted text (stock
// Gio has no image role).
func TagImage(gtx layout.Context, id, label string) {
	TagLabel(gtx, id, label)
	SetRole(gtx, id, RoleImage)
}

// Role is what a control is, in the WAI-ARIA vocabulary: richer than Gio's
// five semantic classes, so a tree node, a table row and a menu item are
// told apart. kit's components set their roles and kittest reads them
// (h.Role, h.ByRole). Roles live in the window's frame (Window.Roles):
// the platform's accessibility tree sees only Gio's semantic classes.
type Role string

// The roles kit's components record.
const (
	RoleButton    Role = "button"
	RoleLink      Role = "link"
	RoleCheckbox  Role = "checkbox"
	RoleSwitch    Role = "switch"
	RoleRadio     Role = "radio"
	RoleTextbox   Role = "textbox"
	RoleSlider    Role = "slider"
	RoleCombobox  Role = "combobox"
	RoleListbox   Role = "listbox"
	RoleOption    Role = "option"
	RoleList      Role = "list"
	RoleListItem  Role = "listitem"
	RoleTree      Role = "tree"
	RoleTreeItem  Role = "treeitem"
	RoleGrid      Role = "grid"
	RoleRow       Role = "row"
	RoleGridCell  Role = "gridcell"
	RoleTabList   Role = "tablist"
	RoleTab       Role = "tab"
	RoleMenu      Role = "menu"
	RoleMenuItem  Role = "menuitem"
	RoleDialog    Role = "dialog"
	RoleImage     Role = "img"
	RoleToolbar   Role = "toolbar"
	RoleStatus    Role = "status" // a toast, a live message
	RoleTooltip   Role = "tooltip"
	RoleGroup     Role = "group"
	RoleSeparator Role = "separator"
)

// SetRole records id's role this frame (see Role).
func SetRole(gtx layout.Context, id string, r Role) {
	if f := frameOf(gtx); f != nil {
		if f.roles == nil {
			f.roles = map[string]Role{}
		}
		f.roles[Path(gtx, id)] = r
	}
}

// Roles are the roles recorded in the last frame, by id path. The map is
// borrowed: it belongs to the window and is cleared and refilled by the
// next frame's Begin, so copy what you keep and do not modify it.
func (w *Window) Roles() map[string]Role { return w.fr.roles }

// Images are the id paths of image areas in the last frame: every id whose
// role is RoleImage (TagImage). It returns a new map on each call.
func (w *Window) Images() map[string]bool {
	images := map[string]bool{}
	for id, r := range w.fr.roles {
		if r == RoleImage {
			images[id] = true
		}
	}
	return images
}
