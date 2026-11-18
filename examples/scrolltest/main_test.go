package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hhhapz/giokit/kittest"
)

func newHarness(t *testing.T) (*bench, *kittest.Harness) {
	b := newBench()
	h := kittest.NewT(t, 1100, 760, b.Frame)
	h.Theme = b.th
	return b, h
}

// Even scrolling over every component kind flags nothing, in both modes.
func TestEvenScrollNoSkips(t *testing.T) {
	b, h := newHarness(t)
	h.ExpectClean()
	for _, ph := range []bool{false, true} {
		b.placeholder = ph
		for range 120 {
			h.ScrollID("comp", 6)
			h.ScrollID("plain", 6)
		}
		h.ExpectClean()
		for _, l := range []*lane{&b.plain, &b.comp} {
			if l.skips != 0 || l.list.Jumps() != 0 || l.abs == 0 {
				t.Errorf("placeholder=%v %s: skips %d, jumps %d, offset %d", ph, l.id, l.skips, l.list.Jumps(), l.abs)
			}
		}
		h.Chord("R")
	}
}

// The detector catches the upstream pattern: small steps, then one ~10× step.
func TestSkipDetected(t *testing.T) {
	b, h := newHarness(t)
	for range 6 {
		h.ScrollID("comp", 2)
	}
	// Disable motion for the injected bad frame so the list's glide does not
	// smooth away the jump this detector is supposed to catch.
	b.th.Motion = 0
	h.ScrollID("comp", 20)
	b.th.Motion = 1
	if b.comp.skips != 1 {
		t.Fatalf("skips = %d, want 1 (deltas %v)", b.comp.skips, b.comp.deltas)
	}
	for range 30 {
		h.ScrollID("plain", 5)
	}
	h.ExpectClean()
	if dir := os.Getenv("KITTEST_SHOTS"); dir != "" {
		if err := h.Save(filepath.Join(dir, "scrolltest.png")); err != nil {
			t.Error(err)
		}
	}
}

// Hover and click a component row: rows stay interactive.
func TestComponentsInteractive(t *testing.T) {
	b, h := newHarness(t)
	h.Hover("2:sw")
	h.ClickID("2:sw")
	if !b.on["2:sw"] {
		t.Fatal("switch didn't toggle")
	}
	label, found := "", false
	var walk func([]kittest.A11yNode)
	walk = func(nodes []kittest.A11yNode) {
		for _, n := range nodes {
			if n.ID == "1:icon" {
				label, found = n.Label, true
			}
			walk(n.Children)
		}
	}
	walk(h.A11yTree())
	if !found || label != "Brightness" {
		t.Errorf("1:icon: label %q (found %v), want %q", label, found, "Brightness")
	}
	h.ExpectClean()
}
