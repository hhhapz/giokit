package pad

import (
	"github.com/hhhapz/giokit/gio/io/key"
	"github.com/hhhapz/giokit/gio/layout"
)

// Map binds buttons to functions, the gamepad counterpart of kit.Keymap.
// Bind once, then Run it every frame where it applies. Maps run in order:
// an event one Map handled is consumed, so run the most specific first (an
// open menu's before the game's). The zero value is an empty map. A Map
// belongs to the UI goroutine, and its functions run there.
type Map struct {
	binds []Binding
}

// Binding is one button → function.
type Binding struct {
	Button Button
	Title  string // human description, for help screens
	Run    func()
	// Repeat runs it again while the button is held, like a held key:
	// for moving a cursor or focus.
	Repeat  bool
	enabled func() bool
}

// Bind runs f when b is pressed.
func (m *Map) Bind(b Button, title string, f func()) {
	m.binds = append(m.binds, Binding{Button: b, Title: title, Run: f})
}

// BindRepeat runs f when b is pressed and again while it is held.
func (m *Map) BindRepeat(b Button, title string, f func()) {
	m.binds = append(m.binds, Binding{Button: b, Title: title, Run: f, Repeat: true})
}

// BindIf is Bind for a binding that only applies while enabled reports
// true; otherwise the press passes on to later Maps, as if unbound.
func (m *Map) BindIf(b Button, title string, enabled func() bool, f func()) {
	m.binds = append(m.binds, Binding{Button: b, Title: title, Run: f, enabled: enabled})
}

// Bindings returns every binding, in bind order, for a help screen.
func (m *Map) Bindings() []Binding { return append([]Binding(nil), m.binds...) }

// Run handles this frame's unconsumed presses (and repeats, for Repeat
// bindings), running the first live binding for each and consuming it.
func (m *Map) Run(gtx layout.Context, p *Pads) {
	for i, e := range p.events {
		if p.used[i] || e.Kind == Release {
			continue
		}
		for _, b := range m.binds {
			if b.Button != e.Button || e.Kind == Repeat && !b.Repeat {
				continue
			}
			if b.enabled != nil && !b.enabled() {
				continue
			}
			p.used[i] = true
			b.Run()
			break
		}
	}
}

// Nav drives keyboard focus from the pad, so any focusable UI (menus,
// dialogs, forms) works without a keyboard: the d-pad (and left stick)
// move focus by geometry, South clicks the focused control, and East runs
// back, if not nil (close the menu or dialog). It consumes those events,
// so run it before Maps that use the same buttons, and only while that UI
// is up. A control "clicks" when the focused tag has a Click method, as
// kit.Clickable does.
func Nav(gtx layout.Context, p *Pads, back func()) {
	for i, e := range p.events {
		if p.used[i] || e.Kind == Release {
			continue
		}
		dir := key.FocusDirection(-1)
		switch e.Button {
		case Up:
			dir = key.FocusUp
		case Down:
			dir = key.FocusDown
		case Left:
			dir = key.FocusLeft
		case Right:
			dir = key.FocusRight
		case South:
			if e.Kind == Press {
				if c, ok := gtx.Source.FocusedTag().(interface{ Click() }); ok {
					c.Click()
				}
			}
			p.used[i] = true
			continue
		case East:
			if e.Kind == Press && back != nil {
				back()
			}
			p.used[i] = true
			continue
		default:
			continue
		}
		p.used[i] = true
		if gtx.Source.FocusedTag() == nil {
			dir = key.FocusForward // nothing focused yet: start at the first control
		}
		gtx.Execute(key.MoveFocusCmd{Dir: dir})
	}
}

// Label is how a pad of family f prints button b: "A" (Xbox), "✕"
// (PlayStation), "B" (Nintendo) for South. Generic and Steam pads use the
// Xbox names.
func Label(f Family, b Button) string {
	if int(b) >= int(ButtonCount) {
		return ""
	}
	switch f {
	case PlayStation:
		return psLabels[b]
	case Nintendo:
		return nintendoLabels[b]
	}
	return xboxLabels[b]
}

// ASCIILabel is Label without symbols, for fonts that lack ✕○□△ and ←: the
// PlayStation face buttons become "CROSS", "CIRCLE", "SQUARE", "TRIANGLE".
func ASCIILabel(f Family, b Button) string {
	if int(b) >= int(ButtonCount) {
		return ""
	}
	if f == PlayStation && b <= North {
		return [...]string{"CROSS", "CIRCLE", "SQUARE", "TRIANGLE"}[b]
	}
	if b >= Up && b <= Right {
		return [...]string{"UP", "DOWN", "LEFT", "RIGHT"}[b-Up]
	}
	return Label(f, b)
}

var (
	xboxLabels = [ButtonCount]string{
		"A", "B", "X", "Y", "LB", "RB", "LT", "RT", "VIEW", "MENU", "LS", "RS", "↑", "↓", "←", "→", "XBOX",
	}
	psLabels = [ButtonCount]string{
		"✕", "○", "□", "△", "L1", "R1", "L2", "R2", "CREATE", "OPTIONS", "L3", "R3", "↑", "↓", "←", "→", "PS",
	}
	nintendoLabels = [ButtonCount]string{
		"B", "A", "Y", "X", "L", "R", "ZL", "ZR", "−", "+", "LS", "RS", "↑", "↓", "←", "→", "HOME",
	}
)
