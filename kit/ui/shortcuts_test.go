package ui_test

import (
	"slices"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

// Bindings sharing a title become one row with both chords.
func TestShortcutsFrom(t *testing.T) {
	var km kit.Keymap
	km.MustBind("Ctrl+K", "Search", func() {})
	km.MustBind("Ctrl+P", "Search", func() {})
	km.MustBind("Ctrl+S", "Save", func() {})
	got := ui.ShortcutsFrom(&km)
	want := []ui.Shortcut{{Title: "Search", Chords: []string{"Ctrl+K", "Ctrl+P"}}, {Title: "Save", Chords: []string{"Ctrl+S"}}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Title != want[i].Title || !slices.Equal(got[i].Chords, want[i].Chords) {
			t.Errorf("row %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestShortcutList(t *testing.T) {
	th := kit.Dark()
	list := ui.ShortcutList{Groups: []ui.ShortcutGroup{
		{Title: "Navigation", Items: []ui.Shortcut{{Title: "Next tab", Chords: []string{"Ctrl+Tab"}}}},
		{Items: []ui.Shortcut{{Title: "Search", Chords: []string{"Ctrl+K", "Ctrl+P"}}}},
	}}
	h := kittest.NewT(t, 400, 300, func(gtx layout.Context) {
		kit.TagArea(gtx, "list", func(gtx layout.Context) layout.Dimensions { return list.Layout(gtx, th) })
	})
	h.Theme = th
	want := []string{"NAVIGATION", "Next tab", "Ctrl", "Tab", "Search", "Ctrl", "K", "/", "Ctrl", "P"}
	got := h.TextsIn("list")
	if !slices.Equal(got, want) && !slices.Equal(got, append([]string{"Navigation"}, want[1:]...)) {
		t.Errorf("texts %q", got)
	}
	h.ExpectClean()
}
