package main

import (
	"fmt"
	"slices"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type controlsState struct {
	multi       ui.SelectableListState
	multiFocus  string
	multiChosen map[string]bool
	kinds       [len(buttonKinds)]kit.Clickable
	iconBtn     kit.Clickable
	iconTip     ui.TooltipState
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

// buttonKind is one button style the Buttons demo shows.
type buttonKind struct {
	name  string // the button's label and id suffix
	style ui.ButtonStyle
}

// buttonKinds are the demo's button styles, in display order.
var buttonKinds = [...]buttonKind{
	{name: "Secondary", style: ui.Secondary},
	{name: "Primary", style: ui.Primary},
	{name: "Ghost", style: ui.Ghost},
	{name: "Danger", style: ui.Danger},
	{name: "Solid", style: ui.Solid},
}

var (
	sizeNames = []string{"Small", "Medium", "Large"}
	tabNames  = []string{"General", "Editor", "Terminal", "Extensions"}
	segNames  = []string{"Day", "Week", "Month"}
)

func (g *gallery) initControls() {
	g.controls.wifiOn, g.controls.size = true, "Medium"
	g.controls.name.SingleLine, g.controls.name.Submit = true, true
	g.controls.search.SingleLine = true
	g.controls.volume.Value = 0.4
	g.controls.locked, g.controls.lockOn = true, true
	g.controls.lockField.SingleLine = true
	g.controls.lockField.SetText("read-only while locked")
	g.controls.lockSlide.Value = 0.6
}

func (g *gallery) handleControls(gtx C) {
	for i := range g.controls.kinds {
		if g.controls.kinds[i].Clicked(gtx) {
			g.status = buttonKinds[i].name + " clicked"
		}
	}
	if g.controls.iconBtn.Clicked(gtx) {
		g.status = "Search clicked"
	}
	if g.controls.wifi.Clicked(gtx) {
		g.controls.wifiOn = !g.controls.wifiOn
	}
	if g.controls.notify.Clicked(gtx) {
		g.controls.notifyOn = !g.controls.notifyOn
	}
	if g.controls.agree.Clicked(gtx) {
		g.controls.agreed = !g.controls.agreed
	}
	for _, s := range sizeNames {
		if g.controls.sizes.Get(s).Clicked(gtx) {
			g.controls.size = s
		}
	}
	if id, ok := g.controls.tabs.Update(gtx); ok {
		g.controls.tab = slices.Index(tabNames, id)
	}
	if id, ok := g.controls.segs.Update(gtx); ok {
		g.controls.seg = slices.Index(segNames, id)
	}
	if g.controls.lock.Clicked(gtx) {
		g.controls.locked = !g.controls.locked
	}
	if g.controls.lockSave.Clicked(gtx) {
		g.status = "Saved"
	}
	if g.controls.lockCheck.Clicked(gtx) {
		g.controls.lockOn = !g.controls.lockOn
	}
	if id, ok := g.controls.lockTabs.Update(gtx); ok {
		g.controls.lockTab = slices.Index(segNames, id)
	}
	g.controls.volume.update(gtx)
	g.controls.lockSlide.update(gtx)
	if e, ok := g.controls.multi.Update(gtx); ok {
		g.controls.multiFocus, g.controls.multiChosen = e.Selected, e.Chosen
	}
	for {
		e, ok := g.controls.name.Update(gtx)
		if !ok {
			break
		}
		if _, ok := e.(widget.SubmitEvent); ok {
			g.status = "Hello, " + g.controls.name.Text()
			kit.Notify(gtx)
		}
	}
}

// section is a titled card on the controls page.
func (g *gallery) section(gtx C, title string, body layout.Widget) D {
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
	scale := map[string]float32{"Small": 0.85, "Medium": 1, "Large": 1.25}[g.controls.size]
	if scale == 1 {
		return g.th
	}
	return g.th.Derive(func(t *kit.Theme) {
		t.Density *= scale
		t.Type.Body *= unit.Sp(scale)
	})
}

func (g *gallery) demoButtons(gtx C) D {
	th := g.sizedTheme()
	var btns []layout.FlexChild
	for i, k := range buttonKinds {
		btns = append(btns, kit.Rigid(func(gtx C) D {
			return ui.Button{ID: "btn:" + k.name, Text: k.name, Style: k.style}.Layout(gtx, th, &g.controls.kinds[i])
		}))
	}
	btns = append(btns, kit.Rigid(func(gtx C) D {
		return ui.Tooltip{ID: "search:tip", Text: "Search (tooltip)", Content: func(gtx C) D {
			return ui.IconButton{ID: "btn:search", Icon: kit.IconSearch, Label: "Search", Active: true}.Layout(gtx, th, &g.controls.iconBtn)
		}}.Layout(gtx, th, &g.controls.iconTip)
	}))
	sizes := []layout.FlexChild{kit.Rigid(func(gtx C) D { return g.th.Text(gtx, "Size", g.th.Type.Body, g.th.Muted, kit.Regular) })}
	for _, s := range sizeNames {
		sizes = append(sizes, kit.Rigid(func(gtx C) D {
			return ui.Radio{ID: "radio:" + s, Label: s, Selected: g.controls.size == s}.Layout(gtx, g.th, g.controls.sizes.Get(s))
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
			return ui.Switch{ID: "switch:wifi", Label: "Wi-Fi", On: g.controls.wifiOn}.Layout(gtx, th, &g.controls.wifi)
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Switch{ID: "switch:notify", Label: "Notifications", On: g.controls.notifyOn}.Layout(gtx, th, &g.controls.notify)
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Checkbox{ID: "check:agree", Label: "I agree", Checked: g.controls.agreed}.Layout(gtx, th, &g.controls.agree)
		}),
	)
}

func (g *gallery) demoSlider(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(func(gtx C) D { return th.Text(gtx, "Volume", th.Type.Body, th.Muted, kit.Regular) }),
		kit.Rigid(kit.Gap(12, 0)),
		kit.Flexed(1, func(gtx C) D { return g.controls.volume.layout(gtx, th, "slider:volume", "Volume") }),
		kit.Rigid(kit.Gap(12, 0)),
		kit.Rigid(kit.Fixed(40, func(gtx C) D {
			return th.Text(gtx, fmt.Sprintf("%d%%", int(g.controls.volume.Value*100+0.5)), th.Type.Body, th.Fg, kit.Medium)
		})),
	)
}

func (g *gallery) demoTextFields(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				kit.Flexed(1, func(gtx C) D {
					return ui.TextField{ID: "field:name", Placeholder: "Your name, then Enter"}.Layout(gtx, th, &g.controls.name)
				}),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, func(gtx C) D {
					return ui.TextField{ID: "field:search", Placeholder: "Search", Icon: kit.IconSearch}.Layout(gtx, th, &g.controls.search)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return ui.TextArea{ID: "field:notes", Placeholder: "Notes (Enter for a new line)", Lines: 3}.Layout(gtx, th, &g.controls.notes)
		}),
	)
}

