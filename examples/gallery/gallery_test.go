package main

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

func newHarness(t *testing.T) (*gallery, *kittest.Harness) {
	g := newGallery()
	h := kittest.NewT(t, 1100, 720, g.Frame)
	h.Theme = g.th
	return g, h
}

// goTo opens a page the way a user would, through the command palette.
func goTo(t *testing.T, h *kittest.Harness, g *gallery, id string) {
	t.Helper()
	title := ""
	for _, p := range pages {
		if p.id == id {
			title = p.title
		}
	}
	if title == "" {
		t.Fatalf("no page %q", id)
	}
	h.Chord("Shortcut+P")
	h.Type("Go to " + title)
	h.Key(key.NameReturn, 0)
	if g.page != id {
		t.Fatalf("palette: page %q, want %q", g.page, id)
	}
}

func TestAuditClean(t *testing.T) {
	_, h := newHarness(t)
	h.ExpectClean()
}

// Every page is reachable (the first nine by shortcut, all from the
// palette), animates in, settles, and passes the audit.
func TestEveryPageClean(t *testing.T) {
	g, h := newHarness(t)
	for i, p := range pages {
		if i < 9 {
			h.Chord("Shortcut+" + string(rune('1'+i)))
			if g.page != p.id {
				t.Fatalf("shortcut %d: page = %q, want %q", i+1, g.page, p.id)
			}
		} else {
			goTo(t, h, g, p.id)
		}
		h.ExpectText(p.title)
		h.ExpectClean()
	}
}

func TestRowHoverFollowsPointer(t *testing.T) {
	g, h := newHarness(t)
	scrollTo(t, h, g, "row:5")                                    // the Tasks card sits below the theme bar and the game
	for _, id := range []string{"row:2", "row:4", "qty:2/plus"} { // a control keeps its row lit
		h.Hover(id)
		lit := "row:2"
		if id == "row:4" {
			lit = "row:4"
		}
		for _, r := range []string{"row:1", "row:2", "row:3", "row:4"} {
			want := g.th.Panel // the Tasks card behind the rows
			if r == lit {
				want = g.th.Hover
			}
			h.ExpectBg(r, want)
		}
	}
}

func TestStepperAndCheckbox(t *testing.T) {
	g, h := newHarness(t)
	scrollTo(t, h, g, "row:5")
	h.ClickID("qty:3/plus")
	h.ExpectTextIn("row:3", "1.5")
	h.ExpectText("5.5") // the Quantity stat caught up in the same settle
	h.ClickID("done:3")
	if !g.find("3").Done {
		t.Fatal("checkbox did not toggle")
	}
	h.ExpectTextIn("row:3", "done")
	h.ExpectText("1/5")
	h.ExpectTextIn("nav:overview", "4")
}

func TestNavAndButtons(t *testing.T) {
	_, h := newHarness(t)
	h.ClickID("nav:buttons")
	h.ExpectID("btn:Danger")
	h.ClickID("primary")
	h.ClickID("primary")
	h.ExpectText("Primary clicked 2×")
	h.ClickID("btn:Danger")
	h.ExpectText("Danger clicked")
	h.ExpectClean()
}

func TestTogglesAndRadio(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "selection")
	before := h.BgColor("switch:notify")
	h.ClickID("switch:notify")
	if !g.notifyOn {
		t.Fatal("switch did not toggle")
	}
	if kittest.Near(before, h.BgColor("switch:notify"), 4) {
		t.Error("switch track did not change color (animation not settled?)")
	}
	h.ClickID("check:agree")
	if !g.agreed {
		t.Fatal("checkbox did not toggle")
	}
	goTo(t, h, g, "buttons") // the size radios live beside the buttons they size
	h.ClickID("radio:Large")
	if g.size != "Large" {
		t.Fatalf("radio: size = %q", g.size)
	}
}

