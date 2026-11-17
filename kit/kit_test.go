package kit

import (
	"errors"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
)

func TestTokensRoundTrip(t *testing.T) {
	styled := Dark().Derive(func(t *Theme) {
		t.Name, t.Radius, t.Stroke, t.Ring, t.Elevation = "Styled", 999, 1.5, 3, 0
		t.HeadingWeight, t.Controls, t.Fonts = SemiBold, ControlLarge, "Inter"
	})
	for _, th := range append(Presets(), styled) {
		back, err := ThemeFromTokens(th.Tokens(), Dark())
		if err != nil {
			t.Fatalf("%s: %v", th.Name, err)
		}
		if back.Palette != th.Palette || back.Name != th.Name || back.Dark != th.Dark ||
			back.Type != th.Type || back.Radius != th.Radius || back.Density != th.Density ||
			back.Stroke != th.Stroke || back.Ring != th.Ring || back.Elevation != th.Elevation ||
			back.HeadingWeight != th.HeadingWeight || back.Controls != th.Controls || back.Fonts != th.Fonts {
			t.Errorf("%s: round trip changed the theme", th.Name)
		}
	}
}

// A design-system file with reference ramps and aliases, in both DTCG color
// forms and both dimension forms.
func TestTokensAliasesAndForms(t *testing.T) {
	src := `{
	  "$name": "Brand",
	  "ref": {"blue": {"500": {"$type": "color", "$value": "#3366ff"}},
	          "gray": {"900": {"$type": "color", "$value": {"colorSpace": "srgb", "components": [0.1, 0.1, 0.1]}}}},
	  "color": {
	    "accent": {"$value": "{ref.blue.500}"},
	    "bg":     {"$value": "{ref.gray.900}"},
	    "focus":  {"$value": "{color.accent}"}
	  },
	  "radius":    {"$value": "10px"},
	  "font-size": {"body": {"$value": {"value": 1, "unit": "rem"}}}
	}`
	th, err := ThemeFromTokens([]byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if Hex(th.Accent) != "#3366ff" || th.Focus != th.Accent {
		t.Errorf("aliases: accent %s focus %s", Hex(th.Accent), Hex(th.Focus))
	}
	if Hex(th.Bg) != "#1a1a1a" {
		t.Errorf("component color: bg %s", Hex(th.Bg))
	}
	if th.Radius != 10 || th.Type.Body != 16 || th.Name != "Brand" {
		t.Errorf("radius %v body %v name %q", th.Radius, th.Type.Body, th.Name)
	}
	if th.Fg != Dark().Fg {
		t.Error("tokens not in the file should come from the base theme")
	}
	if _, err := ThemeFromTokens([]byte(`{"color":{"bg":{"$value":"{nope}"}}}`), nil); err == nil || !strings.Contains(err.Error(), "unknown alias") {
		t.Errorf("bad alias: err = %v", err)
	}
	if _, err := ThemeFromTokens([]byte(`{"a":{"$value":"{b}"},"b":{"$value":"{a}"},"color":{"bg":{"$value":"{a}"}}}`), nil); err == nil {
		t.Error("alias cycle not reported")
	}
}

// Generated themes must stay readable: that's the point of generating in OKLCH.
func TestPresetContrast(t *testing.T) {
	seeds := append([]Seed{}, PresetSeeds...)
	for h := 0.0; h < 360; h += 30 { // and any hue the gallery slider can produce
		for _, dark := range []bool{true, false} {
			seeds = append(seeds, Seed{Name: "hue", Dark: dark, Accent: OKLCH{0.65, 0.16, h}.RGB(), Neutral: OKLCH{0.5, 0.03, h}.RGB()})
		}
	}
	for _, s := range seeds {
		p := GeneratePalette(s)
		check := func(_ string, fg, bg colorPair, want float64) {
			if c := Contrast(fg.c, bg.c); c < want {
				t.Errorf("%s (dark=%v accent=%s): %s/%s contrast %.2f < %.1f", s.Name, s.Dark, Hex(s.Accent), fg.n, bg.n, c, want)
			}
		}
		checkAA(t, s.Name, s.Dark, Hex(s.Accent), p, check)
	}
	d := Dark().Palette // hand-picked, then made Legible
	checkAA(t, "Dark", true, Hex(d.Accent), d, func(_ string, fg, bg colorPair, want float64) {
		if c := Contrast(fg.c, bg.c); c < want {
			t.Errorf("Dark: %s/%s contrast %.2f < %.1f", fg.n, bg.n, c, want)
		}
	})
}

// checkAA asserts WCAG AA (4.5:1) for every text color on every surface it is
// drawn on, and the hierarchy Fg > Muted > Dim.
func checkAA(t *testing.T, _ string, _ bool, _ string, p Palette, check func(string, colorPair, colorPair, float64)) {
	t.Helper()
	for _, bg := range []colorPair{{"bg", p.Bg}, {"panel", p.Panel}, {"surface", p.Surface}, {"hover", p.Hover}} {
		check("text", colorPair{"fg", p.Fg}, bg, ContrastText)
		check("text", colorPair{"muted", p.Muted}, bg, ContrastMuted)
		check("text", colorPair{"dim", p.Dim}, bg, ContrastAA)
		check("accent", colorPair{"accent", p.Accent}, bg, ContrastAA)
		for _, c := range []colorPair{{"good", p.Good}, {"warn", p.Warn}, {"bad", p.Bad}} {
			check("status", c, bg, ContrastAA)
		}
	}
	check("accent", colorPair{"accent", p.Accent}, colorPair{"accent-bg", p.AccentBg}, ContrastAA)
	check("on-accent", colorPair{"on-accent", p.OnAccent}, colorPair{"accent", p.Accent}, ContrastAA)
	check("status", colorPair{"bad", p.Bad}, colorPair{"bad-bg", p.BadBg}, ContrastAA)
	check("status", colorPair{"good", p.Good}, colorPair{"good-bg", p.GoodBg}, ContrastAA)
	check("status", colorPair{"warn", p.Warn}, colorPair{"warn-bg", p.WarnBg}, ContrastAA)
	if !(Contrast(p.Fg, p.Surface) > Contrast(p.Muted, p.Surface) && Contrast(p.Muted, p.Surface) > Contrast(p.Dim, p.Surface)*1.2) {
		t.Errorf("text hierarchy collapsed: fg %s muted %s dim %s", Hex(p.Fg), Hex(p.Muted), Hex(p.Dim))
	}
}

type colorPair struct {
	n string
	c color.NRGBA
}

func TestOKLCHRoundTrip(t *testing.T) {
	for _, h := range []string{"#74ade8", "#1e2124", "#ffffff", "#000000", "#d07277"} {
		c, _ := ParseHex(h)
		if got := Hex(ToOKLCH(c).RGB()); got != h {
			t.Errorf("%s → %s", h, got)
		}
	}
}

func TestParseChord(t *testing.T) {
	cases := []struct {
		in   string
		name key.Name
		mods key.Modifiers
	}{
		{"Ctrl+K", "K", key.ModCtrl},
		{"shift+ctrl+p", "P", key.ModCtrl | key.ModShift},
		{"Shortcut+1", "1", key.ModShortcut},
		{"Alt+Up", key.NameUpArrow, key.ModAlt},
		{"F12", "F12", 0},
		{"Escape", key.NameEscape, 0},
		{"Ctrl+,", ",", key.ModCtrl},
	}
	for _, c := range cases {
		n, m, err := ParseChord(c.in)
		if err != nil || n != c.name || m != c.mods {
			t.Errorf("%q → %q %v %v, want %q %v", c.in, n, m, err, c.name, c.mods)
		}
	}
	for _, bad := range []string{"Hyper+K", "Ctrl+", "Ctrl+Nope"} {
		if _, _, err := ParseChord(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
	var km Keymap
	km.MustBind("Ctrl+K", "Palette", func() {})
	km.MustBind("ctrl+k", "Other", func() {}) // same chord: replaces
	if len(km.Bindings()) != 1 || km.Hint("Other") != "Ctrl+K" || km.Hint("Palette") != "" {
		t.Errorf("rebinding: %+v", km.Bindings())
	}
}

func TestTweenSettlesAndRequestsFrames(t *testing.T) {
	var ops op.Ops
	now := time.Unix(0, 0)
	gtx := layout.Context{Ops: &ops, Now: now}
	var tw Tween
	if v := tw.To(gtx, 0, 100*time.Millisecond); v != 0 {
		t.Fatalf("first value %v: should snap", v)
	}
	if v := tw.To(gtx, 1, 100*time.Millisecond); v != 0 {
		t.Fatalf("start of transition = %v", v)
	}
	gtx.Now = now.Add(50 * time.Millisecond)
	mid := tw.To(gtx, 1, 100*time.Millisecond)
	if mid <= 0.5 || mid >= 1 { // EaseOut is past halfway at half time
		t.Fatalf("midway = %v", mid)
	}
	if !tw.Running(gtx) {
		t.Fatal("tween should be running mid-way")
	}
	gtx.Now = now.Add(200 * time.Millisecond)
	if v := tw.To(gtx, 1, 100*time.Millisecond); v != 1 || tw.Running(gtx) {
		t.Fatalf("after duration: %v running=%v", v, tw.Running(gtx))
	}
	var zero Tween // Motion = 0: durations are 0, values jump
	zero.To(gtx, 0, 0)
	if v := zero.To(gtx, 1, 0); v != 1 || zero.Running(gtx) {
		t.Fatalf("zero duration: %v", v)
	}
}

func TestActionsAndOverrides(t *testing.T) {
	var a Actions
	runs := 0
	a.MustRegister(Action{ID: "save", Title: "Save file", Run: func() { runs++ }})
	a.MustRegister(Action{ID: "delete", Title: "Delete file", Run: func() { runs += 10 }, Enabled: func() bool { return false }})
	if err := a.Register(Action{ID: "save", Title: "Again", Run: func() {}}); err == nil {
		t.Fatal("duplicate action accepted")
	}
	if a.Run("delete") || runs != 0 {
		t.Fatal("disabled action ran")
	}
	if !a.Run("save") || runs != 1 {
		t.Fatal("save action did not run")
	}
	if got := a.Search("file"); len(got) != 1 || got[0].ID != "save" {
		t.Fatalf("search = %+v", got)
	}
	var km Keymap
	if err := km.BindActions(&a, map[string]string{"save": "Shortcut+S", "delete": "Delete"}, []byte(`{"save":"Shortcut+K","delete":""}`)); err != nil {
		t.Fatal(err)
	}
	if got := km.Hint("Save file"); got != "Ctrl+K" && got != "Cmd+K" {
		t.Fatalf("override = %q", got)
	}
	if len(km.Bindings()) != 1 {
		t.Fatalf("bindings = %+v", km.Bindings())
	}
	// A keymap from a newer version names an action this build lacks: the
	// rest must still bind, and the caller learns which ids were skipped.
	var km2 Keymap
	err := km2.BindActions(&a, nil, []byte(`{"unknown":"Ctrl+U","save":"Ctrl+J"}`))
	var unk *UnknownActionsError
	if !errors.As(err, &unk) || !slices.Equal(unk.IDs, []string{"unknown"}) {
		t.Fatalf("unknown override: err = %v", err)
	}
	if got := km2.Hint("Save file"); got != "Ctrl+J" {
		t.Fatalf("known override not bound alongside unknown one: %q", got)
	}
}

// A list whose ids churn (a log, a chat) must not keep state for every id it
// has ever seen.
func TestPoolSweepDropsUntouched(t *testing.T) {
	var p Pool[Clickable]
	for frame := range 10 * poolIdle {
		p.Get("live")
		p.Get(itoa(frame)) // a new id every frame, never seen again
		p.Sweep()
	}
	if n := p.Len(); n > 2*poolIdle+2 {
		t.Fatalf("pool holds %d entries after %d frames of churn", n, 10*poolIdle)
	}
	live := p.Get("live")
	for range 3 * poolIdle {
		p.Get("live")
		p.Sweep()
	}
	if p.Get("live") != live {
		t.Fatal("an id touched every frame was evicted")
	}
}

// Posting from the UI goroutine (inside a post or a frame) used to block
// forever once 64 posts were queued: the only reader was that goroutine.
func TestPostFromUIGoroutineNeverBlocks(t *testing.T) {
	a := NewApp("test", "test", 100, 100)
	ran := 0
	done := make(chan struct{})
	go func() {
		for range 1000 {
			a.Post(func() { ran++ })
		}
		a.Drain()
		a.Post(func() { a.Post(func() { ran++ }) }) // nested: runs next drain
		a.Drain()
		a.Drain()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Post blocked on the UI goroutine")
	}
	if ran != 1001 {
		t.Fatalf("ran %d posts, want 1001", ran)
	}
}

// Actions resolve from the focused element up through its KeyContexts: the
// nearest handler wins, and a binding outside any handler does nothing.
func TestDispatchNearestHandler(t *testing.T) {
	var w Window
	var inner, outer Clickable
	got := ""
	frame := func(gtx C) {
		OnAction(gtx, ActMoveDown, func() { got = "root" })
		KeyContext(gtx, "outer", func(gtx C) D {
			OnAction(gtx, ActMoveDown, func() { got = "outer" })
			outer.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(10, 10)} })
			return KeyContext(gtx, "inner", func(gtx C) D {
				OnAction(gtx, ActMoveDown, func() { got = "inner" })
				return inner.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(10, 10)} })
			})
		})
	}
	var r input.Router
	var ops op.Ops
	run := func() {
		ops.Reset()
		gtx := w.Begin(C{Ops: &ops, Source: r.Source(), Constraints: layout.Exact(image.Pt(100, 100))})
		frame(gtx)
		w.End(gtx)
		r.Frame(&ops)
	}
	for _, tc := range []struct {
		focus event.Tag
		want  string
	}{{inner.FocusTag(), "inner"}, {outer.FocusTag(), "outer"}, {nil, "root"}} {
		run()
		r.Source().Execute(key.FocusCmd{Tag: tc.focus})
		run()
		got = ""
		r.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
		run()
		if got != tc.want {
			t.Errorf("focus %v: handler %q, want %q", tc.focus, got, tc.want)
		}
	}
}

func TestPartEscapes(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{Part("lang", "opt", "3"), "lang/opt/3"},
		{Part("files", "src/main.go"), "files/src%2Fmain.go"},
		{Part("x", "100%"), "x/100%25"},
		{Part("row:42", "pay"), "row:42/pay"}, // ":" is free inside a segment
		{EscapeID("plain"), "plain"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}
