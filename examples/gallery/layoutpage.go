package main

import (
	"fmt"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kit/x"
)

type layoutState struct {
	outer, inner ui.SplitState
	files        kit.List
	fileClicks   kit.Pool[kit.Clickable]
	fileGroup    kit.FocusGroup
	openFile     string
	termView     x.LogViewState
}

var fileNames = []string{"main.go", "controls.go", "logs.go", "layoutpage.go", "themepage.go", "gallery_test.go", "go.mod", "README.md"}

// panelTitle is a small header strip above a pane.
func (g *gallery) panelTitle(gtx C, title string) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Fill(gtx, th.Panel, kit.Stretch(kit.Pad(6, 10, func(gtx C) D { return th.Caption(gtx, title) })))
		}),
		kit.Rigid(kit.HLine(th.Divider)),
	)
}

func (g *gallery) initPanes() {
	g.panes.outer.Ratio, g.panes.inner.Ratio = 0.25, 0.62
	g.panes.openFile = fileNames[0]
	g.panes.termView.NoTime = true
}

func (g *gallery) handlePanes(gtx C) {
	for _, f := range fileNames {
		c := g.panes.fileClicks.Get(f)
		g.panes.fileGroup.Join(c, f == g.panes.openFile)
		if c.Clicked(gtx) {
			g.panes.openFile = f
		}
	}
}

func (g *gallery) layoutPage(gtx C) D {
	th := g.th
	files := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, "Files") }),
			kit.Flexed(1, func(gtx C) D {
				return kit.Pad(6, 6, func(gtx C) D {
					nav := keyNav{ID: "files", IDs: fileNames, Current: &g.panes.openFile, Reveal: g.panes.files.ScrollIntoView}
					return nav.Layout(gtx, &g.panes.fileGroup, func(gtx C) D {
						return th.List(gtx, &g.panes.files, "files", len(fileNames), func(gtx C, i int) D {
							f := fileNames[i]
							return ui.NavItem{ID: "file:" + f, Label: f, Active: f == g.panes.openFile}.Layout(gtx, th, g.panes.fileClicks.Get(f))
						})
					})
				})(gtx)
			}),
		)
	}
	editor := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, g.panes.openFile) }),
			kit.Flexed(1, func(gtx C) D {
				return kit.Pad(12, 14, func(gtx C) D {
					return kit.Col(gtx,
						kit.Rigid(func(gtx C) D { return th.Code(gtx, "// "+g.panes.openFile, th.Type.Body, th.Dim) }),
						kit.Rigid(func(gtx C) D { return th.Code(gtx, "package main", th.Type.Body, th.Accent) }),
						kit.Rigid(kit.Gap(0, 8)),
						kit.Rigid(func(gtx C) D {
							return th.Paragraph(gtx, fmt.Sprintf("Drag the dividers, or Tab to one and use the arrow keys. Click a file, then Up/Down to move. Split ratios: outer %.2f, inner %.2f.", g.panes.outer.Ratio, g.panes.inner.Ratio), th.Muted)
						}),
					)
				})(gtx)
			}),
		)
	}
	terminal := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, "Output") }),
			kit.Flexed(1, func(gtx C) D {
				return x.LogView{ID: "output", Buffer: &g.logs.buf}.Layout(gtx, th, &g.panes.termView)
			}),
		)
	}
	return g.pageFrame(gtx, "Layout", nil, func(gtx C) D {
		return layout.Inset{Left: 24, Right: 24, Bottom: 20}.Layout(gtx, func(gtx C) D {
			return kit.Surface{Border: th.Border, Radius: th.Radius}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				return ui.Split{ID: "split:outer", Min: 120, A: files, B: func(gtx C) D {
					return ui.Split{ID: "split:inner", Axis: layout.Vertical, Min: 60, A: editor, B: terminal}.Layout(gtx, th, &g.panes.inner)
				}}.Layout(gtx, th, &g.panes.outer)
			})
		})
	})
}