// Keyboard: Tab reaches buttons, Enter activates; the focus ring draws.
func TestKeyboardFocusAndActivate(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "buttons")
	r := h.ExpectID("btn:Secondary")
	edge := image.Pt(r.Min.X+1, r.Min.Y+r.Dy()/2)
	before := h.Pixel(edge)
	for i := 0; i < 20 && h.Pixel(edge) == before; i++ {
		h.Tab(false)
	}
	if h.Pixel(edge) == before {
		t.Fatal("no focus ring appeared on btn:Secondary after tabbing")
	}
	h.Key(key.NameReturn, 0)
	if g.status != "Secondary clicked" {
		t.Fatalf("Enter on focused button: status = %q", g.status)
	}
}

func TestTextFieldSubmit(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "text")
	h.ClickID("field:name")
	h.Type("Ada")
	h.Key(key.NameReturn, 0)
	if g.status != "Hello, Ada" {
		t.Fatalf("status = %q", g.status)
	}
	h.ExpectText("Hello, Ada")
}

func TestSliderKeyboard(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "selection")
	h.ClickID("slider:volume") // clicking jumps the value to the pointer; focus with Tab instead
	start := g.volume.Value
	for i := 0; i < 30; i++ { // tab until the slider has focus (arrow keys move it)
		h.Key(key.NameRightArrow, 0)
		if g.volume.Value != start {
			break
		}
		h.Tab(false)
	}
	if g.volume.Value <= start {
		t.Fatalf("Right arrow did not raise the slider (%v → %v)", start, g.volume.Value)
	}
	h.Key(key.NameEnd, 0)
	if g.volume.Value != 1 {
		t.Fatalf("End: value = %v", g.volume.Value)
	}
	h.ExpectText("100%")
}

func TestTabsMoveIndicator(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "tabs")
	h.ClickID("tab/Terminal")
	if g.tab != 2 {
		t.Fatalf("tab = %d", g.tab)
	}
	h.ExpectText("Terminal settings would go here.")
	// the underline sits under the selected tab once the animation settled
	r := h.ExpectID("tab/Terminal")
	under := h.Pixel(image.Pt(kittest.Center(r).X, r.Max.Y+gtxPad(h)))
	if !kittest.Near(under, g.th.Accent, 30) {
		t.Errorf("no accent underline under the selected tab: %v", under)
	}
}

// gtxPad is the underline's offset below a tab's bounds (Tabs pads 6dp).
func gtxPad(h *kittest.Harness) int { return h.Metric.Dp(6) - 1 }

func TestTooltipAfterDelay(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "buttons")
	h.Hover("btn:search")
	if _, ok := h.FindText("Search (tooltip)"); ok {
		t.Fatal("tooltip showed before the delay")
	}
	h.Wait(kit.TooltipDelay + 50*time.Millisecond)
	h.ExpectText("Search (tooltip)")
	h.Hover("btn:Ghost")
	if _, ok := h.FindText("Search (tooltip)"); ok {
		t.Error("tooltip stayed after the pointer left")
	}
}

func TestLogsFilterAndFollow(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "logs")
	h.ExpectText("200 of 200 lines")
	h.ClickID("level/Error")
	if n := g.logView.Count(&g.logBuf); n == 0 || n >= 200 {
		t.Fatalf("error filter shows %d lines", n)
	}
	h.ClickID("level/All")
	h.ClickID("logs:filter")
	h.Type("payments")
	n := g.logView.Count(&g.logBuf)
	if n == 0 || n >= 200 {
		t.Fatalf("text filter shows %d lines", n)
	}
	h.ExpectText(itoa(n) + " of 200 lines")
	h.ClickID("logs:add")
	if !g.logView.Following() {
		t.Error("view stopped following the tail after appending")
	}
	h.ScrollID("logs", -2000) // scroll up: pauses follow and shows the jump button
	h.ExpectID("logs/jump")
	h.ClickID("logs/jump")
	h.ExpectNoID("logs/jump")
	if !g.logView.Following() {
		t.Error("jump did not resume following")
	}
}

