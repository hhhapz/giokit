package ui_test

import (
	"testing"

	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// The palette's selection is clamped when its results shrink: Enter after
// the enabled actions dropped below the selection indexed past the end.
func TestCommandPaletteResultsShrink(t *testing.T) {
	th := kit.Dark()
	var acts kit.Actions
	enabled, ran := true, ""
	for _, id := range []string{"a", "b", "c"} {
		acts.MustRegister(kit.Action{ID: id, Title: "Action " + id, Run: func() { ran = id },
			Enabled: func() bool { return id == "a" || enabled }})
	}
	var p ui.CommandPaletteState
	h := kittest.NewT(t, 400, 400, func(gtx layout.Context) {
		ui.CommandPalette{ID: "cmd", Actions: &acts, Page: kit.Grow}.Layout(gtx, th, &p)
	})
	p.Open()
	h.Settle()
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameDownArrow, 0)
	enabled = false
	h.Settle()
	h.Key(key.NameReturn, 0)
	if ran != "a" {
		t.Fatalf("Enter ran %q, want the one remaining action", ran)
	}
}

// Shift+F10 opens a context menu from the keyboard while focus is inside
// its content; the menu takes focus, and a pick is reported.
func TestContextMenuKeyboard(t *testing.T) {
	th := kit.Dark()
	var m ui.MenuState
	var btn kit.Clickable
	picked := ""
	h := kittest.NewT(t, 400, 300, func(gtx layout.Context) {
		if id, ok := m.Update(gtx); ok {
			picked = id
		}
		ui.ContextMenu{ID: "ctx", Items: []ui.MenuItem{{ID: "copy", Label: "Copy"}, {ID: "paste", Label: "Paste"}}, Content: func(gtx layout.Context) layout.Dimensions {
			return ui.Button{ID: "inside", Text: "Inside"}.Layout(gtx, th, &btn)
		}}.Layout(gtx, th, &m)
	})
	h.Tab(false)
	h.Key(key.NameF10, key.ModShift)
	if !m.Opened() {
		t.Fatal("Shift+F10 did not open the menu")
	}
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameReturn, 0)
	if picked != "copy" {
		t.Fatalf("picked %q", picked)
	}
}

// Enter on the keypad runs the selected command, as Return does.
func TestCommandPaletteKeypadEnter(t *testing.T) {
	th := kit.Dark()
	var acts kit.Actions
	ran := ""
	acts.MustRegister(kit.Action{ID: "a", Title: "Action", Run: func() { ran = "a" }})
	var p ui.CommandPaletteState
	h := kittest.NewT(t, 400, 400, func(gtx layout.Context) {
		ui.CommandPalette{ID: "cmd", Actions: &acts, Page: kit.Grow}.Layout(gtx, th, &p)
	})
	p.Open()
	h.Settle()
	h.Key(key.NameEnter, 0)
	if ran != "a" {
		t.Fatalf("keypad Enter ran %q", ran)
	}
}

// Two Downs in one frame move two menu items: each key moves on from the
// item the previous one reached.
func TestMenuKeyBatch(t *testing.T) {
	th := kit.Dark()
	var m ui.MenuState
	var btn kit.Clickable
	items := []ui.MenuItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}, {ID: "d", Label: "Delta"}}
	h := kittest.NewT(t, 400, 400, func(gtx layout.Context) {
		m.Update(gtx)
		ui.Menu{ID: "menu", Items: items, Trigger: func(gtx layout.Context) layout.Dimensions {
			return ui.Button{ID: "open", Text: "Open"}.Layout(gtx, th, &btn)
		}}.Layout(gtx, th, &m)
	})
	m.Show()
	h.Settle()
	h.Key(key.NameDownArrow, 0)
	if f := h.FocusedID(); f != "menu/a" {
		t.Fatalf("first Down: focus %q, want menu/a", f)
	}
	h.Keys(key.NameDownArrow, key.NameDownArrow)
	if f := h.FocusedID(); f != "menu/c" {
		t.Fatalf("two Downs in one frame: focus %q, want menu/c", f)
	}
	h.ExpectClean()
}

// Two Downs in one frame move two options in an open Select.
func TestSelectKeyBatch(t *testing.T) {
	th := kit.Dark()
	var s ui.SelectState
	sel := "a"
	opts := []ui.Choice{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}, {ID: "d", Label: "Delta"}}
	h := kittest.NewT(t, 400, 400, func(gtx layout.Context) {
		if id, ok := s.Update(gtx); ok {
			sel = id
		}
		ui.Select{ID: "sel", Options: opts, Selected: sel}.Layout(gtx, th, &s)
	})
	h.ClickID("sel/field")
	if f := h.FocusedID(); f != "sel/a" {
		t.Fatalf("open: focus %q, want the selected option sel/a", f)
	}
	h.Keys(key.NameDownArrow, key.NameDownArrow)
	if f := h.FocusedID(); f != "sel/c" {
		t.Fatalf("two Downs in one frame: focus %q, want sel/c", f)
	}
	h.Key(key.NameReturn, 0)
	if sel != "c" {
		t.Fatalf("Enter picked %q, want c", sel)
	}
	h.ExpectClean()
}
