// Package ui holds giokit's components, built only from package kit's
// exported API: anything here could be written outside giokit.
//
// A component is a props struct laid out with a theme and its state:
//
//	ui.Combobox{ID: "city", Options: cities}.Layout(gtx, th, &v.city)
//
// Props are values, set fresh each frame (the value is controlled: it comes
// in through props). The state struct holds interaction state only. Events
// are read in the handle pass, before layout, with the state's Update.
package ui

import "github.com/hhhapz/giokit/kit"

type (
	C = kit.C
	D = kit.D
)