func TestLogStreamUsesVirtualClock(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "logs")
	h.ClickID("logs:stream")
	before := g.logBuf.Len()
	h.Wait(time.Second)
	if got := g.logBuf.Len() - before; got < 7 || got > 10 {
		t.Fatalf("1s of streaming at 120ms/line appended %d lines", got)
	}
}

func TestSplitDragAndKeys(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "panes")
	// The divider follows the pointer exactly (a drag once moved it ~2.5×:
	// Gio reports drag positions relative to the handle at press time).
	for _, dx := range []int{120, -70, 30} {
		before := h.ExpectID("split:outer")
		h.DragID("split:outer", dx, 0)
		if moved := h.ExpectID("split:outer").Min.X - before.Min.X; moved < dx-2 || moved > dx+2 {
			t.Fatalf("drag %+d moved the divider %+d", dx, moved)
		}
	}
	r := h.ExpectID("split:outer")
	files := h.ExpectID("file:main.go")
	if files.Max.X > r.Min.X+2 {
		t.Errorf("left pane content overflows the divider: %v vs %v", files, r)
	}
	before := h.ExpectID("split:inner")
	h.DragID("split:inner", 0, -80)
	if moved := h.ExpectID("split:inner").Min.Y - before.Min.Y; moved < -82 || moved > -78 {
		t.Fatalf("drag -80 moved the inner divider %+d", moved)
	}
	h.ExpectClean()
}

// A mouse wheel notch arrives as one big delta (Gio: ~150px on Wayland);
// th.List glides there over several frames instead of jumping.
func TestWheelGlides(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "logs")
	h.ScrollID("logs", -3000) // leave the tail
	abs := func() int { p := g.logView.ListPos(); return p.First*1000 + p.Offset }
	at := kittest.Center(h.ExpectID("logs"))
	h.Move(at)
	start, frames := abs(), h.Frames
	h.Scroll(at, 150)
	if h.Frames-frames < 10 { // baseline for any scroll call is ~4 (move, settle, stale check)
		t.Errorf("a 150px wheel step took %d frames: it jumped instead of gliding", h.Frames-frames)
	}
	p := g.logView.ListPos()
	if p == (layout.Position{}) || abs() == start {
		t.Fatal("wheel step did not scroll")
	}
	// small (touchpad) deltas apply immediately
	h.Move(at)
	frames = h.Frames
	h.Scroll(at, 6)
	if h.Frames-frames > 5 {
		t.Errorf("a 6px touchpad delta took %d frames", h.Frames-frames)
	}
	// A high-resolution wheel mini-step scaled by 5 is about 9px. It must
	// glide too, or repeated mini-steps look like a series of small jumps.
	h.Move(at)
	frames = h.Frames
	h.Scroll(at, 9)
	if h.Frames-frames < 7 {
		t.Errorf("a 9px scaled wheel mini-step took %d frames: it jumped instead of gliding", h.Frames-frames)
	}
}

// Scrolling through every page, by wheel notches and by touchpad-sized
// deltas, at scale 1 and 2, the content moves exactly as far as asked: no
// skips when items enter or leave the viewport.
func TestScrollContinuity(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		g := newGallery()
		h, err := kittest.New(1100, 720, scale, g.Frame)
		if err != nil {
			kittest.NoGPU(t, err)
		}
		h.Strict = false
		lists := map[string]*kit.List{"all": g.pageLists.Get("all"), "theme": &g.themeLst}
		for page, l := range lists {
			if page == "all" {
				h.Chord("Shortcut+2")
			} else {
				goTo(t, h, g, page)
			}
			at := kittest.Center(must(t, h, page))
			for _, d := range []float32{150, 150, 150, 7, 7, 9, 9, -150, -150, -9, -9, -7, -150, -150} {
				h.Scroll(at, d*scale)
			}
			if l.Jumps() != 0 {
				t.Errorf("scale %v, %s: %d skipped frames while scrolling", scale, page, l.Jumps())
			}
		}
		h.Release()
	}
}

func must(t *testing.T, h *kittest.Harness, id string) image.Rectangle {
	r, ok := h.Find(id)
	if !ok {
		t.Fatalf("no %q", id)
	}
	return r
}

