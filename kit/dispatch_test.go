package kit

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/hhhapz/giokit/gio/f32"
	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/io/event"
	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/io/pointer"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/widget"
)

// windowRunner drives frames through a Window and a Gio router, the way
// App.Run does, without a display.
type windowRunner struct {
	w   Window
	r   input.Router
	ops op.Ops
}

func (wr *windowRunner) frame(draw func(gtx layout.Context)) {
	wr.ops.Reset()
	gtx := wr.w.Begin(layout.Context{
		Ops:         &wr.ops,
		Source:      wr.r.Source(),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Now:         time.Unix(0, 0),
	})
	draw(gtx)
	wr.w.End(gtx)
	wr.r.Frame(&wr.ops)
}

// focus gives tag focus, with frames before and after so the router knows
// the tag and the widgets see the focus.
func (wr *windowRunner) focus(draw func(gtx layout.Context), tag event.Tag) {
	wr.frame(draw)
	wr.r.Source().Execute(key.FocusCmd{Tag: tag})
	wr.frame(draw)
}

// press queues a key press and runs a frame to dispatch it.
func (wr *windowRunner) press(draw func(gtx layout.Context), name key.Name, mods key.Modifiers) {
	wr.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	wr.frame(draw)
}

// customFocusable is a focusable built from Gio primitives and kit's
// exported API only, the way kit/ui builds a slider or a split handle.
type customFocusable struct{}

func (c *customFocusable) layout(gtx layout.Context, id string) {
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: c}); !ok {
			break
		}
	}
	defer clip.Rect(image.Rect(0, 0, 10, 10)).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, c)
	Tag(gtx, id)
	FocusID(gtx, c, id)
}

// A custom focusable that declares itself with FocusID takes part in key
// dispatch: an action of the KeyContext around it runs while it is focused.
func TestFocusIDJoinsKeyContext(t *testing.T) {
	var wr windowRunner
	var c customFocusable
	got := ""
	draw := func(gtx layout.Context) {
		OnAction(gtx, ActConfirm, func() { got = "root" })
		KeyContext(gtx, "editor", func(gtx layout.Context) layout.Dimensions {
			OnAction(gtx, ActConfirm, func() { got = "editor" })
			c.layout(gtx, "custom")
			return layout.Dimensions{}
		})
	}
	wr.focus(draw, &c)
	if id := wr.w.FocusedID(wr.r.Source()); id != "custom" {
		t.Fatalf("focused id %q, want custom", id)
	}
	wr.press(draw, key.NameReturn, 0)
	if got != "editor" {
		t.Fatalf("Enter on a custom focusable ran %q, want the enclosing context's handler", got)
	}
}

// A focusable registered twice in one frame (a dialog tracks its focus
// inside its own context, then names it outside) resolves from its first
// registration.
func TestFocusFirstRegistrationWins(t *testing.T) {
	var wr windowRunner
	var c customFocusable
	got := ""
	draw := func(gtx layout.Context) {
		KeyContext(gtx, "dialog", func(gtx layout.Context) layout.Dimensions {
			OnAction(gtx, ActCancel, func() { got = "dialog" })
			trackFocus(gtx, &c)
			return layout.Dimensions{}
		})
		c.layout(gtx, "custom")
	}
	wr.focus(draw, &c)
	wr.press(draw, key.NameEscape, 0)
	if got != "dialog" {
		t.Fatalf("Escape ran %q, want the context of the first registration", got)
	}
}

// A tag named twice in one frame keeps its first id, as it keeps its first
// KeyContext.
func TestFocusIDFirstNameWins(t *testing.T) {
	var wr windowRunner
	var c customFocusable
	draw := func(gtx layout.Context) {
		c.layout(gtx, "first")
		FocusID(gtx, &c, "second")
	}
	wr.focus(draw, &c)
	if id := wr.w.FocusedID(wr.r.Source()); id != "first" {
		t.Fatalf("focused id %q, want first", id)
	}
}

// A shortcut bound to a disabled action does not take its chord: the key
// bubbles to the next binding outward. Once enabled, the action runs.
func TestDisabledActionBindingBubbles(t *testing.T) {
	var wr windowRunner
	enabled := false
	ran := ""
	var a Actions
	a.MustRegister(Action{ID: "inner", Title: "Inner", Run: func() { ran = "inner" }, Enabled: func() bool { return enabled }})
	var inner, outer Keymap
	if err := inner.BindActions(&a, map[string]string{"inner": "Ctrl+K"}, nil); err != nil {
		t.Fatal(err)
	}
	outer.MustBind("Ctrl+K", "Outer", func() { ran = "outer" })
	var c Clickable
	draw := func(gtx layout.Context) {
		outer.Attach(gtx)
		KeyContext(gtx, "inner", func(gtx layout.Context) layout.Dimensions {
			inner.Attach(gtx)
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(10, 10)} })
		})
	}
	wr.focus(draw, &c)
	wr.press(draw, "K", key.ModCtrl)
	if ran != "outer" {
		t.Fatalf("disabled action: ran %q, want the outer binding", ran)
	}
	enabled, ran = true, ""
	wr.press(draw, "K", key.ModCtrl)
	if ran != "inner" {
		t.Fatalf("enabled action: ran %q, want inner", ran)
	}
}

