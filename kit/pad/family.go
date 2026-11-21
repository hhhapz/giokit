package pad

import (
	"strconv"
	"strings"
)

// familyOf guesses the controller family from its USB ids and name.
func familyOf(vendor, product uint16, name string) Family {
	switch vendor {
	case 0x045e:
		return Xbox
	case 0x054c:
		return PlayStation
	case 0x057e:
		return Nintendo
	case 0x28de:
		if product == 0x11ff { // Steam Input's virtual Xbox 360 pad
			return Xbox
		}
		return Steam
	}
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "xbox") || strings.Contains(n, "x-box") || strings.Contains(n, "xinput"):
		return Xbox
	case strings.Contains(n, "playstation") || strings.Contains(n, "dualsense") || strings.Contains(n, "dualshock"):
		return PlayStation
	case strings.Contains(n, "nintendo") || strings.Contains(n, "pro controller") || strings.Contains(n, "joy-con"):
		return Nintendo
	case strings.Contains(n, "steam"):
		return Steam
	}
	return Generic
}

// webFamily reads the USB ids browsers put in a pad's id: Chrome writes
// "... (STANDARD GAMEPAD Vendor: 054c Product: 0ce6)", Firefox
// "054c-0ce6-DualSense Wireless Controller".
func webFamily(id string) Family {
	hex := func(s string) uint16 {
		v, _ := strconv.ParseUint(s, 16, 16)
		return uint16(v)
	}
	var vendor, product uint16
	if i := strings.Index(id, "Vendor: "); i >= 0 && len(id) >= i+12 {
		vendor = hex(id[i+8 : i+12])
		if j := strings.Index(id, "Product: "); j >= 0 && len(id) >= j+13 {
			product = hex(id[j+9 : j+13])
		}
	} else if len(id) > 10 && id[4] == '-' && id[9] == '-' {
		vendor, product = hex(id[:4]), hex(id[5:9])
	}
	return familyOf(vendor, product, id)
}

// Axis noise: a resting stick drifts by a few percent and jitters, which
// would wake the app many times a second for nothing. Sources pass raw
// values through steadyAxis: inside the dead zone a stick reads 0, and
// values snap to 1/64ths so tiny jitter changes nothing.
const (
	stickDeadZone = 0.12
	axisSteps     = 64
)

// steadyAxis converts a 0..1 raw reading to a stick's -1..1 (or a
// trigger's 0..1), with the dead zone and snapping applied.
func steadyAxis(v float32, trigger bool) float32 {
	if !trigger {
		v = v*2 - 1
	}
	dz := float32(stickDeadZone)
	if trigger {
		dz /= 2
	}
	switch {
	case v > -dz && v < dz:
		return 0
	case v > 0:
		v = (v - dz) / (1 - dz)
	default:
		v = (v + dz) / (1 - dz)
	}
	v = float32(int(v*axisSteps+copysign(0.5, v))) / axisSteps
	return min(max(v, -1), 1)
}

func copysign(x, sign float32) float32 {
	if sign < 0 {
		return -x
	}
	return x
}
