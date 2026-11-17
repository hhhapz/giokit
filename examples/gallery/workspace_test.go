package main

import (
	"bytes"
	"image"
	"slices"
	"testing"

	"gioui.org/io/key"
	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

func TestWorkspaceInteractions(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "collections")
	h.ExpectID("work/icons")
	h.ExpectTextIn("work/icons", "Review icons")
	h.Hover("work/spec")
	h.ClickID("work/spec")
	if g.workSel != "spec" {
		t.Fatalf("selected %q", g.workSel)
	}
	h.Key(key.NameDownArrow, 0)
	if g.workSel != "icons" {
		t.Fatalf("arrow selected %q", g.workSel)
	}
	h.ClickID("folders/project/toggle")
	h.ExpectNoID("folders/design")
	h.ClickID("folders/project/toggle")
	h.ClickID("folders/design")
	if g.folderSel != "design" {
		t.Fatalf("folder selected %q", g.folderSel)
	}
	h.ClickID("folders/project")
	h.Key(key.NameLeftArrow, 0)
	h.ExpectNoID("folders/design")
	h.Key(key.NameRightArrow, 0)
	h.ExpectID("folders/design")
	h.ClickID("work:priority/plus")
	if g.priority.Value != 4 {
		t.Fatalf("priority %v", g.priority.Value)
	}
	for i := 0; i < 5; i++ {
		h.ClickID("work:priority/plus")
	}
	if g.priority.Value != 5 {
		t.Fatalf("maximum priority %v", g.priority.Value)
	}
	h.ClickID("work:priority")
	h.Key(key.NameDownArrow, 0)
	if g.priority.Value != 4 {
		t.Fatalf("keyboard priority %v", g.priority.Value)
	}
	h.Chord("Shortcut+A")
	h.Type("bad")
	if g.priority.Error == "" {
		t.Fatal("invalid number accepted")
	}
	h.ClickID("work:priority/plus")
	if g.priority.Error != "" {
		t.Fatalf("step did not clear error: %q", g.priority.Error)
	}
	h.ClickID("worktable/sort/1")
	if g.workTable.SortColumn != 1 || g.workTable.Descending {
		t.Fatal("table did not sort ascending")
	}
	h.ClickID("worktable/sort/1")
	if !g.workTable.Descending {
		t.Fatal("table did not reverse sort")
	}
	h.ClickID("worktable/icons")
	if g.workSel != "icons" {
		t.Fatalf("table row %q", g.workSel)
	}
	h.ClickID("work:secret")
	h.Type("hidden")
	if g.secret.Text() != "hidden" {
		t.Fatalf("password text %q", g.secret.Text())
	}
	for _, text := range h.Texts() {
		if text == "hidden" {
			t.Fatal("password rendered in clear text")
		}
	}
	g.workName.SetText("")
	h.Settle()
	h.ClickID("work:save")
	h.ExpectText("Required")
	g.workName.SetText("Review iconography")
	h.Settle()
	h.ClickID("work:save")
	h.ExpectText("Review iconography")
	if g.records[1].Name != "Review iconography" {
		t.Fatalf("saved name %q", g.records[1].Name)
	}
	h.ExpectClean()
	h.ExpectTabReachable()
}

// A keymap attached inside a KeyContext applies while focus is within it
// and wins over a window-wide one; with focus elsewhere the global one runs.
func TestScopedShortcuts(t *testing.T) {
	th := kit.Dark()
	var global, editor kit.Keymap
	var inside, outside kit.Clickable
	got := ""
	global.MustBind("Shortcut+J", "Global", func() { got = "global" })
	editor.MustBind("Shortcut+J", "Editor", func() { got = "editor" })
	h := kittest.NewT(t, 300, 100, func(gtx C) {
		global.Attach(gtx)
		inside.Clicked(gtx)
		outside.Clicked(gtx)
		kit.Row(gtx,
			kit.Rigid(func(gtx C) D {
				return kit.KeyContext(gtx, "editor", func(gtx C) D {
					editor.Attach(gtx)
					return ui.Button{ID: "inside", Text: "Inside"}.Layout(gtx, th, &inside)
				})
			}),
			kit.Rigid(func(gtx C) D { return ui.Button{ID: "outside", Text: "Outside"}.Layout(gtx, th, &outside) }),
		)
	})
	h.Tab(false) // keyboard focus on "inside"
	h.Chord("Shortcut+J")
	if got != "editor" {
		t.Fatalf("focus inside the context ran %q", got)
	}
	h.Tab(false) // "outside"
	h.Chord("Shortcut+J")
	if got != "global" {
		t.Fatalf("focus outside the context ran %q", got)
	}
}

