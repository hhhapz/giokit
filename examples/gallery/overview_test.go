package main

import (
	"fmt"
	"testing"

	"gioui.org/io/key"

	"github.com/hhhapz/giokit/kit"
)

// The Overview's theme bar restyles the whole app: a preset from the select,
// a knob shared with the Theme page, and Surprise me.
func TestOverviewTheme(t *testing.T) {
	g, h := newHarness(t)
	h.ClickID("quick:preset/field")
	h.ClickID("quick:preset/Midnight") // Midnight
	if g.th.Name != "Midnight" {
		t.Fatalf("preset select: theme %q", g.th.Name)
	}
	h.ClickID(kit.Part("knob:corners", "Pill"))
	if g.th.Radius != 999 || g.theme.knobs.corners.sel != 4 {
		t.Fatalf("corners from the overview: radius %v", g.th.Radius)
	}
	before := g.th.Name + g.th.Fonts + g.theme.knobs.corners.selected().label
	h.ClickID("quick:surprise")
	if g.th.Name+g.th.Fonts+g.theme.knobs.corners.selected().label == before {
		t.Fatal("Surprise me changed nothing")
	}
	h.Theme = g.th
	h.ExpectClean()
	scrollTo(t, h, g, "explore:menus")
	h.ClickID("explore:menus")
	if g.page != "menus" {
		t.Fatalf("explore link: page %q", g.page)
	}
}

// Lights out by mouse: replaying the presses that made the puzzle solves it.
func TestLightsOutMouse(t *testing.T) {
	g, h := newHarness(t)
	if g.overview.game.lit() == 0 {
		t.Fatal("a new puzzle has no lights on")
	}
	for _, i := range g.overview.game.solution {
		h.ClickID(kit.Part("lights", cellID(i)))
	}
	if !g.overview.game.solved || g.overview.game.moves != len(g.overview.game.solution) {
		t.Fatalf("solved %v after %d moves (lit %d)", g.overview.game.solved, g.overview.game.moves, g.overview.game.lit())
	}
	h.ExpectText(fmt.Sprintf("Solved in %d moves!", len(g.overview.game.solution)))
	h.ClickID("game:new")
	if g.overview.game.solved || g.overview.game.lit() == 0 {
		t.Fatal("Play again did not start a new puzzle")
	}
	h.ExpectClean()
}

// Lights out by keyboard: the board is one Tab stop; arrows move the active
// light and Enter presses it.
func TestLightsOutKeyboard(t *testing.T) {
	g, h := newHarness(t)
	for range 60 {
		if h.FocusedID() == kit.Part("lights", "3-3") {
			break
		}
		h.Tab(false)
	}
	if got := h.FocusedID(); got != kit.Part("lights", "3-3") {
		t.Fatalf("Tab never reached the board's middle light: %q", got)
	}
	h.Key(key.NameUpArrow, 0)
	h.Key(key.NameLeftArrow, 0)
	if got := h.FocusedID(); got != kit.Part("lights", "2-2") {
		t.Fatalf("arrows moved to %q, want lights/2-2", got)
	}
	was := g.overview.game.on
	h.Key(key.NameReturn, 0)
	if g.overview.game.moves != 1 || g.overview.game.on == was {
		t.Fatalf("Enter did not press the light: moves %d", g.overview.game.moves)
	}
	// 2-2 and its four neighbours flipped, nothing else.
	flipped := 0
	for i := range was {
		if was[i] != g.overview.game.on[i] {
			flipped++
		}
	}
	if flipped != 5 {
		t.Fatalf("a press flipped %d lights, want 5", flipped)
	}
	h.Tab(false)
	if got := h.FocusedID(); len(got) > 7 && got[:7] == "lights/" {
		t.Fatalf("Tab stayed in the board: %q", got)
	}
	h.ExpectTabReachable()
}

// Keys arriving in one frame: two arrows move twice, and Enter presses the
// light the arrow just moved to.
func TestLightsOutKeysInOneFrame(t *testing.T) {
	g, h := newHarness(t)
	lo := &g.overview.game
	for range 60 {
		if h.FocusedID() == kit.Part("lights", "3-3") {
			break
		}
		h.Tab(false)
	}
	h.Keys(key.NameUpArrow, key.NameUpArrow)
	if lo.group.Active != "1-3" {
		t.Fatalf("two Ups in one frame moved to %q, want 1-3", lo.group.Active)
	}
	was := lo.on
	h.Keys(key.NameRightArrow, key.NameReturn)
	want := was
	for _, i := range []int{3, 2, 4, 8} { // 1-4 and its neighbours
		want[i] = !want[i]
	}
	if lo.on != want {
		t.Fatalf("Right, Enter in one frame pressed the wrong light (active %q)", lo.group.Active)
	}
}
