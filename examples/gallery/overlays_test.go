package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/io/key"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kittest"
)

func overlaysPage(t *testing.T, id string) (*gallery, *kittest.Harness) {
	t.Helper()
	g, h := newHarness(t)
	goTo(t, h, g, id)
	return g, h
}

// Menu by mouse: opens below its trigger, picks, closes; disabled items do
// nothing; the audit stays clean while it's open (the backdrop is intended).
func TestMenuMouse(t *testing.T) {
	g, h := overlaysPage(t, "menus")
	h.ClickID("file:btn")
	trig, item := h.ExpectID("file:btn"), h.ExpectID("file/new")
	if item.Min.Y < trig.Max.Y {
		t.Errorf("menu not below its trigger: item %v, trigger %v", item, trig)
	}
	h.ExpectClean()
	if !h.Disabled("file/save") {
		t.Error("Save should be disabled")
	}
	h.ClickID("file/save")
	if !g.overlays.fileMenu.Opened() {
		t.Fatal("clicking a disabled item closed the menu")
	}
	h.Hover("file/new")
	h.ClickID("file/new")
	if g.overlays.fileMenu.Opened() || g.status != "File → new" {
		t.Fatalf("pick: open %v, status %q", g.overlays.fileMenu.Opened(), g.status)
	}
	h.ExpectNoID("file/new")
	if f := h.FocusedID(); f != "file:btn" {
		t.Errorf("focus after a pick: %q, want the trigger", f)
	}
	// A press outside closes it, and goes nowhere else.
	h.ClickID("file:btn")
	h.ClickID("more:btn")
	if g.overlays.fileMenu.Opened() || g.overlays.moreMenu.Opened() {
		t.Fatalf("outside press: file menu open %v, other menu open %v (the press leaked through)", g.overlays.fileMenu.Opened(), g.overlays.moreMenu.Opened())
	}
}

// Menu by keyboard: Enter opens, Down enters, arrows move, Enter picks;
// Escape closes and returns focus to the trigger; Tab closes too.
func TestMenuKeyboard(t *testing.T) {
	g, h := overlaysPage(t, "menus")
	h.ClickID("file:btn")
	h.Key(key.NameEscape, 0)
	if g.overlays.fileMenu.Opened() || h.FocusedID() != "file:btn" {
		t.Fatalf("Escape: open %v, focus %q", g.overlays.fileMenu.Opened(), h.FocusedID())
	}
	h.Key(key.NameReturn, 0)
	if !g.overlays.fileMenu.Opened() {
		t.Fatal("Enter on the trigger did not open the menu")
	}
	h.Key(key.NameDownArrow, 0)
	if f := h.FocusedID(); f != "file/new" {
		t.Fatalf("Down from the panel: focus %q, want file:new", f)
	}
	h.Key(key.NameDownArrow, 0) // skips the disabled Save
	h.Key(key.NameDownArrow, 0)
	if f := h.FocusedID(); f != "file/delete" {
		t.Fatalf("Down ×3: focus %q, want file:delete (Save is disabled)", f)
	}
	h.Key(key.NameReturn, 0)
	if !g.overlays.dialog.Opened() {
		t.Fatal("Enter on Delete… did not open the dialog")
	}
	h.Key(key.NameEscape, 0) // closes the dialog
	if g.overlays.dialog.Opened() {
		t.Fatal("Escape did not close the dialog")
	}
	h.ClickID("file:btn")
	h.Key(key.NameDownArrow, 0)
	h.Tab(false)
	if g.overlays.fileMenu.Opened() || h.FocusedID() != "file:btn" {
		t.Fatalf("Tab in the menu: open %v, focus %q", g.overlays.fileMenu.Opened(), h.FocusedID())
	}
}

