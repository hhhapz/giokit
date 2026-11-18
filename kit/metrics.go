package kit

import "gioui.org/unit"

// Metrics are the theme's fixed sizes: everything that is neither spacing
// (the SpaceN scale, through Theme.Space) nor a corner (Theme.Radius).
// Components read them from the theme instead of writing numbers.
type Metrics struct {
	IconSize       unit.Dp // icons in controls, rows and fields
	IconSizeLg     unit.Dp // icons in alerts and toasts
	CheckSize      unit.Dp // checkbox box
	IconButtonSize unit.Dp // square icon-only button
	SwitchWidth    unit.Dp // switch track width
	SwitchHeight   unit.Dp // switch track height, and a slider's height
	HitSize        unit.Dp // smallest pointer target for thin things (split handles, scrollbars)
	PaneMinSize    unit.Dp // smallest split pane

	MenuMinWidth    unit.Dp // menus are at least this wide
	ListMaxHeight   unit.Dp // a select's list scrolls past this
	PopoverMaxWidth unit.Dp // popover panels are at most this wide
	DialogMaxWidth  unit.Dp // modal dialogs are at most this wide
	ToastWidth      unit.Dp // a toast's width, narrowed in small windows
	ReadingWidth    unit.Dp // longest comfortable text block (empty states)
	LabelWidth      unit.Dp // form label column
	NarrowWidth     unit.Dp // below this, forms stack labels above fields
}

// DefaultMetrics returns giokit's sizes at density 1. Each call returns a
// fresh value.
func DefaultMetrics() Metrics {
	return Metrics{
		IconSize: 14, IconSizeLg: 16, CheckSize: 18, IconButtonSize: 22,
		SwitchWidth: 34, SwitchHeight: 20, HitSize: 16, PaneMinSize: 80,
		MenuMinWidth: 180, ListMaxHeight: 300, PopoverMaxWidth: 420, DialogMaxWidth: 480,
		ToastWidth: 340, ReadingWidth: 360, LabelWidth: 132, NarrowWidth: 440,
	}
}

// metricToken names one Metrics field for DTCG files (group "size").
type metricToken struct {
	name  string
	value *unit.Dp
}

// tokens names each metric for DTCG files.
func (m *Metrics) tokens() []metricToken {
	return []metricToken{
		{name: "icon", value: &m.IconSize},
		{name: "icon-lg", value: &m.IconSizeLg},
		{name: "check", value: &m.CheckSize},
		{name: "icon-button", value: &m.IconButtonSize},
		{name: "switch-width", value: &m.SwitchWidth},
		{name: "switch-height", value: &m.SwitchHeight},
		{name: "hit", value: &m.HitSize},
		{name: "pane-min", value: &m.PaneMinSize},
		{name: "menu-min-width", value: &m.MenuMinWidth},
		{name: "list-max-height", value: &m.ListMaxHeight},
		{name: "popover-max-width", value: &m.PopoverMaxWidth},
		{name: "dialog-max-width", value: &m.DialogMaxWidth},
		{name: "toast-width", value: &m.ToastWidth},
		{name: "reading-width", value: &m.ReadingWidth},
		{name: "label-width", value: &m.LabelWidth},
		{name: "narrow-width", value: &m.NarrowWidth},
	}
}