func TestCommandPalette(t *testing.T) {
	g, h := newHarness(t)
	before := g.th.Name
	h.Chord("Shortcut+P")
	h.ExpectID("commands/query")
	h.Type("next theme")
	h.Key(key.NameReturn, 0)
	if g.th.Name == before {
		t.Fatal("command did not change theme")
	}
	h.ExpectNoID("commands")
	h.Theme = g.th
	h.ExpectClean()
}

// A list is one Tab stop: Tab lands on it once, arrows move the selection,
// and the next Tab leaves the list instead of walking its rows.
func TestSelectableListOneTabStop(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "collections")
	h.ClickID("work/spec")
	if got := h.FocusedID(); got != "work/spec" {
		t.Fatalf("click focused %q, want the list on its active row", got)
	}
	h.Key(key.NameEnd, 0)
	if g.workSel != "release" {
		t.Fatalf("End selected %q", g.workSel)
	}
	h.Key(key.NameHome, 0)
	if g.workSel != "spec" {
		t.Fatalf("Home selected %q", g.workSel)
	}
	h.Tab(false)
	if got := h.FocusedID(); got == "work" || got == "" || isRow(h, got) {
		t.Fatalf("Tab from the list went to %q, want the next control", got)
	}
	h.Tab(true)
	if got := h.FocusedID(); got != "work/spec" {
		t.Fatalf("Shift+Tab back landed on %q, want the list's active row", got)
	}
	if rows := h.GroupMembers()["work"]; len(rows) != 4 {
		t.Fatalf("group members %v", rows)
	}
	h.ExpectTabReachable()
	h.ExpectClean()
	// A click focuses quietly (no ring), even though the Tab walk above left
	// keyboard focus on the list; an arrow key shows the ring on the active
	// row. Same row selected both times: only the ring differs.
	h.ClickID("work/icons")
	r, _ := h.Find("work/icons")
	quiet := crop(h.Image(), r)
	h.Key(key.NameUpArrow, 0)
	h.Key(key.NameDownArrow, 0)
	if g.workSel != "icons" {
		t.Fatalf("selected %q", g.workSel)
	}
	if bytes.Equal(quiet, crop(h.Image(), r)) {
		t.Fatal("arrow keys after a click did not show the focus ring")
	}
}

func crop(img *image.RGBA, r image.Rectangle) []byte {
	var out []byte
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := img.PixOffset(r.Min.X, y)
		out = append(out, img.Pix[i:i+4*r.Dx()]...)
	}
	return out
}

func isRow(h *kittest.Harness, id string) bool {
	for _, m := range h.GroupMembers()["work"] {
		if m == id {
			return true
		}
	}
	return false
}

// Tree keys follow WAI-ARIA: Left on a leaf goes to its parent, Left on an
// open node closes it, Right opens it, Right again goes to the first child.
// Tree and table are one Tab stop each, toggles included.
func TestTreeAndTableKeyboard(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "collections")
	h.ClickID("folders/design")
	steps := []struct {
		key      key.Name
		selected string
		design   bool // "design" row visible
	}{
		{key.NameLeftArrow, "project", true},
		{key.NameLeftArrow, "project", false},
		{key.NameRightArrow, "project", true},
		{key.NameRightArrow, "design", true},
	}
	for i, s := range steps {
		h.Key(s.key, 0)
		_, shown := h.Find("folders/design")
		if g.folderSel != s.selected || shown != s.design {
			t.Fatalf("step %d (%s): selected %q, design shown %v", i, s.key, g.folderSel, shown)
		}
	}
	h.Tab(false)
	if got := h.FocusedID(); slices.Contains(h.GroupMembers()["folders"], got) || got == "folders" {
		t.Fatalf("Tab from the tree landed on %q", got)
	}

	h.ClickID("worktable/icons")
	h.Key(key.NameDownArrow, 0)
	if g.workSel != "tests" {
		t.Fatalf("table Down selected %q", g.workSel)
	}
	h.Key(key.NameHome, 0)
	if g.workSel != "spec" {
		t.Fatalf("table Home selected %q", g.workSel)
	}
	h.ExpectTabReachable()
	h.ExpectClean()
}
