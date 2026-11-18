// Package ui holds giokit's components, built only from package kit's
// exported API: anything here could be written outside giokit.
//
// A component is a props struct laid out with a theme and its state:
//
//	ui.Combobox{ID: "city", Options: cities}.Layout(gtx, th, &v.city)
//
// Props are values, set fresh each frame. State structs hold what must
// survive between frames; they contain event tags, so a state must not be
// copied or moved after its first use, and only the UI goroutine touches
// it. The zero value of every state is ready to use.
//
// # Values
//
// Controlled values are the default: the value comes in through props
// (Checkbox.Checked, Select.Selected, Tabs.Selected, DateField.Value,
// Slider.Value, Accordion.Open, SelectableList.Selected), and the caller
// stores what the state's Update reports and passes it back next frame.
// The component never changes it behind the caller's back.
//
// A few values are retained by the state instead, because they are
// interaction state that changes continuously or locally, and each is
// documented on its state type:
//
//   - SplitState.Ratio: the divider position while dragging;
//   - NumberFieldState's value and the text being edited (SetValue, Value);
//   - TreeState's expanded nodes (SetExpanded);
//   - DataTableState's sort column and direction;
//   - the editor text of TextField, TextArea and Combobox, which lives in
//     the widget.Editor the caller owns;
//   - x.LogView's filters.
//
// # Input and events
//
// Events are read in the handle pass, before layout, with the state's
// Update (or Clickable.Clicked for single-button components): kit's frame
// rule is to handle input, then draw. Update consumes the input it reports
// and returns at most one event per call. Components whose events are
// picks (Menu, Select, Toolbar, Breadcrumbs, Tabs, Accordion) report the
// first and leave the rest for the next call; value components (Slider,
// NumberField) apply every input of the frame in order and report the
// final value once.
//
// Keys handled by kit's action dispatch (arrows, Home/End, Enter in focus
// groups) run after layout, when the frame's focus is known. A component
// that picks from such a key queues the pick and reports it from the next
// Update, asking for that frame with kit.Notify. Several keys in one frame
// each move on from the previous one.
//
// Some components handle their input inside Layout, because it depends on
// their props or is not reported to the caller:
//
//   - editors with suggestions: Combobox (its suggestions depend on
//     Options) and CommandPalette (its results depend on Actions, and it
//     runs the picked action itself);
//   - self-contained overlays: Tooltip (hover timing), ToastHost (expiry
//     and dismissal) and Split (its retained Ratio).
//
// Navigation in text-field popups (Combobox, CommandPalette) wraps around;
// focus-group lists and menus clamp at the ends. Enter means Return or the
// keypad's Enter everywhere.
package ui

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

// describe records a control's semantics: its Gio class, its kit role
// under id, and its spoken label (none when empty). Call it inside the
// control's clip area, as th.Press's callback is.
func describe(gtx layout.Context, id string, role kit.Role, class semantic.ClassOp, label string) {
	class.Add(gtx.Ops)
	kit.SetRole(gtx, id, role)
	if label != "" {
		semantic.LabelOp(label).Add(gtx.Ops)
	}
}