// Re-applying overrides after the user clears a shortcut must remove the
// binding, not leave the old one live.
func TestBindActionsRemovesClearedShortcut(t *testing.T) {
	var a Actions
	a.MustRegister(Action{ID: "save", Title: "Save", Run: func() {}})
	a.MustRegister(Action{ID: "open", Title: "Open", Run: func() {}})
	defaults := map[string]string{"save": "Ctrl+S", "open": "Ctrl+O"}
	var km Keymap
	km.MustBind("Ctrl+Q", "Quit", func() {}) // not an action: kept
	if err := km.BindActions(&a, defaults, nil); err != nil {
		t.Fatal(err)
	}
	if err := km.BindActions(&a, defaults, []byte(`{"save":"","open":"Ctrl+P"}`)); err != nil {
		t.Fatal(err)
	}
	var chords []string
	for _, b := range km.Bindings() {
		chords = append(chords, b.Chord)
	}
	if !slices.Equal(chords, []string{"Ctrl+Q", "Ctrl+P"}) {
		t.Fatalf("bindings after clearing save and moving open: %v", chords)
	}
	if km.HintFor("save") != "" || km.HintFor("open") != "Ctrl+P" {
		t.Fatalf("hints: save %q, open %q", km.HintFor("save"), km.HintFor("open"))
	}
	// A failing call changes nothing.
	if err := km.BindActions(&a, defaults, []byte(`{"open":"Hyper+P"}`)); err == nil {
		t.Fatal("invalid chord accepted")
	}
	if len(km.Bindings()) != 2 {
		t.Fatalf("a failed BindActions changed the keymap: %+v", km.Bindings())
	}
}

// Bindings is a snapshot: changing it does not change the keymap.
func TestBindingsIsACopy(t *testing.T) {
	var km Keymap
	km.MustBind("Ctrl+K", "Palette", func() {})
	b := km.Bindings()
	b[0].Title = "Changed"
	if km.Hint("Palette") != "Ctrl+K" {
		t.Fatal("changing the slice Bindings returned changed the keymap")
	}
}

func TestParseChordPlusAndFunctionKeys(t *testing.T) {
	for _, c := range []struct {
		in   string
		name key.Name
		mods key.Modifiers
	}{
		{in: "+", name: "+"},
		{in: "Ctrl++", name: "+", mods: key.ModCtrl},
		{in: "Ctrl+Shift++", name: "+", mods: key.ModCtrl | key.ModShift},
		{in: "F1", name: key.NameF1},
		{in: "f12", name: key.NameF12},
		{in: "Shift+F5", name: key.NameF5, mods: key.ModShift},
		{in: "Back", name: key.NameBack},
	} {
		n, m, err := ParseChord(c.in)
		if err != nil || n != c.name || m != c.mods {
			t.Errorf("%q → %q %v %v, want %q %v", c.in, n, m, err, c.name, c.mods)
		}
	}
	for _, bad := range []string{"F0", "F13", "F99", "Ctrl+F13", "++", "+K"} {
		if _, _, err := ParseChord(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
	// The plus key round-trips through its display form.
	n, m, _ := ParseChord("Ctrl++")
	if n2, m2, err := ParseChord(FormatChord(n, m)); err != nil || n2 != n || m2 != m {
		t.Errorf("Ctrl++ does not round-trip: %q", FormatChord(n, m))
	}
}

// Theme.Press animates hover at the theme's motion speed: with Motion 0
// the hover value jumps straight to 1.
func TestPressHonoursMotion(t *testing.T) {
	for _, tc := range []struct {
		motion float32
		jumps  bool
	}{{motion: 1, jumps: false}, {motion: 0, jumps: true}} {
		th := Dark()
		th.Motion = tc.motion
		var wr windowRunner
		var c Clickable
		var hover float32
		draw := func(gtx layout.Context) {
			th.Press(gtx, &c, "b", func(gtx layout.Context, st State) layout.Dimensions {
				hover = st.Hover
				return layout.Dimensions{Size: image.Pt(50, 50)}
			})
		}
		wr.frame(draw)
		wr.r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(10, 10)})
		wr.frame(draw)
		if !c.Hovered() {
			t.Fatalf("motion %v: pointer move did not hover", tc.motion)
		}
		if got := hover == 1; got != tc.jumps {
			t.Errorf("motion %v: hover %v on the first hovered frame", tc.motion, hover)
		}
	}
}