func (g *gallery) demoTabs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Tabs{ID: "tab", Items: choices(tabNames), Selected: tabNames[g.controls.tab]}.Layout(gtx, th, &g.controls.tabs)
		}),
		kit.Rigid(kit.HLine(th.Divider)),
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 14}.Layout(gtx, func(gtx C) D {
				return th.Text(gtx, tabNames[g.controls.tab]+" settings would go here.", th.Type.Body, th.Muted, kit.Regular)
			})
		}),
		kit.Rigid(func(gtx C) D {
			return ui.Tabs{ID: "seg", Items: choices(segNames), Selected: segNames[g.controls.seg], Style: ui.TabsSegmented}.Layout(gtx, th, &g.controls.segs)
		}),
	)
}

func (g *gallery) demoDisabled(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return ui.Switch{ID: "switch:lock", Label: "Lock the form", On: g.controls.locked}.Layout(gtx, th, &g.controls.lock)
		}),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			return th.Disable(gtx, g.controls.locked, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Flexed(1, func(gtx C) D {
								return ui.TextField{ID: "lock:field", Placeholder: "Name"}.Layout(gtx, th, &g.controls.lockField)
							}),
							kit.Rigid(kit.Gap(10, 0)),
							kit.Rigid(func(gtx C) D {
								return ui.Button{ID: "lock:save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &g.controls.lockSave)
							}),
						)
					}),
					kit.Rigid(kit.Gap(0, 14)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D {
								return ui.Checkbox{ID: "lock:check", Label: "Sync", Checked: g.controls.lockOn}.Layout(gtx, th, &g.controls.lockCheck)
							}),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Rigid(func(gtx C) D {
								return ui.Tabs{ID: "lock:tab", Items: choices(segNames), Selected: segNames[g.controls.lockTab], Style: ui.TabsSegmented}.Layout(gtx, th, &g.controls.lockTabs)
							}),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Flexed(1, func(gtx C) D { return g.controls.lockSlide.layout(gtx, th, "lock:slider", "Locked slider") }),
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
		kit.Rigid(func(gtx C) D { return th.Progress(gtx, g.controls.volume.Value) }),
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
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
			return ui.ShortcutList{Groups: []ui.ShortcutGroup{{Items: ui.ShortcutsFrom(&g.keys)}}}.Layout(gtx, th)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D {
			return th.Text(gtx, "Tab / Shift+Tab move focus; arrows move inside lists and menus; Enter activates. Ctrl+P finds any page.", th.Type.Small, th.Dim, kit.Regular)
		}),
	)
}

// slider is a ui.Slider that keeps its own value: update applies input in
// the handle pass, layout draws the value.
type slider struct {
	Value float32
	state ui.SliderState
}

// update applies this frame's input to Value and reports whether it
// changed.
func (s *slider) update(gtx C) bool {
	v, ok := s.state.Update(gtx)
	if ok {
		s.Value = v
	}
	return ok
}

func (s *slider) layout(gtx C, th *kit.Theme, id, label string) D {
	return ui.Slider{ID: id, Label: label, Value: s.Value}.Layout(gtx, th, &s.state)
}

var fruit = []ui.Choice{{ID: "apple", Label: "Apple"}, {ID: "banana", Label: "Banana"}, {ID: "cherry", Label: "Cherry"},
	{ID: "date", Label: "Date"}, {ID: "elder", Label: "Elderberry"}, {ID: "fig", Label: "Fig"}}

// demoMultiSelect shows multi-selection: Ctrl-click, Shift-click,
// Shift+arrows, Ctrl+A and type-ahead.
func (g *gallery) demoMultiSelect(gtx C) D {
	th := g.th
	return kit.Flex{Axis: layout.Vertical, Gap: kit.Space2}.Layout(gtx, th,
		kit.Rigid(func(gtx C) D {
			return th.Paragraph(gtx, fmt.Sprintf("%d chosen. Ctrl-click toggles, Shift-click or Shift+arrows select a range, Ctrl+A selects all; type to jump.", len(g.controls.multiChosen)), th.Muted)
		}),
		kit.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(320))
			gtx.Constraints.Max.Y = gtx.Dp(200)
			return ui.SelectableList{ID: "multi", Items: fruit, Selected: g.controls.multiFocus, Multiple: true, Chosen: g.controls.multiChosen}.Layout(gtx, th, &g.controls.multi)
		}),
	)
}
