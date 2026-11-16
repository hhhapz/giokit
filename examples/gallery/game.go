package main

import (
	"fmt"
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
)

// Lights Out: a 5×5 grid of lights. Pressing one toggles it and its four
// neighbours; turn every light off. It is built only from public kit pieces:
// Clickables in a FocusGroup (one Tab stop, arrows move, Enter presses),
// a KeyContext with OnAction, Tweens for the glow, and Surface-free drawing.
const lightsN = 5

type lightsOut struct {
	on       [lightsN * lightsN]bool
	cells    [lightsN * lightsN]kit.Clickable
	glow     [lightsN * lightsN]kit.Tween
	group    kit.FocusGroup
	newGame  kit.Clickable
	moves    int
	solved   bool
	seed     uint32
	solution []int // the presses that generated this puzzle (tests replay them)
}

func cellID(i int) string { return fmt.Sprintf("%d-%d", i/lightsN+1, i%lightsN+1) }

// press toggles cell i and its orthogonal neighbours.
func (lo *lightsOut) press(i int) {
	r, c := i/lightsN, i%lightsN
	for _, d := range [][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		rr, cc := r+d[0], c+d[1]
		if rr >= 0 && rr < lightsN && cc >= 0 && cc < lightsN {
			lo.on[rr*lightsN+cc] = !lo.on[rr*lightsN+cc]
		}
	}
}

// shuffle starts a new puzzle by pressing random cells from all-off, so it
// is always solvable. A small LCG keeps it deterministic for tests.
func (lo *lightsOut) shuffle() {
	lo.on = [lightsN * lightsN]bool{}
	lo.moves, lo.solved, lo.solution = 0, false, nil
	for len(lo.solution) < 6 || lo.allOff() {
		lo.seed = lo.seed*1664525 + 1013904223
		i := int(lo.seed>>16) % len(lo.on)
		lo.press(i)
		lo.solution = append(lo.solution, i)
	}
	lo.group.Active = cellID(lightsN * lightsN / 2) // keyboard starts in the middle
}

func (lo *lightsOut) allOff() bool {
	for _, v := range lo.on {
		if v {
			return false
		}
	}
	return true
}

func (lo *lightsOut) lit() int {
	n := 0
	for _, v := range lo.on {
		if v {
			n++
		}
	}
	return n
}

func (lo *lightsOut) play(i int) {
	if lo.solved {
		return
	}
	lo.press(i)
	lo.moves++
	lo.solved = lo.allOff()
}

func (g *gallery) handleGame(gtx C) {
	lo := &g.game
	if lo.newGame.Clicked(gtx) {
		lo.shuffle()
	}
	for i := range lo.cells {
		lo.group.Join(&lo.cells[i], cellID(i) == lo.group.Active)
		if lo.cells[i].Clicked(gtx) {
			lo.group.Active = cellID(i)
			lo.play(i)
		}
	}
}

func (g *gallery) demoGame(gtx C) D {
	th, lo := g.th, &g.game
	status := fmt.Sprintf("%d lit · %d moves", lo.lit(), lo.moves)
	if lo.solved {
		status = fmt.Sprintf("Solved in %d moves!", lo.moves)
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D { return g.lightsGrid(gtx) }),
				kit.Rigid(kit.Gap(20, 0)),
				kit.Flexed(1, func(gtx C) D {
					return kit.Col(gtx,
						kit.Rigid(func(gtx C) D {
							c := th.Fg
							if lo.solved {
								c = th.Good
							}
							return th.Text(gtx, status, th.Type.Text, c, kit.SemiBold)
						}),
						kit.Rigid(kit.Gap(0, 10)),
						kit.Rigid(func(gtx C) D {
							label := "New puzzle"
							if lo.solved {
								label = "Play again"
							}
							return th.Button(gtx, &lo.newGame, "game:new", label, kit.Primary)
						}),
					)
				}),
			)
		}),
	)
}

