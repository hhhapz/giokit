package main

import (
	"fmt"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
)

type layoutState struct {
	outer, inner kit.Split
	files        kit.List
	fileClicks   kit.Pool[kit.Clickable]
	fileGroup    kit.FocusGroup
	openFile     string
	termView     kit.LogView
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

func (g *gallery) layoutPage(gtx C) D {
	th := g.th
	if g.outer.Ratio == 0 {
		g.outer.Ratio, g.outer.Min = 0.25, 120
		g.inner.Axis, g.inner.Ratio, g.inner.Min = layout.Vertical, 0.62, 60
		g.openFile = fileNames[0]
		g.termView.NoTime = true
	}
	for _, f := range fileNames {
		c := g.fileClicks.Get(f)
		g.fileGroup.Join(c, f == g.openFile)
		if c.Clicked(gtx) {
			g.openFile = f
		}
	}
	files := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, "Files") }),
			kit.Flexed(1, func(gtx C) D {
				return kit.Pad(6, 6, func(gtx C) D {
					return navKeys(gtx, &g.fileGroup, g.files.ScrollIntoView, "files", fileNames, g.openFile, func(f string) { g.openFile = f }, func(gtx C) D {
						return th.List(gtx, &g.files, "files", len(fileNames), func(gtx C, i int) D {
							f := fileNames[i]
							return th.NavItem(gtx, g.fileClicks.Get(f), "file:"+f, f, "", th.Dim, f == g.openFile)
						})
					})
				})(gtx)
			}),
		)
	}
	editor := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, g.openFile) }),
			kit.Flexed(1, func(gtx C) D {
				return kit.Pad(12, 14, func(gtx C) D {
					return kit.Col(gtx,
						kit.Rigid(func(gtx C) D { return th.Code(gtx, "// "+g.openFile, th.Type.Body, th.Dim) }),
						kit.Rigid(func(gtx C) D { return th.Code(gtx, "package main", th.Type.Body, th.Accent) }),
						kit.Rigid(kit.Gap(0, 8)),
						kit.Rigid(func(gtx C) D {
							return th.Paragraph(gtx, fmt.Sprintf("Drag the dividers, or Tab to one and use the arrow keys. Click a file, then Up/Down to move. Split ratios: outer %.2f, inner %.2f.", g.outer.Ratio, g.inner.Ratio), th.Muted)
						}),
					)
				})(gtx)
			}),
		)
	}
	terminal := func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return g.panelTitle(gtx, "Output") }),
			kit.Flexed(1, func(gtx C) D { return th.LogView(gtx, &g.termView, &g.logBuf, "output") }),
		)
	}
	return g.pageFrame(gtx, "Layout", nil, func(gtx C) D {
		return layout.Inset{Left: 24, Right: 24, Bottom: 20}.Layout(gtx, func(gtx C) D {
			return kit.Surface{Border: th.Border, Radius: th.Radius}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				return th.Split(gtx, &g.outer, "split:outer", files, func(gtx C) D {
					return th.Split(gtx, &g.inner, "split:inner", editor, terminal)
				})
			})
		})
	})
}
