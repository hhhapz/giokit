//go:build js

package pad

import "syscall/js"

// The web source reads the Gamepad API. Browsers expose a pad only after
// its first button press, and report button changes only by polling, so
// Pads draws frames while a pad is connected (Continuous). Pads with the
// "standard" mapping (most, in Chrome and Safari; Firefox on some systems)
// match the standard layout index for index; others are read the same way,
// as the best guess.

func platformSource() Source { return &webSource{} }

type webSource struct {
	nav      js.Value
	ok       bool
	handlers []js.Func
}

func (s *webSource) Start(wake func()) error {
	s.nav = js.Global().Get("navigator")
	s.ok = s.nav.Truthy() && s.nav.Get("getGamepads").Truthy()
	if !s.ok {
		return nil
	}
	for _, ev := range []string{"gamepadconnected", "gamepaddisconnected"} {
		f := js.FuncOf(func(js.Value, []js.Value) any { wake(); return nil })
		s.handlers = append(s.handlers, f)
		js.Global().Call("addEventListener", ev, f)
	}
	return nil
}

func (s *webSource) Continuous() bool { return true }

// OwnFocus: browsers already hide gamepads from background pages, and the
// web window's Focused means a text input has focus, so Pads mustn't gate
// on it.
func (s *webSource) OwnFocus() bool { return true }

// standardButtons is the standard mapping's button order.
var standardButtons = [...]Button{
	South, East, West, North, LeftShoulder, RightShoulder, LeftTrigger, RightTrigger,
	Back, Start, LeftStick, RightStick, Up, Down, Left, Right, Guide,
}

func (s *webSource) Poll(dst []Pad) []Pad {
	if !s.ok {
		return dst
	}
	list := s.nav.Call("getGamepads")
	for i := range list.Length() {
		g := list.Index(i)
		if !g.Truthy() || !g.Get("connected").Truthy() {
			continue
		}
		id := g.Get("id").String()
		p := Pad{ID: g.Get("index").Int() + 1, Name: id, Family: webFamily(id)}
		buttons := g.Get("buttons")
		for j := range min(buttons.Length(), len(standardButtons)) {
			b := buttons.Index(j)
			if b.Get("pressed").Truthy() {
				p.Buttons = p.Buttons.With(standardButtons[j])
			}
			switch standardButtons[j] {
			case LeftTrigger:
				p.Axes[LeftTriggerAxis] = float32(b.Get("value").Float())
			case RightTrigger:
				p.Axes[RightTriggerAxis] = float32(b.Get("value").Float())
			}
		}
		axes := g.Get("axes")
		for j := range min(axes.Length(), 4) {
			p.Axes[Axis(j)] = steadyAxis((float32(axes.Index(j).Float())+1)/2, false)
		}
		dst = append(dst, p)
	}
	return dst
}

func (s *webSource) Close() error {
	for i, ev := range []string{"gamepadconnected", "gamepaddisconnected"} {
		js.Global().Call("removeEventListener", ev, s.handlers[i])
		s.handlers[i].Release()
	}
	s.handlers = nil
	return nil
}
