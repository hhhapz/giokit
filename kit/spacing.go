package kit

import "gioui.org/unit"

// The spacing scale, GPUI/Tailwind style: SpaceN is N×4dp, with half steps
// spelled 0p5, 1p5, 2p5, 3p5. Use these instead of hand-picked dp values; pass
// them through Theme.Space (or kit.Flex, which does it) so Density applies.
const (
	Space0p5 unit.Dp = 2
	Space1   unit.Dp = 4
	Space1p5 unit.Dp = 6
	Space2   unit.Dp = 8
	Space2p5 unit.Dp = 10
	Space3   unit.Dp = 12
	Space3p5 unit.Dp = 14
	Space4   unit.Dp = 16
	Space5   unit.Dp = 20
	Space6   unit.Dp = 24
	Space8   unit.Dp = 32
	Space10  unit.Dp = 40
	Space12  unit.Dp = 48
)