func TestCheckboxLabelClick(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "selection")
	r := h.ExpectID("check:agree")
	h.Click(image.Pt(r.Max.X-6, kittest.Center(r).Y)) // on the "I agree" text
	if !g.agreed {
		t.Fatal("clicking the label did not toggle the checkbox")
	}
}

func TestSizeRadiosResizeButtons(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "buttons")
	medium := h.ExpectID("btn:Secondary").Dy()
	h.ClickID("radio:Large")
	if large := h.ExpectID("btn:Secondary").Dy(); large <= medium {
		t.Fatalf("Large buttons are %dpx tall, Medium %dpx", large, medium)
	}
	h.ExpectClean()
}

func TestArrowKeysMoveListSelection(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "panes")
	h.ClickID("file:controls.go")
	h.Key(key.NameDownArrow, 0)
	h.Key(key.NameDownArrow, 0)
	if g.openFile != "layoutpage.go" {
		t.Fatalf("after two Downs: %q", g.openFile)
	}
	h.Key(key.NameHome, 0)
	if g.openFile != "main.go" {
		t.Fatalf("after Home: %q", g.openFile)
	}
	// the sidebar too
	h.ClickID("nav:panes")
	h.Key(key.NameDownArrow, 0)
	if g.page != "forms" {
		t.Fatalf("sidebar Down: page %q", g.page)
	}
}

// Input bubbles: a focused slider takes the wheel; unfocused, the page
// scrolls. Clicking the slider focuses it (quietly).
func TestSliderWheelOnlyWhenFocused(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "all") // a page tall enough to scroll
	scroll := g.pageLists.Get("all")
	h.ScrollID("slider:volume", 40)
	if g.volume.Value != 0.4 {
		t.Fatalf("unfocused slider took the wheel: %v", g.volume.Value)
	}
	if scroll.Position.Offset == 0 && scroll.Position.First == 0 {
		t.Fatal("wheel over an unfocused slider did not scroll the page")
	}
	scroll.Position = layout.Position{} // back to the top
	h.Settle()
	h.ClickID("slider:volume") // sets the value at the pointer and focuses
	v := g.volume.Value
	pos := scroll.Position
	h.ScrollID("slider:volume", -150) // wheel up
	if g.volume.Value <= v {
		t.Fatalf("focused slider ignored the wheel: %v → %v", v, g.volume.Value)
	}
	if scroll.Position != pos {
		t.Errorf("wheel on a focused slider also scrolled the page: %+v → %+v", pos, scroll.Position)
	}
}

// Keys bubble: the focused field keeps the chords it handles (Ctrl+A);
// others (Ctrl+1) reach the window keymap.
func TestKeysBubbleFromFocusedWidget(t *testing.T) {
	g, h := newHarness(t)
	fired := false
	g.keys.MustBind("Shortcut+A", "Global A", func() { fired = true })
	goTo(t, h, g, "text")
	h.ClickID("field:name")
	h.Type("Ada")
	h.Chord("Shortcut+A")
	if fired {
		t.Error("the keymap stole Ctrl+A from the focused text field")
	}
	h.Chord("Shortcut+1")
	if g.page != "overview" {
		t.Errorf("Ctrl+1 inside a text field did not reach the keymap: page %q", g.page)
	}
	h.Chord("Shortcut+A") // no field focused now
	if !fired {
		t.Error("Ctrl+A with no focused field did not reach the keymap")
	}
}

func TestThemeSwitching(t *testing.T) {
	g, h := newHarness(t)
	bg := h.BgColor("nav:overview")
	h.Chord("Shortcut+T")
	if g.th.Name != "Light" {
		t.Fatalf("theme = %q", g.th.Name)
	}
	if kittest.Near(bg, h.BgColor("nav:overview"), 10) {
		t.Error("sidebar color did not change with the theme")
	}
	goTo(t, h, g, "theme")
	h.ClickID("preset:Paper")
	if g.th.Name != "Paper" {
		t.Fatalf("theme = %q", g.th.Name)
	}
	h.ClickID("preset:custom")
	if g.th.Name != "Custom" {
		t.Fatalf("theme = %q", g.th.Name)
	}
	scrollTo(t, h, g, "switch:motion")
	h.ClickID("density/Compact")
	if g.th.Density >= 1 {
		t.Fatalf("density = %v", g.th.Density)
	}
	h.ClickID("switch:motion")
	if g.th.Motion != 0 {
		t.Fatalf("motion = %v", g.th.Motion)
	}
	h.ExpectClean()
}

