package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

type controlsState struct {
	kinds    [5]kit.Clickable
	iconBtn  kit.Clickable
	iconTip  kit.Tooltip
	saveTip  kit.Tooltip
	wifi     kit.Clickable
	notify   kit.Clickable
	agree    kit.Clickable
	sizes    kit.Pool[kit.Clickable]
	name     widget.Editor
	search   widget.Editor
	notes    widget.Editor
	volume   kit.Slider
	tabs     kit.Tabs
	segs     kit.Tabs
	wifiOn   bool
	notifyOn bool
	agreed   bool
	size     string
	tab      int
	seg      int

	// Disabled section: a form that the switch above it locks.
	lock      kit.Clickable
	locked    bool
	lockSave  kit.Clickable
	lockCheck kit.Clickable
	lockOn    bool
	lockField widget.Editor
	lockSlide kit.Slider
	lockTabs  kit.Tabs
	lockTab   int
}

var (
	kindNames = []string{"Secondary", "Primary", "Ghost", "Danger", "Solid"}
	kindVals  = []kit.ButtonKind{kit.Secondary, kit.Primary, kit.Ghost, kit.Danger, kit.Solid}
	sizeNames = []string{"Small", "Medium", "Large"}
	tabNames  = []string{"General", "Editor", "Terminal", "Extensions"}
	segNames  = []string{"Day", "Week", "Month"}
)

func (g *gallery) initControls() {
	g.wifiOn, g.size = true, "Medium"
	g.name.SingleLine, g.name.Submit = true, true
	g.search.SingleLine = true
	g.volume.Value = 0.4
	g.segs.Style = kit.TabsSegmented
	g.locked, g.lockOn = true, true
	g.lockField.SingleLine = true
	g.lockField.SetText("read-only while locked")
	g.lockSlide.Value = 0.6
	g.lockTabs.Style = kit.TabsSegmented
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
	if i, ok := g.tabs.Clicked(gtx); ok {
		g.tab = i
	}
	if i, ok := g.segs.Clicked(gtx); ok {
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
	if i, ok := g.lockTabs.Clicked(gtx); ok {
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

// hrow lays widgets out in a row with even gaps.
func hrow(gtx C, gap float32, ws ...kit.W) D {
	var ch []layout.FlexChild
	for i, w := range ws {
		if i > 0 {
			ch = append(ch, kit.Rigid(kit.Gap(unitDp(gap), 0)))
		}
		ch = append(ch, kit.Rigid(w))
	}
	return kit.Row(gtx, ch...)
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
	var ws []kit.W
	for i := range kindNames {
		i := i
		ws = append(ws, func(gtx C) D { return th.Button(gtx, &g.kinds[i], "btn:"+kindNames[i], kindNames[i], kindVals[i]) })
	}
	ws = append(ws, func(gtx C) D {
		return th.Tooltip(gtx, &g.iconTip, "Search (tooltip)", func(gtx C) D {
			return th.IconButton(gtx, &g.iconBtn, "btn:search", kit.IconSearch, true)
		})
	})
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return hrow(gtx, 8, ws...) }),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			ws := []kit.W{func(gtx C) D { return g.th.Text(gtx, "Size", g.th.Type.Body, g.th.Muted, kit.Regular) }}
			for _, s := range sizeNames {
				s := s
				ws = append(ws, func(gtx C) D { return g.th.Radio(gtx, g.sizes.Get(s), "radio:"+s, s, g.size == s) })
			}
			return hrow(gtx, 20, ws...)
		}),
	)
}

func (g *gallery) demoToggles(gtx C) D {
	th := g.th
	return hrow(gtx, 24,
		func(gtx C) D { return th.Switch(gtx, &g.wifi, "switch:wifi", "Wi-Fi", g.wifiOn) },
		func(gtx C) D { return th.Switch(gtx, &g.notify, "switch:notify", "Notifications", g.notifyOn) },
		func(gtx C) D { return th.CheckboxLabel(gtx, &g.agree, "check:agree", "I agree", g.agreed) },
	)
}

