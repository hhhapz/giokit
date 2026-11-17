package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type controlsState struct {
	multi       ui.SelectableListState
	multiFocus  string
	multiChosen map[string]bool
	kinds       [5]kit.Clickable
	iconBtn     kit.Clickable
	iconTip     ui.TooltipState
	saveTip     ui.TooltipState
	wifi        kit.Clickable
	notify      kit.Clickable
	agree       kit.Clickable
	sizes       kit.Pool[kit.Clickable]
	name        widget.Editor
	search      widget.Editor
	notes       widget.Editor
	volume      slider
	tabs        ui.TabsState
	segs        ui.TabsState
	wifiOn      bool
	notifyOn    bool
	agreed      bool
	size        string
	tab         int
	seg         int

	// Disabled section: a form that the switch above it locks.
	lock      kit.Clickable
	locked    bool
	lockSave  kit.Clickable
	lockCheck kit.Clickable
	lockOn    bool
	lockField widget.Editor
	lockSlide slider
	lockTabs  ui.TabsState
	lockTab   int
}

var (
	kindNames = []string{"Secondary", "Primary", "Ghost", "Danger", "Solid"}
	kindVals  = []ui.ButtonStyle{ui.Secondary, ui.Primary, ui.Ghost, ui.Danger, ui.Solid}
	sizeNames = []string{"Small", "Medium", "Large"}
	tabNames  = []string{"General", "Editor", "Terminal", "Extensions"}
	segNames  = []string{"Day", "Week", "Month"}
)

func (g *gallery) initControls() {
	g.wifiOn, g.size = true, "Medium"
	g.name.SingleLine, g.name.Submit = true, true
	g.search.SingleLine = true
	g.volume.Value = 0.4
	g.locked, g.lockOn = true, true
	g.lockField.SingleLine = true
	g.lockField.SetText("read-only while locked")
	g.lockSlide.Value = 0.6
}

func (g *gallery) handleControls(gtx C) {
	for i := range g.kinds {
		if g.kinds[i].Clicked(gtx) {
			g.status = kindNames[i] + " clicked"
		}
	}
	if g.iconBtn.Clicked(gtx) {
		g.status = "Search clicked"
	}
	if g.wifi.Clicked(gtx) {
		g.wifiOn = !g.wifiOn
	}
	if g.notify.Clicked(gtx) {
		g.notifyOn = !g.notifyOn
	}
	if g.agree.Clicked(gtx) {
		g.agreed = !g.agreed
	}
	for _, s := range sizeNames {
		if g.sizes.Get(s).Clicked(gtx) {
			g.size = s
		}
	}
	if i, ok := g.tabs.Update(gtx); ok {
		g.tab = i
	}
	if i, ok := g.segs.Update(gtx); ok {
		g.seg = i
	}
	if g.lock.Clicked(gtx) {
		g.locked = !g.locked
	}
	if g.lockSave.Clicked(gtx) {
		g.status = "Saved"
	}
	if g.lockCheck.Clicked(gtx) {
		g.lockOn = !g.lockOn
	}
	if i, ok := g.lockTabs.Update(gtx); ok {
		g.lockTab = i
	}
	for {
		e, ok := g.name.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.SubmitEvent); ok {
			g.status = "Hello, " + g.name.Text()
			kit.Notify(gtx)
		}
	}
}

// section is a titled card on the controls page.
func (g *gallery) section(gtx C, title string, body kit.W) D {
	th := g.th
	return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D {
		return th.Card(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return kit.Col(gtx,
				kit.Rigid(func(gtx C) D { return th.Caption(gtx, title) }),
				kit.Rigid(kit.Gap(0, 12)),
				kit.Rigid(body),
			)
		})
	})
}

// sizedTheme scales the demo buttons with the Small/Medium/Large radios:
// a derived theme with a different type scale and density, nothing else.
func (g *gallery) sizedTheme() *kit.Theme {
	scale := map[string]float32{"Small": 0.85, "Medium": 1, "Large": 1.25}[g.size]
	if scale == 1 {
		return g.th
	}
	return g.th.Derive(func(t *kit.Theme) {
		t.Density *= scale
		t.Type.Body *= unitSp(scale)
	})
}