// Every option of every theme knob applies to the theme and leaves every
// demo page clean; the knob's own tab shows it selected.
func TestThemeKnobs(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "theme")
	for _, k := range g.knobs {
		for i, name := range k.names {
			id := kit.Part("knob:"+k.id, name)
			scrollTo(t, h, g, id)
			h.ClickID(id)
			if k.sel != i {
				t.Fatalf("%s: clicking %q selected %d", k.id, name, k.sel)
			}
			h.Theme = g.th
			h.ExpectClean()
		}
		// leave the default selected for the next knob
		def := map[string]int{"corners": 2, "ring": 1, "text": 1, "heading": 2, "elevation": 1, "controls": 1}[k.id]
		h.ClickID(kit.Part("knob:"+k.id, k.names[def]))
	}
	checks := map[string]func() bool{
		"corners/Pill":   func() bool { return g.th.Radius == 999 },
		"stroke/Bold":    func() bool { return g.th.Stroke == 2 },
		"ring/Thick":     func() bool { return g.th.Ring == 3 },
		"fonts/Atkinson": func() bool { return g.th.Fonts == "Atkinson" && g.th.UI == "Atkinson Hyperlegible" },
		"text/130%":      func() bool { return g.th.Type.Body > kit.DefaultType.Body*1.25 },
		"heading/Medium": func() bool { return g.th.HeadingWeight == kit.Medium },
		"elevation/Flat": func() bool { return g.th.Elevation == 0 },
		"contrast/High":  func() bool { return kit.Contrast(g.th.Border, g.th.Bg) >= 3 },
		"controls/Large": func() bool { return g.th.Controls == kit.ControlLarge },
	}
	for opt, ok := range checks {
		id := "knob:" + opt
		scrollTo(t, h, g, kit.Part("knob:"+opt[:strings.Index(opt, "/")], opt[strings.Index(opt, "/")+1:]))
		h.ClickID(kit.Part("knob:"+opt[:strings.Index(opt, "/")], opt[strings.Index(opt, "/")+1:]))
		if !ok() {
			t.Errorf("%s did not apply", id)
		}
	}
	h.Theme = g.th
	for _, p := range pages { // every page under the last mix of knobs
		goTo(t, h, g, p.id)
		h.ExpectClean()
	}
}