// A menu at the right edge shifts left to stay inside the window.
func TestMenuShiftsAtEdge(t *testing.T) {
	_, h := overlaysPage(t, "menus")
	h.ClickID("more:btn")
	if r := h.ExpectID("more"); r.Max.X > h.Size.X || r.Min.X < 0 {
		t.Fatalf("menu leaves the window: %v (window %v)", r, h.Size)
	}
	h.ExpectClean()
}

// A popup without room below flips above its anchor.
func TestPopupFlipsAtBottom(t *testing.T) {
	g := newGallery()
	h := kittest.NewT(t, 1100, 400, g.Frame)
	h.Theme = g.th
	goTo(t, h, g, "pickers")
	f := h.ExpectID("zone/field")
	h.ClickID("zone/field")
	p := h.ExpectID("zone")
	if room := h.Size.Y - f.Max.Y; room < p.Dy() && p.Min.Y > f.Min.Y {
		t.Errorf("no room below (%dpx for %dpx), but the panel didn't flip: %v, field %v", room, p.Dy(), p, f)
	}
	if p.Max.Y > h.Size.Y || p.Min.Y < 0 {
		t.Errorf("panel outside the window: %v", p)
	}
	h.ExpectClean()
}

// A popup opened from the keyboard, before the pointer ever crossed its
// anchor, flips too: kit places it from the anchor's laid-out bounds.
func TestPopupFlipsFromKeyboard(t *testing.T) {
	g := newGallery()
	h := kittest.NewT(t, 1100, 260, g.Frame)
	h.Theme = g.th
	goTo(t, h, g, "pickers")
	f := h.ExpectID("zone/field")
	for i := 0; h.FocusedID() != "zone/field"; i++ {
		if i == 60 {
			t.Fatal("Tab never reached zone/field")
		}
		h.Tab(false)
	}
	h.Key(key.NameReturn, 0)
	p := h.ExpectID("zone")
	if p.Min.Y > f.Min.Y {
		t.Errorf("opened by keyboard at the bottom, the panel didn't flip: %v, field %v", p, f)
	}
	h.ExpectClean()
}

// Select: opens on the selected option, arrows move, Enter picks; the field
// shows the pick; a long list scrolls inside its panel.
func TestSelect(t *testing.T) {
	g, h := overlaysPage(t, "pickers")
	h.ClickID("size/field")
	if f := h.FocusedID(); f != "size/Medium" {
		t.Errorf("open: focus %q, want the selected option size:1", f)
	}
	if w, f := h.ExpectID("size").Dx(), h.ExpectID("size/field").Dx(); w < f {
		t.Errorf("panel %dpx narrower than the field %dpx", w, f)
	}
	h.ExpectClean()
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameReturn, 0)
	if g.overlays.sizeI != 2 || g.overlays.sizeSel.Opened() {
		t.Fatalf("keyboard pick: index %d, open %v", g.overlays.sizeI, g.overlays.sizeSel.Opened())
	}
	h.ExpectTextIn("size/field", "Large")
	h.ClickID("size/field")
	h.ClickID("size/Small")
	if g.overlays.sizeI != 0 {
		t.Fatalf("mouse pick: index %d", g.overlays.sizeI)
	}
	h.ClickID("zone/field")
	h.ExpectID("zone/list")
	h.ScrollID("zone/list", 600)
	h.ClickID(kit.Part("zone", "Pacific/Honolulu"))
	if g.overlays.zoneI != 19 {
		t.Fatalf("picked zone %d, want 19", g.overlays.zoneI)
	}
}