func TestFocusVisible(t *testing.T) {
	var v FocusVisible
	if !v.Visible(true) {
		t.Fatal("zero value should show the ring")
	}
	v.Hide()
	if v.Visible(true) {
		t.Fatal("ring shown after a pointer focus")
	}
	if v.Visible(false) || !v.Visible(true) {
		t.Fatal("losing focus should reset the ring to visible")
	}
	v.Hide()
	v.Show()
	if !v.Visible(true) {
		t.Fatal("Show did not show the ring")
	}
}

// While a text field has focus, single-key shortcuts leave their keys to
// it, and chords with Ctrl still work; unfocused, the single key runs.
func TestTypingKeepsPrintableKeys(t *testing.T) {
	var wr windowRunner
	ran := ""
	var km Keymap
	km.MustBind("1", "One", func() { ran += "1" })
	km.MustBind("Shift+/", "Help", func() { ran += "?" })
	km.MustBind("Ctrl+S", "Save", func() { ran += "s" })
	var e widget.Editor
	var c Clickable
	th := Dark()
	draw := func(gtx layout.Context) {
		km.Attach(gtx)
		th.EditorField(gtx, &e, "field", EditorField{})
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(10, 10)} })
	}
	wr.focus(draw, &e)
	wr.press(draw, "1", 0)
	wr.press(draw, "/", key.ModShift)
	wr.press(draw, "S", key.ModCtrl)
	if ran != "s" {
		t.Fatalf("typing in a field ran %q, want only the Ctrl chord", ran)
	}
	ran = ""
	wr.focus(draw, &c)
	wr.press(draw, "1", 0)
	if ran != "1" {
		t.Fatalf("outside a field ran %q, want 1", ran)
	}
}

// An editor drawn by the app itself, declared with TextInput, keeps
// printable keys from shortcuts as EditorField does.
func TestTextInputKeepsPrintableKeys(t *testing.T) {
	var wr windowRunner
	ran := ""
	var km Keymap
	km.MustBind("M", "Mute", func() { ran += "m" })
	var e widget.Editor
	var c Clickable
	th := Dark()
	draw := func(gtx layout.Context) {
		km.Attach(gtx)
		e.Layout(gtx, th.Shaper, font.Font{}, 12, op.CallOp{}, op.CallOp{})
		TextInput(gtx, &e)
		c.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(10, 10)} })
	}
	wr.focus(draw, &e)
	wr.press(draw, "M", 0)
	if ran != "" {
		t.Fatalf("typing in a custom editor ran %q", ran)
	}
	wr.focus(draw, &c)
	wr.press(draw, "M", 0)
	if ran != "m" {
		t.Fatalf("outside it ran %q, want m", ran)
	}
}

// Escape and F-keys reach shortcuts while an editor has focus; the keys
// that edit text stay the editor's. Escape is a one-rune key name ("⎋"),
// which was once taken for a printable character (audit 16, CP-03).
func TestTypingPassesSpecialKeys(t *testing.T) {
	var wr windowRunner
	ran := ""
	var km Keymap
	km.MustBind("Escape", "Close", func() { ran += "esc " })
	km.MustBind("F2", "Rename", func() { ran += "f2 " })
	km.MustBind("Left", "Back", func() { ran += "left " })
	km.MustBind("Backspace", "Up", func() { ran += "bs " })
	var e widget.Editor
	th := Dark()
	draw := func(gtx layout.Context) {
		km.Attach(gtx)
		th.EditorField(gtx, &e, "field", EditorField{})
	}
	wr.focus(draw, &e)
	wr.press(draw, key.NameEscape, 0)
	wr.press(draw, key.NameF2, 0)
	wr.press(draw, key.NameLeftArrow, 0)
	wr.press(draw, key.NameDeleteBackward, 0)
	if ran != "esc f2 " {
		t.Fatalf("typing in a field ran %q, want esc f2", ran)
	}
}

// Display chords use words for keys Gio names with symbols the bundled
// fonts lack, and parse back to the same chord.
func TestFormatChordWords(t *testing.T) {
	for spec, want := range map[string]string{
		"Ctrl+Delete": "Ctrl+Delete", "Backspace": "Backspace", "Shift+Home": "Shift+Home",
		"End": "End", "PageUp": "PageUp", "Ctrl+PageDown": "Ctrl+PageDown", "Ctrl+Enter": "Ctrl+Enter",
	} {
		n, m, err := ParseChord(spec)
		if err != nil {
			t.Fatal(err)
		}
		got := FormatChord(n, m)
		if got != want {
			t.Errorf("FormatChord(%q) = %q, want %q", spec, got, want)
		}
		if n2, m2, err := ParseChord(got); err != nil || n2 != n || m2 != m {
			t.Errorf("%q does not parse back: %v", got, err)
		}
	}
	if got := FormatChord(key.NameEnter, 0); got != "Enter" {
		t.Errorf("keypad Enter: %q", got)
	}
}