// Every preset renders every page cleanly (contrast is checked in kit's tests).
func TestPresetsClean(t *testing.T) {
	g, h := newHarness(t)
	for range g.presets {
		h.Chord("Shortcut+T")
		h.Theme = g.th // the audit judges text against the current palette
		for _, p := range pages {
			goTo(t, h, g, p.id)
			h.ExpectClean()
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

// TestSheet writes contact sheets of key states when KITTEST_SHOTS is set:
// a couple of images to review instead of many.
func TestSheet(t *testing.T) {
	dir := os.Getenv("KITTEST_SHOTS")
	if dir == "" {
		t.Skip("set KITTEST_SHOTS to write a contact sheet")
	}
	g, h := newHarness(t)
	h.Hover("row:2")
	h.Snap("overview, hover row 2")
	goTo(t, h, g, "all")
	h.Hover("btn:search")
	h.Wait(kit.TooltipDelay + 100*time.Millisecond)
	h.Snap("controls: tooltip")
	for i := 0; i < 12; i++ { // keyboard focus ring somewhere on the page
		h.Tab(false)
	}
	h.Snap("controls: focus after 12 Tabs")
	h.Chord("Shortcut+T")
	h.Snap("controls, light")
	if err := h.SaveSheet(filepath.Join(dir, "gallery-sheet.png"), 2, 2); err != nil {
		t.Fatal(err)
	}
}

// TestThemeSheet renders a few presets (a separate test: two live harnesses
// in one test share and then tear down one EGL display).
func TestThemeSheet(t *testing.T) {
	dir := os.Getenv("KITTEST_SHOTS")
	if dir == "" {
		t.Skip("set KITTEST_SHOTS to write a contact sheet")
	}
	g2, h2 := newHarness(t)
	for _, name := range []string{"Midnight", "Paper", "High contrast"} {
		for i, p := range g2.presets {
			if p.Name == name {
				g2.preset = i
				g2.applyTheme()
			}
		}
		h2.Chord("Shortcut+1")
		h2.Settle()
		h2.Snap("overview, " + name)
	}
	goTo(t, h2, g2, "panes")
	h2.Snap("panes, High contrast")
	if err := h2.SaveSheet(filepath.Join(dir, "gallery-themes.png"), 2, 2); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkFrame measures one full frame per page (layout + Gio's GPU prep
// and submit), no readback:
//
//	go test -run x -bench Frame ./examples/gallery
//
// Nov 2026, RX 6700 XT: overview ~0.75ms, controls ~1.2ms, logs ~1.1ms, of
// which our layout is 0.15–0.3ms. Before outlines replaced clip.Stroke,
// controls took ~2.9ms.
func BenchmarkFrame(b *testing.B) {
	for _, p := range pages {
		b.Run(p.id, func(b *testing.B) {
			g := newGallery()
			g.page = p.id
			h, err := kittest.New(1100, 720, 1, g.Frame)
			if err != nil {
				b.Skip(err)
			}
			defer h.Release()
			h.Strict = false
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.Settle()
			}
		})
	}
}

// scrollTo scrolls the current demo page until id is fully on screen.
func scrollTo(t *testing.T, h *kittest.Harness, g *gallery, id string) image.Rectangle {
	t.Helper()
	// Up or down: a virtualized list drops rows that scrolled out, so when
	// id is not on screen at all, try downward first, then upward.
	dir := float32(30)
	for i := range 120 {
		r, ok := h.Find(id)
		switch {
		case ok && r.Min.Y >= 120 && r.Max.Y <= h.Size.Y-40: // clear of the title bar and page header
			return r
		case ok && r.Min.Y < 120:
			dir = -30
		case ok:
			dir = 30
		case i == 40:
			dir = -30 // not found going down: it is above
		}
		h.ScrollID(g.page, dir)
	}
	t.Fatalf("%q never scrolled into view", id)
	return image.Rectangle{}
}

// th.Disable: nothing inside takes input, the semantics say disabled, the
// cursor stays an arrow; unlocking brings it all back in one click.
func TestDisabledFormTakesNoInput(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "forms")
	scrollTo(t, h, g, "lock:slider")
	for _, id := range []string{"lock:field", "lock:save", "lock:check", "lock:tab/Week", "lock:slider"} {
		if !h.Disabled(id) {
			t.Errorf("%s: not disabled in the semantics", id)
		}
	}
	if h.Disabled("switch:lock") {
		t.Error("the lock switch itself is disabled")
	}
	save := h.ExpectID("lock:save")
	bg := h.BgColor("lock:save")
	h.Hover("lock:save")
	if h.Cursor() != pointer.CursorDefault {
		t.Errorf("cursor over a disabled button: %v", h.Cursor())
	}
	h.ExpectBg("lock:save", bg) // no hover tint
	h.Click(kittest.Center(save))
	h.ClickID("lock:check")
	h.ClickID("lock:tab/Week")
	h.DragID("lock:slider", -80, 0)
	h.ClickID("lock:field")
	h.Type("xyz")
	if g.status == "Saved" || !g.lockOn || g.lockTab != 0 || g.lockSlide.Value != 0.6 ||
		g.lockField.Text() != "read-only while locked" {
		t.Fatalf("locked form took input: status %q, check %v, tab %d, slider %v, field %q",
			g.status, g.lockOn, g.lockTab, g.lockSlide.Value, g.lockField.Text())
	}
	h.ExpectClean()

	h.ClickID("switch:lock")
	if h.Disabled("lock:save") {
		t.Fatal("still disabled after unlocking")
	}
	h.ClickID("lock:save")
	h.ClickID("lock:check")
	h.ClickID("lock:tab/Week")
	if g.status != "Saved" || g.lockOn || g.lockTab != 1 {
		t.Fatalf("unlocked form ignored input: status %q, check %v, tab %d", g.status, g.lockOn, g.lockTab)
	}
	h.ExpectClean()
}

// A disabled control leaves the Tab order, and rejoins it when enabled.
func TestDisabledSkipsTabOrder(t *testing.T) {
	th := kit.Dark()
	var a, b kit.Clickable
	off, clicked := true, ""
	h := kittest.NewT(t, 300, 100, func(gtx C) {
		if a.Clicked(gtx) {
			clicked += "a"
		}
		if b.Clicked(gtx) {
			clicked += "b"
		}
		kit.Fill(gtx, th.Bg, func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D {
					return th.Disable(gtx, off, func(gtx C) D { return th.Button(gtx, &a, "a", "A", kit.Secondary) })
				}),
				kit.Rigid(func(gtx C) D { return th.Button(gtx, &b, "b", "B", kit.Secondary) }),
			)
		})
	})
	h.Settle() // a clickable learns it is disabled from its first layout
	h.Tab(false)
	h.Key(key.NameReturn, 0)
	if clicked != "b" {
		t.Fatalf("Tab, Enter with A disabled: clicked %q, want \"b\"", clicked)
	}
	off, clicked = false, ""
	h.Settle()
	h.Tab(true) // back from B to A
	h.Key(key.NameReturn, 0)
	if clicked != "a" {
		t.Fatalf("Shift+Tab, Enter with A enabled: clicked %q, want \"a\"", clicked)
	}
}