// lightsGrid is the board: one focus group with grid arrow keys.
func (g *gallery) lightsGrid(gtx C) D {
	lo := &g.game
	cur := 0
	for i := range lo.cells {
		if cellID(i) == lo.group.Active {
			cur = i
		}
	}
	move := func(dr, dc int) func() {
		return func() {
			r := min(max(cur/lightsN+dr, 0), lightsN-1)
			c := min(max(cur%lightsN+dc, 0), lightsN-1)
			lo.group.Active = cellID(r*lightsN + c)
		}
	}
	return kit.KeyContext(gtx, "lights", func(gtx C) D {
		kit.OnAction(gtx, kit.ActMoveUp, move(-1, 0))
		kit.OnAction(gtx, kit.ActMoveDown, move(1, 0))
		kit.OnAction(gtx, kit.ActCollapse, move(0, -1)) // Left
		kit.OnAction(gtx, kit.ActExpand, move(0, 1))    // Right
		kit.OnAction(gtx, kit.ActFirst, func() { lo.group.Active = cellID(0) })
		kit.OnAction(gtx, kit.ActLast, func() { lo.group.Active = cellID(len(lo.cells) - 1) })
		kit.OnAction(gtx, kit.ActConfirm, func() { lo.play(cur) })
		return lo.group.Layout(gtx, "lights", func(gtx C) D { return kit.TagArea(gtx, "lights", g.lightsCells) })
	})
}

func (g *gallery) lightsCells(gtx C) D {
	th := g.th
	var rows []layout.FlexChild
	for r := range lightsN {
		if r > 0 {
			rows = append(rows, kit.Rigid(kit.Gap(0, 6)))
		}
		rows = append(rows, kit.Rigid(func(gtx C) D {
			var cells []layout.FlexChild
			for c := range lightsN {
				if c > 0 {
					cells = append(cells, kit.Rigid(kit.Gap(6, 0)))
				}
				i := r*lightsN + c
				cells = append(cells, kit.Rigid(func(gtx C) D { return g.lightCell(gtx, th, i) }))
			}
			return kit.Row(gtx, cells...)
		}))
	}
	return kit.Col(gtx, rows...)
}

func (g *gallery) lightCell(gtx C, th *kit.Theme, i int) D {
	lo := &g.game
	return lo.cells[i].Press(gtx, kit.Part("lights", cellID(i)), func(gtx C, st kit.State) D {
		semantic.CheckBox.Add(gtx.Ops)
		r, c := i/lightsN+1, i%lightsN+1
		semantic.LabelOp(fmt.Sprintf("Light, row %d column %d", r, c)).Add(gtx.Ops)
		semantic.SelectedOp(lo.on[i]).Add(gtx.Ops)
		target := float32(0)
		if lo.on[i] {
			target = 1
		}
		p := lo.glow[i].To(gtx, target, th.Dur(kit.DurNormal))
		sz := image.Pt(gtx.Dp(40), gtx.Dp(40))
		rad := min(gtx.Dp(th.RadiusSm()), sz.X/2)
		rect := image.Rectangle{Max: sz}
		off := kit.LerpColor(th.Surface, th.Hover, st.Hover)
		paint.FillShape(gtx.Ops, kit.LerpColor(off, th.Accent, p), clip.UniformRRect(rect, rad).Op(gtx.Ops))
		if p < 1 { // an unlit tile is outlined like a checkbox: visible at 3:1
			edge := kit.LerpColor(th.Dim, th.Muted, st.Hover)
			kit.Border(gtx, rect, rad, max(1, gtx.Dp(max(th.Stroke, 1.5))), kit.WithAlpha(edge, uint8(255*(1-p))))
		}
		if st.Focused {
			// Inside the tile (a Clickable clips to its area), thicker than
			// the outline, and in a colour that differs from the fill.
			ring := th.Focus
			if p > 0.5 {
				ring = th.OnAccent // an accent ring vanishes on a lit tile
			}
			kit.Border(gtx, rect, rad, max(2, gtx.Dp(th.Ring*1.5)), ring)
		}
		return D{Size: sz}
	})
}