func (g *gallery) demoSlider(gtx C) D {
	th := g.th
	return kit.Row(gtx,
		kit.Rigid(func(gtx C) D { return th.Text(gtx, "Volume", th.Type.Body, th.Muted, kit.Regular) }),
		kit.Rigid(kit.Gap(12, 0)),
		kit.Flexed(1, func(gtx C) D { return th.Slider(gtx, &g.volume, "slider:volume") }),
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
				kit.Flexed(1, func(gtx C) D { return th.TextField(gtx, &g.name, "field:name", "Your name, then Enter") }),
				kit.Rigid(kit.Gap(10, 0)),
				kit.Flexed(1, func(gtx C) D { return th.TextFieldIcon(gtx, &g.search, "field:search", kit.IconSearch, "Search") }),
			)
		}),
		kit.Rigid(kit.Gap(0, 10)),
		kit.Rigid(func(gtx C) D { return th.TextArea(gtx, &g.notes, "field:notes", "Notes (Enter for a new line)", 3) }),
	)
}

func (g *gallery) demoTabs(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return th.Tabs(gtx, &g.tabs, "tab", tabNames, g.tab) }),
		kit.Rigid(kit.HLine(th.Divider)),
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 14}.Layout(gtx, func(gtx C) D {
				return th.Text(gtx, tabNames[g.tab]+" settings would go here.", th.Type.Body, th.Muted, kit.Regular)
			})
		}),
		kit.Rigid(func(gtx C) D { return th.Tabs(gtx, &g.segs, "seg", segNames, g.seg) }),
	)
}

func (g *gallery) demoDisabled(gtx C) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D { return th.Switch(gtx, &g.lock, "switch:lock", "Lock the form", g.locked) }),
		kit.Rigid(kit.Gap(0, 14)),
		kit.Rigid(func(gtx C) D {
			return th.Disable(gtx, g.locked, func(gtx C) D {
				return kit.Col(gtx,
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Flexed(1, func(gtx C) D { return th.TextField(gtx, &g.lockField, "lock:field", "Name") }),
							kit.Rigid(kit.Gap(10, 0)),
							kit.Rigid(func(gtx C) D { return th.Button(gtx, &g.lockSave, "lock:save", "Save", kit.Solid) }),
						)
					}),
					kit.Rigid(kit.Gap(0, 14)),
					kit.Rigid(func(gtx C) D {
						return kit.Row(gtx,
							kit.Rigid(func(gtx C) D { return th.CheckboxLabel(gtx, &g.lockCheck, "lock:check", "Sync", g.lockOn) }),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Rigid(func(gtx C) D { return th.Tabs(gtx, &g.lockTabs, "lock:tab", segNames, g.lockTab) }),
							kit.Rigid(kit.Gap(20, 0)),
							kit.Flexed(1, func(gtx C) D { return th.Slider(gtx, &g.lockSlide, "lock:slider") }),
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
			return hrow(gtx, 8,
				func(gtx C) D { return th.Badge(gtx, kit.StatusInfo, "info") },
				func(gtx C) D { return th.Badge(gtx, kit.StatusGood, "passing") },
				func(gtx C) D { return th.Badge(gtx, kit.StatusWarn, "pending") },
				func(gtx C) D { return th.Badge(gtx, kit.StatusBad, "failed") },
				func(gtx C) D { return th.Spinner(gtx, 18) },
				func(gtx C) D { return th.Text(gtx, "Working…", th.Type.Body, th.Muted, kit.Regular) },
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
			return th.Banner(gtx, kit.StatusInfo, "Update available", "Version 0.2 adds themes, splits, logs and animation.")
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D { return th.Banner(gtx, kit.StatusGood, "All checks passed", "") }),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D {
			return th.Banner(gtx, kit.StatusWarn, "Disk almost full", "Free up space to keep syncing.")
		}),
		kit.Rigid(kit.Gap(0, 8)),
		kit.Rigid(func(gtx C) D { return th.Banner(gtx, kit.StatusBad, "Build failed", "3 errors in kit/theme.go.") }),
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
					kit.Rigid(func(gtx C) D { return th.Kbd(gtx, b.Chord) }),
				)
			})
		}))
	}
	rows = append(rows, kit.Rigid(func(gtx C) D {
		return th.Text(gtx, "Tab / Shift+Tab move focus; arrows move inside lists and menus; Enter activates. Ctrl+P finds any page.", th.Type.Small, th.Dim, kit.Regular)
	}))
	return kit.Col(gtx, rows...)
}