// Context menu: a right-click opens it at the pointer; a left click doesn't.
func TestContextMenu(t *testing.T) {
	g, h := overlaysPage(t, "menus")
	area := h.ExpectID("ctx/area")
	at := area.Min.Add(image.Pt(60, 20))
	h.Click(at)
	if g.overlays.ctxMenu.Opened() {
		t.Fatal("left click opened the context menu")
	}
	h.RightClick(at)
	if !g.overlays.ctxMenu.Opened() {
		t.Fatal("right-click did not open the context menu")
	}
	if r := h.ExpectID("ctx"); abs(r.Min.X-at.X) > 2 || abs(r.Min.Y-at.Y) > 2 {
		t.Errorf("menu at %v, pointer at %v", r.Min, at)
	}
	h.ExpectClean()
	h.ClickID("ctx/rename")
	if g.status != "Context → rename" || g.overlays.ctxMenu.Opened() {
		t.Fatalf("pick: status %q, open %v", g.status, g.overlays.ctxMenu.Opened())
	}
}

func abs(v int) int { return max(v, -v) }

// Popover holds a little form: it stays open while you use it.
func TestPopoverForm(t *testing.T) {
	g, h := overlaysPage(t, "overlays")
	h.ClickID("filters:btn")
	h.ClickID("filters:open-only")
	h.ClickID("filters:mine")
	if !g.overlays.filters.Opened() || !g.overlays.openOnly || !g.overlays.mineOnly {
		t.Fatalf("popover form: open %v, openOnly %v, mine %v", g.overlays.filters.Opened(), g.overlays.openOnly, g.overlays.mineOnly)
	}
	h.ExpectClean()
	h.ClickID("filters:apply")
	if g.overlays.filters.Opened() {
		t.Fatal("Apply did not close the popover")
	}
}

// Dialog: modal (the page takes no input), Tab stays inside, Escape and the
// backdrop dismiss it, its buttons work.
func TestDialogModal(t *testing.T) {
	g, h := overlaysPage(t, "overlays")
	h.ClickID("dialog:open")
	if !g.overlays.dialog.Opened() {
		t.Fatal("dialog did not open")
	}
	h.ExpectClean()
	if !h.Disabled("toast:info") {
		t.Error("the page behind the dialog is not disabled")
	}
	h.ExpectFocusTrapped("dialog", 6)
	h.ExpectEscapeCloses("dialog")
	if g.status != "Delete cancelled" {
		t.Errorf("Escape: status %q, want the dismissal reported", g.status)
	}
	h.ClickID("dialog:open")
	h.Click(image.Pt(10, 10)) // the backdrop
	if g.overlays.dialog.Opened() {
		t.Fatal("backdrop click did not close the dialog")
	}
	h.ClickID("dialog:open")
	h.ClickID("dialog:delete")
	if g.overlays.dialog.Opened() || g.overlays.toasts.Len() != 1 {
		t.Fatalf("Delete: dialog open %v, toasts %d", g.overlays.dialog.Opened(), g.overlays.toasts.Len())
	}
}

// Toasts stack bottom-right, newest last, and close by button or after
// ToastDuration (on the virtual clock).
func TestToasts(t *testing.T) {
	g, h := overlaysPage(t, "overlays")
	h.ClickID("toast:info")
	h.ClickID("toast:bad")
	if g.overlays.toasts.Len() != 2 {
		t.Fatalf("toasts: %d", g.overlays.toasts.Len())
	}
	a, b := h.ExpectID("toasts/1"), h.ExpectID("toasts/2")
	if b.Min.Y < a.Max.Y || b.Max.X > h.Size.X {
		t.Errorf("toasts not stacked newest-last at the bottom right: %v, %v", a, b)
	}
	h.ExpectText("Build failed")
	h.ExpectClean()
	h.ClickID("toasts/1/dismiss")
	if g.overlays.toasts.Len() != 1 {
		t.Fatalf("close button: %d left", g.overlays.toasts.Len())
	}
	h.Wait(ui.ToastDuration)
	if g.overlays.toasts.Len() != 0 {
		t.Fatalf("after ToastDuration: %d left", g.overlays.toasts.Len())
	}
	h.ExpectNoID("toasts/2")
}