func (g *gallery) demoButtons(gtx C) D {
	th := g.sizedTheme()
	var btns []layout.FlexChild
	for i := range kindNames {
		btns = append(btns, kit.Rigid(func(gtx C) D {
			return ui.Button{ID: "btn:" + kindNames[i], Text: kindNames[i], Style: kindVals[i]}.Layout(gtx, th, &g.kinds[i])
		}))
	}
	btns = append(btns, kit.Rigid(func(gtx C) D {
		return ui.Tooltip{Text: "Search (tooltip)", Content: func(gtx C) D {
			return ui.IconButton{ID: "btn:search", Icon: kit.IconSearch, Label: "btn:search", Active: true}.Layout(gtx, th, &g.iconBtn)
		}}.Layout(gtx, th, &g.iconTip)
	}))
	sizes := []layout.FlexChild{kit.Rigid(func(gtx C) D { return g.th.Text(gtx, "Size", g.th.Type.Body, g.th.Muted, kit.Regular) })}
	for _, s := range sizeNames {
		sizes = append(sizes, kit.Rigid(func(gtx C) D {
			return ui.Radio{ID: "radio:" + s, Label: s, Selected: g.size == s}.Layout(gtx, g.th, g.sizes.Get(s))
		}))
	}
	return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3p5}.Layout(gtx, g.th,
		kit.Rigid(func(gtx C) D { return kit.Flex{Gap: kit.Space2, Align: layout.Middle}.Layout(gtx, th, btns...) }),
		kit.Rigid(func(gtx C) D { return kit.Flex{Gap: kit.Space5, Align: layout.Middle}.Layout(gtx, g.th, sizes...) }),
	)
}

func (g *gallery) demoToggles(gtx C) D {
	th := g.th
	return kit.Flex{Gap: kit.Space6, Align: layout.Middle}.Layout(gtx, th,
		kit.Rigid(func(gtx C) D {
			return ui.Switch{ID: "switch:wifi", Label: "Wi-Fi", On: g.wifiOn}.Layout(gtx, th, &g.wifi)
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Switch{ID: "switch:notify", Label: "Notifications", On: g.notifyOn}.Layout(gtx, th, &g.notify)
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Checkbox{ID: "check:agree", Label: "I agree", Checked: g.agreed}.Layout(gtx, th, &g.agree)
		}),
	)
}

func (g *gallery) demoSlider(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(func(gtx C) D { return th.Text(gtx, "Volume", th.Type.Body, th.Muted, kit.Regular) }),
		kit.Rigid(kit.Gap(12, 0)),
		kit.Flexed(1, func(gtx C) D { return g.volume.layout(gtx, th, "slider:volume", "Volume") }),
		kit.Rigid(kit.Gap(12, 0)),
		kit.Rigid(kit.Fixed(40, func(gtx C) D {
			return th.Text(gtx, fmt.Sprintf("%d%%", int(g.volume.Value*100+0.5)), th.Type.Body, th.Fg, kit.Medium)
		})),
	)
}

func (g *gallery) demoTextFields(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				kit.Flexed(1, func(gtx C) D {
					return ui.TextField{ID: "field:name", Placeholder: "Your name, then Enter"}.Layout(gtx, th, &g.name)
				}),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, func(gtx C) D {
					return ui.TextField{ID: "field:search", Placeholder: "Search", Icon: kit.IconSearch}.Layout(gtx, th, &g.search)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return ui.TextArea{ID: "field:notes", Placeholder: "Notes (Enter for a new line)", Lines: 3}.Layout(gtx, th, &g.notes)
		}),
	)
}

func (g *gallery) demoTabs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return ui.Tabs{ID: "tab", Labels: tabNames, Selected: g.tab}.Layout(gtx, th, &g.tabs) }),
		kit.Rigid(kit.HLine(th.Divider)),
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 14}.Layout(gtx, func(gtx C) D {
				return th.Text(gtx, tabNames[g.tab]+" settings would go here.", th.Type.Body, th.Muted, kit.Regular)
			})
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Tabs{ID: "seg", Labels: segNames, Selected: g.seg, Style: ui.TabsSegmented}.Layout(gtx, th, &g.segs)
		}),
	)
}

func (g *gallery) demoDisabled(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Switch{ID: "switch:lock", Label: "Lock the form", On: g.locked}.Layout(gtx, th, &g.lock)
		}),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			return th.Disable(gtx, g.locked, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Flexed(1, func(gtx C) D {
								return ui.TextField{ID: "lock:field", Placeholder: "Name"}.Layout(gtx, th, &g.lockField)
							}),
							kit.Rigid(kit.Gap(10, 0)),
							kit.Rigid(func(gtx C) D {
								return ui.Button{ID: "lock:save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &g.lockSave)
							}),
						)
					}),
					kit.Rigid(kit.Gap(0, 14)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D {
								return ui.Checkbox{ID: "lock:check", Label: "Sync", Checked: g.lockOn}.Layout(gtx, th, &g.lockCheck)
							}),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Rigid(func(gtx C) D {
								return ui.Tabs{ID: "lock:tab", Labels: segNames, Selected: g.lockTab, Style: ui.TabsSegmented}.Layout(gtx, th, &g.lockTabs)
							}),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Flexed(1, func(gtx C) D { return g.lockSlide.layout(gtx, th, "lock:slider", "Locked slider") }),
						)
					}),
				)
			})
		}),
	)
}

