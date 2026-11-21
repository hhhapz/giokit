// SPDX-License-Identifier: Unlicense OR MIT

package key

// MoveFocusCmd moves the keyboard focus in direction Dir, as Tab and
// Shift+Tab do (FocusForward, FocusBackward) or by geometry (FocusUp, ...).
// It lets input that isn't a keyboard, such as a gamepad's d-pad, drive
// focus.
type MoveFocusCmd struct {
	Dir FocusDirection
}

func (MoveFocusCmd) ImplementsCommand() {}