// TestOverlaySheet renders every overlay open, for review
// (KITTEST_SHOTS=dir).
func TestOverlaySheet(t *testing.T) {
	dir := os.Getenv("KITTEST_SHOTS")
	if dir == "" {
		t.Skip("set KITTEST_SHOTS")
	}
	_, h := overlaysPage(t, "menus")
	h.ClickID("file:btn")
	h.Key(key.NameDownArrow, 0)
	h.Snap("file menu (keyboard)")
	h.Key(key.NameEscape, 0)
	h.ClickID("zone/field")
	h.Snap("zone select")
	h.Key(key.NameEscape, 0)
	h.RightClick(h.ExpectID("ctx/area").Min.Add(image.Pt(300, 30)))
	h.Snap("context menu")
	h.Key(key.NameEscape, 0)
	h.ClickID("filters:btn")
	h.Snap("popover")
	h.Key(key.NameEscape, 0)
	h.ClickID("more:btn")
	h.Snap("more menu, shifted in from the edge")
	h.Key(key.NameEscape, 0)
	h.ClickID("toast:info")
	h.ClickID("toast:good")
	h.ClickID("dialog:open")
	h.Snap("dialog + toasts")
	if err := h.SaveSheet(filepath.Join(dir, "overlays.png"), 2, 1); err != nil {
		t.Fatal(err)
	}
}