func (g *gallery) demoStatus(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Flex{Gap: kit.Space2, Align: layout.Middle}.Layout(gtx, th,
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusInfo, Text: "info"}.Layout(gtx, th) }),
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusGood, Text: "passing"}.Layout(gtx, th) }),
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusWarn, Text: "pending"}.Layout(gtx, th) }),
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusBad, Text: "failed"}.Layout(gtx, th) }),
				kit.Rigid(func(gtx C) D { return th.Spinner(gtx, 18) }),
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "Working…", th.Type.Body, th.Muted, kit.Regular) }),
			)
		}),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D { return th.Progress(gtx, g.volume.Value) }),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D { return th.Progress(gtx, -1) }),
	)
}

func (g *gallery) demoBanners(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Banner{Status: kit.StatusInfo, Title: "Update available", Body: "Version 0.2 adds themes, splits, logs and animation."}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return ui.Banner{Status: kit.StatusGood, Title: "All checks passed", Body: ""}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return ui.Banner{Status: kit.StatusWarn, Title: "Disk almost full", Body: "Free up space to keep syncing."}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return ui.Banner{Status: kit.StatusBad, Title: "Build failed", Body: "3 errors in kit/theme.go."}.Layout(gtx, th)
		}),
	)
}

func (g *gallery) demoKeyboard(gtx C) D {
	th := g.th
	var rows []layout.FlexChild
	for _, b := range g.keys.Bindings() {
		b := b
		rows = append(rows, kit.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(kit.Fixed(200, func(gtx C) D { return th.Text(gtx, b.Title, th.Type.Body, th.Fg, kit.Regular) })),
					kit.Rigid(func(gtx C) D { return ui.Kbd{Chord: b.Chord}.Layout(gtx, th) }),
				)
			})
		}))
	}
	rows = append(rows, kit.Rigid(func(gtx C) D {
		return th.Text(gtx, "Tab / Shift+Tab move focus; arrows move inside lists and menus; Enter activates. Ctrl+P finds any page.", th.Type.Small, th.Dim, kit.Regular)
	}))
	return kit.Col(gtx, rows...)
}

// slider pairs a ui.Slider's value with its state, for sliders that keep
// their own value.
type slider struct {
	Value float32
	state ui.SliderState
}

func (s *slider) layout(gtx C, th *kit.Theme, id, label string) D {
	if v, ok := s.state.Update(gtx); ok {
		s.Value = v
	}
	return ui.Slider{ID: id, Label: label, Value: s.Value}.Layout(gtx, th, &s.state)
}

var fruit = []ui.Choice{{ID: "apple", Label: "Apple"}, {ID: "banana", Label: "Banana"}, {ID: "cherry", Label: "Cherry"},
	{ID: "date", Label: "Date"}, {ID: "elder", Label: "Elderberry"}, {ID: "fig", Label: "Fig"}}

// demoMultiSelect: Ctrl-click, Shift-click, Shift+arrows, Ctrl+A, type-ahead.
func (g *gallery) demoMultiSelect(gtx C) D {
	th := g.th
	if e, ok := g.multi.UpdateMulti(gtx); ok {
		g.multiFocus, g.multiChosen = e.Focus, e.Chosen
	}
	return kit.Flex{Axis: layout.Vertical, Gap: kit.Space2}.Layout(gtx, th,
		kit.Rigid(func(gtx C) D {
			return th.Paragraph(gtx, fmt.Sprintf("%d chosen. Ctrl-click toggles, Shift-click or Shift+arrows select a range, Ctrl+A selects all; type to jump.", len(g.multiChosen)), th.Muted)
		}),
		kit.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(320))
			gtx.Constraints.Max.Y = gtx.Dp(200)
			return ui.SelectableList{ID: "multi", Items: fruit, Selected: g.multiFocus, Multiple: true, Chosen: g.multiChosen}.Layout(gtx, th, &g.multi)
		}),
	)
}
