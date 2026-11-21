package pad

import "testing"

func TestWebFamily(t *testing.T) {
	for id, want := range map[string]Family{
		"Xbox 360 Controller (XInput STANDARD GAMEPAD)":                               Xbox,
		"DualSense Wireless Controller (STANDARD GAMEPAD Vendor: 054c Product: 0ce6)": PlayStation,
		"054c-0ce6-DualSense Wireless Controller":                                     PlayStation,
		"057e-2009-Pro Controller":                                                    Nintendo,
		"Unknown Gamepad (Vendor: 1234 Product: 5678)":                                Generic,
	} {
		if got := webFamily(id); got != want {
			t.Errorf("%q: family %d, want %d", id, got, want)
		}
	}
}

func TestFamilyOf(t *testing.T) {
	for _, c := range []struct {
		vendor, product uint16
		name            string
		want            Family
	}{
		{0x045e, 0x0b12, "Microsoft Xbox Controller", Xbox},
		{0x054c, 0x0ce6, "Sony Interactive Entertainment DualSense Wireless Controller", PlayStation},
		{0x057e, 0x2009, "Nintendo Switch Pro Controller", Nintendo},
		{0x28de, 0x11ff, "Microsoft X-Box 360 pad 0", Xbox},
		{0x28de, 0x1205, "Steam Deck", Steam},
		{0x2dc8, 0x3106, "8BitDo Pro 2 Wired Controller", Generic},
		{0x24c6, 0x543a, "PowerA Xbox One wired controller", Xbox},
	} {
		if got := familyOf(c.vendor, c.product, c.name); got != c.want {
			t.Errorf("%q: family %d, want %d", c.name, got, c.want)
		}
	}
}

func TestXInputPad(t *testing.T) {
	p := xinputPad(1, xinputState{buttons: 0x1000 | 0x4000 | 0x0001, lt: 255, lx: 32767, ly: 32767})
	if !p.Buttons.Has(South) || !p.Buttons.Has(West) || !p.Buttons.Has(Up) || p.Buttons.Has(North) {
		t.Errorf("buttons %b", p.Buttons)
	}
	if p.Axes[LeftX] < 0.99 || p.Axes[LeftY] > -0.99 || p.Axes[LeftTriggerAxis] < 0.99 {
		t.Errorf("axes %v: want right, up, left trigger", p.Axes)
	}
}

func TestSteadyAxis(t *testing.T) {
	for _, c := range []struct {
		raw     float32
		trigger bool
		want    float32
	}{
		{0.5, false, 0}, {0.53, false, 0}, {1, false, 1}, {0, false, -1}, {0.75, false, 0.4375},
		{0, true, 0}, {0.03, true, 0}, {1, true, 1},
	} {
		if got := steadyAxis(c.raw, c.trigger); got != c.want {
			t.Errorf("steadyAxis(%v, %v) = %v, want %v", c.raw, c.trigger, got, c.want)
		}
	}
	// Jitter around a held position reads the same.
	if a, b := steadyAxis(0.8, false), steadyAxis(0.8003, false); a != b {
		t.Errorf("jitter changed the value: %v, %v", a, b)
	}
}