// DateField: opens on the current date, keyboard moves by day/week and pages
// months, Enter picks; clicking a day picks it.
func TestDateField(t *testing.T) {
	g, h := overlaysPage(t, "pickers")
	h.ExpectTextIn("due/field", "Fri, Nov 20, 2026")
	h.ClickID("due/field")
	if f := h.FocusedID(); f != "due/2026-11-20" {
		t.Fatalf("open: focus %q, want the selected day", f)
	}
	h.ExpectClean()
	h.Key(key.NameRightArrow, 0) // 21
	h.Key(key.NameDownArrow, 0)  // 28
	h.Key(key.NameDownArrow, 0)  // Dec 5: pages to December
	if f := h.FocusedID(); f != "due/2026-12-05" {
		t.Fatalf("after Right, Down, Down: focus %q, want due:2026-12-05", f)
	}
	h.ExpectText("December 2026")
	h.Key(key.NameReturn, 0)
	if g.overlays.due.Opened() || !g.overlays.dueDate.Equal(time.Date(2026, 12, 5, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("Enter: open %v, date %v", g.overlays.due.Opened(), g.overlays.dueDate)
	}
	h.ClickID("due/field")
	h.ClickID("due/prev")
	h.ClickID("due/2026-11-03")
	if !g.overlays.dueDate.Equal(time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("click: date %v", g.overlays.dueDate)
	}
	h.ExpectTextIn("due/field", "Tue, Nov 3, 2026")
}

// DateField zooms out for far dates: title to months, again to years,
// prev steps a decade; a year opens its months, a month its days. The
// keyboard moves across the month and year grids.
func TestDateFieldYearsAndMonths(t *testing.T) {
	g, h := overlaysPage(t, "pickers")
	h.ClickID("due/field")
	h.ClickID("due/view")
	h.ExpectText("2026")
	if f := h.FocusedID(); f != "due/2026-11" {
		t.Fatalf("months view: focus %q, want the shown month", f)
	}
	h.ClickID("due/view")
	h.ExpectText("2020–2029")
	if f := h.FocusedID(); f != "due/2026" {
		t.Fatalf("years view: focus %q, want the shown year", f)
	}
	h.ExpectClean()
	for range 7 {
		h.ClickID("due/prev")
	}
	h.ExpectText("1950–1959")
	h.ClickID("due/1950")
	h.ExpectText("1950")
	h.ClickID("due/1950-03")
	h.ExpectText("March 1950")
	h.ClickID("due/1950-03-14")
	if g.overlays.due.Opened() || !g.overlays.dueDate.Equal(time.Date(1950, 3, 14, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("birthday: open %v, date %v", g.overlays.due.Opened(), g.overlays.dueDate)
	}

	// Keyboard: reopen on March 1950, zoom out, move, pick with Enter.
	h.ClickID("due/field")
	h.ClickID("due/view")        // months of 1950, focus on March
	h.Key(key.NameDownArrow, 0)  // June
	h.Key(key.NameRightArrow, 0) // July
	if f := h.FocusedID(); f != "due/1950-07" {
		t.Fatalf("month keys: focus %q, want due/1950-07", f)
	}
	h.Key(key.NameRightArrow, 0) // Aug
	h.Key(key.NameDownArrow, 0)  // Nov
	h.Key(key.NameDownArrow, 0)  // past December: Feb 1951
	h.ExpectText("1951")
	h.Key(key.NameReturn, 0) // into February 1951's days
	h.ExpectText("February 1951")
	if f := h.FocusedID(); f != "due/1951-02-01" {
		t.Fatalf("Enter on a month: focus %q, want its first day", f)
	}
	h.ClickID("due/view")
	h.ClickID("due/view")     // years: 1950–1959, focus 1951
	h.Key(key.NameUpArrow, 0) // 1948: the previous decade
	h.ExpectText("1940–1949")
	if f := h.FocusedID(); f != "due/1948" {
		t.Fatalf("year keys: focus %q, want due/1948", f)
	}
	h.ExpectClean()
}

// Combobox: typing filters, Down/Enter takes a suggestion, a click does too,
// Escape closes the list, focus stays in the field throughout.
func TestCombobox(t *testing.T) {
	g, h := overlaysPage(t, "text")
	h.ClickID("prof")
	h.Type("ka")
	if !g.overlays.prof.Opened() {
		t.Fatal("typing a match did not open the suggestions")
	}
	if f := h.FocusedID(); f != "prof" {
		t.Errorf("focus moved to %q, want the field", f)
	}
	h.ExpectID("prof/opt/Kathryn Hare")
	h.ExpectID("prof/opt/Lila Kari")
	h.ExpectClean()
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameReturn, 0)
	if got := g.overlays.prof.Editor.Text(); got != "Karen Yeats" || g.overlays.prof.Opened() {
		t.Fatalf("Down Down Enter: %q (open %v)", got, g.overlays.prof.Opened())
	}
	g.overlays.prof.Editor.SetText("")
	h.Type("menez")
	h.ClickID("prof/opt/Alfred Menezes")
	if got := g.overlays.prof.Editor.Text(); got != "Alfred Menezes" {
		t.Fatalf("click: %q", got)
	}
	g.overlays.prof.Editor.SetText("")
	h.Type("i")
	h.Key(key.NameEscape, 0)
	if g.overlays.prof.Opened() {
		t.Fatal("Escape did not close the suggestions")
	}
	if g.overlays.prof.Editor.Text() != "i" {
		t.Error("Escape changed the text")
	}
}

// Disclosure toggles by click and keyboard; Flow wraps its children.
func TestDisclosureFlow(t *testing.T) {
	g, h := overlaysPage(t, "disclosure")
	h.ExpectNoID("tags:body")
	if _, ok := h.FindText("Lila Kari"); ok {
		t.Fatal("closed disclosure shows its body")
	}
	h.ClickID("tags")
	if !g.overlays.tagsOpen.Open {
		t.Fatal("click did not open")
	}
	header := h.ExpectID("tags")
	first, _ := h.FindText("Kathryn Hare")
	last, _ := h.FindText("Alfred Menezes")
	if first.Empty() || last.Empty() || first.Min.Y < header.Max.Y {
		t.Fatalf("chips: first %v, last %v, header %v", first, last, header)
	}
	for i := 0; i < 40 && h.FocusedID() != "tags"; i++ {
		h.Tab(false)
	}
	h.Key(key.NameSpace, 0)
	if g.overlays.tagsOpen.Open {
		t.Fatal("Space on the focused header did not close it")
	}
	h.ExpectClean()
}
