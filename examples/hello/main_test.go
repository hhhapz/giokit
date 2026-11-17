package main

import (
	"testing"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// The README's example works: real clicks and typing, then the audits.
func TestHello(t *testing.T) {
	v := &view{th: kit.Dark()}
	h := kittest.NewT(t, 480, 320, v.Frame)
	h.Theme = v.th
	h.ClickID("inc")
	h.ClickID("inc")
	h.ExpectText("Clicked 2 times")
	h.ClickID("name")
	h.Type("Ada")
	h.ClickID("greet")
	h.ExpectText("Hello, Ada")
	h.ExpectClean()
	h.ExpectTabReachable()
}
