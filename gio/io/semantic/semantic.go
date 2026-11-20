// SPDX-License-Identifier: Unlicense OR MIT

// Package semantic provides operations for semantic descriptions of a user
// interface, to facilitate presentation and interaction in external software
// such as screen readers.
//
// Semantic descriptions are organized in a tree, with clip operations as
// nodes. Operations in this package are associated with the current semantic
// node, that is the most recent pushed clip operation.
package semantic

import (
	"github.com/hhhapz/giokit/gio/internal/ops"
	"github.com/hhhapz/giokit/gio/op"
)

// LabelOp provides the content of a textual component.
type LabelOp string

// DescriptionOp describes a component.
type DescriptionOp string

// ClassOp provides the component class.
type ClassOp int

const (
	Unknown ClassOp = iota
	Button
	CheckBox
	Editor
	RadioButton
	Switch
	Image
	// giokit: WAI-ARIA roles beyond upstream's classes.
	Link      // navigates somewhere
	Slider    // picks a value in a range
	ComboBox  // a field that opens a list of choices
	ListBox   // a list of choices
	Option    // a choice in a ListBox
	List      // a list of items
	ListItem  // an item in a List
	Tree      // a hierarchical list
	TreeItem  // an item in a Tree
	Grid      // a table of interactive cells
	Row       // a row in a Grid
	GridCell  // a cell in a Row
	TabList   // a set of tabs
	Tab       // a tab in a TabList
	Menu      // a list of commands
	MenuItem  // a command in a Menu
	Dialog    // a window over the page
	Toolbar   // a set of controls
	Status    // a live message (a toast)
	Tooltip   // a description shown on hover or focus
	Group     // a named set of components
	Separator // divides content
)

// SelectedOp describes the selected state for components that have
// boolean state.
type SelectedOp bool

// EnabledOp describes the enabled state.
type EnabledOp bool

func (l LabelOp) Add(o *op.Ops) {
	data := ops.Write1String(&o.Internal, ops.TypeSemanticLabelLen, string(l))
	data[0] = byte(ops.TypeSemanticLabel)
}

func (d DescriptionOp) Add(o *op.Ops) {
	data := ops.Write1String(&o.Internal, ops.TypeSemanticDescLen, string(d))
	data[0] = byte(ops.TypeSemanticDesc)
}

func (c ClassOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeSemanticClassLen)
	data[0] = byte(ops.TypeSemanticClass)
	data[1] = byte(c)
}

func (s SelectedOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeSemanticSelectedLen)
	data[0] = byte(ops.TypeSemanticSelected)
	if s {
		data[1] = 1
	}
}

func (e EnabledOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeSemanticEnabledLen)
	data[0] = byte(ops.TypeSemanticEnabled)
	if e {
		data[1] = 1
	}
}

func (c ClassOp) String() string {
	switch c {
	case Unknown:
		return "Unknown"
	case Button:
		return "Button"
	case CheckBox:
		return "CheckBox"
	case Editor:
		return "Editor"
	case RadioButton:
		return "RadioButton"
	case Switch:
		return "Switch"
	case Image:
		return "Image"
	case Link:
		return "Link"
	case Slider:
		return "Slider"
	case ComboBox:
		return "ComboBox"
	case ListBox:
		return "ListBox"
	case Option:
		return "Option"
	case List:
		return "List"
	case ListItem:
		return "ListItem"
	case Tree:
		return "Tree"
	case TreeItem:
		return "TreeItem"
	case Grid:
		return "Grid"
	case Row:
		return "Row"
	case GridCell:
		return "GridCell"
	case TabList:
		return "TabList"
	case Tab:
		return "Tab"
	case Menu:
		return "Menu"
	case MenuItem:
		return "MenuItem"
	case Dialog:
		return "Dialog"
	case Toolbar:
		return "Toolbar"
	case Status:
		return "Status"
	case Tooltip:
		return "Tooltip"
	case Group:
		return "Group"
	case Separator:
		return "Separator"
	default:
		panic("invalid ClassOp")
	}
}