// Native single-line editing keys (Gio's Editor, patched): word delete with
// Ctrl on Linux/Windows, Left/Right collapsing a selection to its edge,
// Up/Down jumping to the start/end of a single-line field.
func TestTextFieldNativeKeys(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "text")
	h.ClickID("field:name")
	set := func(s string) {
		g.name.SetText(s)
		g.name.SetCaret(len(s), len(s))
		h.Settle()
	}
	expect := func(step, want string) {
		t.Helper()
		if got := g.name.Text(); got != want {
			t.Errorf("%s: %q, want %q", step, got, want)
		}
	}
	set("hello big world")
	h.Key(key.NameDeleteBackward, key.ModShortcutAlt)
	expect("Ctrl+Backspace", "hello big ")
	h.Key(key.NameHome, 0)
	h.Key(key.NameDeleteForward, key.ModShortcutAlt)
	expect("Ctrl+Delete at start", " big ")

	set("hello world")
	h.Chord("Shortcut+A")
	h.Key(key.NameRightArrow, 0)
	h.Type("!")
	expect("select all, Right", "hello world!")
	h.Chord("Shortcut+A")
	h.Key(key.NameLeftArrow, 0)
	h.Type(">")
	expect("select all, Left", ">hello world!")

	set("abc")
	h.Key(key.NameUpArrow, 0)
	h.Type("[")
	expect("Up", "[abc")
	h.Key(key.NameDownArrow, 0)
	h.Type("]")
	expect("Down", "[abc]")
	h.Key(key.NameUpArrow, key.ModShift)
	h.Type("x")
	expect("Shift+Up selects to the start", "x")
}

// TextArea: Enter inserts a newline instead of submitting.
func TestTextAreaNewlines(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "text")
	h.ClickID("field:notes")
	h.Type("one")
	h.Key(key.NameReturn, 0)
	h.Type("two")
	if got := g.notes.Text(); got != "one\ntwo" {
		t.Fatalf("text %q, want two lines", got)
	}
	h.ExpectClean()
}
