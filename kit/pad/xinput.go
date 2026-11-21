package pad

// xinputState is XINPUT_STATE, kept platform-neutral so the conversion
// is tested everywhere.
type xinputState struct {
	packet  uint32
	buttons uint16
	lt, rt  uint8
	lx, ly  int16
	rx, ry  int16
}

// xinputPad converts an XINPUT_STATE.
func xinputPad(id int, st xinputState) Pad {
	p := Pad{ID: id, Name: "XInput Controller", Family: Xbox}
	for bit, b := range map[uint16]Button{
		0x0001: Up, 0x0002: Down, 0x0004: Left, 0x0008: Right,
		0x0010: Start, 0x0020: Back, 0x0040: LeftStick, 0x0080: RightStick,
		0x0100: LeftShoulder, 0x0200: RightShoulder,
		0x1000: South, 0x2000: East, 0x4000: West, 0x8000: North,
	} {
		if st.buttons&bit != 0 {
			p.Buttons = p.Buttons.With(b)
		}
	}
	stick := func(v int16) float32 { return steadyAxis((float32(v)+32768)/65535, false) }
	p.Axes[LeftX], p.Axes[LeftY] = stick(st.lx), -stick(st.ly) // XInput's Y points up
	p.Axes[RightX], p.Axes[RightY] = stick(st.rx), -stick(st.ry)
	p.Axes[LeftTriggerAxis] = steadyAxis(float32(st.lt)/255, true)
	p.Axes[RightTriggerAxis] = steadyAxis(float32(st.rt)/255, true)
	return p
}
