package main

import (
	"fmt"
	"math/rand"
	"slices"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kit/x"
)

type logsState struct {
	buf       x.LogBuffer
	view      x.LogViewState
	filter    widget.Editor
	levels    ui.TabsState
	stream    kit.Clickable
	addMany   kit.Clickable
	clear     kit.Clickable
	streaming bool
	nextLine  time.Time
	rng       *rand.Rand
	level     int
}

// levelOption is one choice of the log demo's level filter.
type levelOption struct {
	label string     // the tab's label and id
	min   x.LogLevel // the least severe level shown
}

// levels are the level filter's tabs, in display order.
var levels = []levelOption{
	{label: "All", min: x.LogDebug},
	{label: "Info", min: x.LogInfo},
	{label: "Warn", min: x.LogWarn},
	{label: "Error", min: x.LogError},
}

// levelChoices are the level filter's items.
func levelChoices() []ui.Choice {
	items := make([]ui.Choice, len(levels))
	for i, l := range levels {
		items[i] = ui.Choice{ID: l.label, Label: l.label}
	}
	return items
}

var (
	logSources = []string{"http", "db", "auth", "cache", "worker"}
	logMsgs    = []string{
		"GET /api/items 200 in 12ms", "connection pool at 8/16", "token refreshed for user 42",
		"cache miss key=items:all", "job sync-inventory finished in 1.3s", "slow query (412ms): SELECT * FROM items",
		"retrying upstream request (attempt 2)", "failed to reach payments: timeout after 5s", "POST /api/items 201 in 31ms",
		"evicted 120 entries", "scheduled job cleanup in 10m", "websocket client connected",
	}
)

func (g *gallery) initLogs() {
	g.logs.rng = rand.New(rand.NewSource(7))
	g.logs.buf.Cap = 50000
	g.logs.filter.SingleLine = true
	base := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	for i := 0; i < 200; i++ {
		g.logs.buf.Append(g.fakeLine(base.Add(time.Duration(i) * 137 * time.Millisecond)))
	}
}

func (g *gallery) fakeLine(t time.Time) x.LogLine {
	i := g.logs.rng.Intn(len(logMsgs))
	lvl := x.LogInfo
	switch i {
	case 5, 6:
		lvl = x.LogWarn
	case 7:
		lvl = x.LogError
	case 1, 3, 9:
		lvl = x.LogDebug
	}
	return x.LogLine{Time: t, Level: lvl, Source: logSources[g.logs.rng.Intn(len(logSources))], Text: logMsgs[i]}
}

func (g *gallery) handleLogs(gtx C) {
	if g.logs.stream.Clicked(gtx) {
		g.logs.streaming = !g.logs.streaming
		g.logs.nextLine = gtx.Now
	}
	if g.logs.addMany.Clicked(gtx) {
		for i := 0; i < 1000; i++ {
			g.logs.buf.Append(g.fakeLine(gtx.Now))
		}
	}
	if g.logs.clear.Clicked(gtx) {
		g.logs.buf.Clear()
	}
	if id, ok := g.logs.levels.Update(gtx); ok {
		g.logs.level = slices.IndexFunc(levels, func(l levelOption) bool { return l.label == id })
	}
	for {
		if _, ok := g.logs.filter.Update(gtx); !ok {
			break
		}
	}
	g.logs.view.Filter = g.logs.filter.Text()
	g.logs.view.MinLevel = levels[g.logs.level].min
	// Streaming without a goroutine: emit lines on a timer driven by the frame
	// clock. (A real tail would run in a goroutine and hand lines over with
	// app.Post; see kit.Spawn.)
	if g.logs.streaming {
		for !gtx.Now.Before(g.logs.nextLine) {
			g.logs.buf.Append(g.fakeLine(g.logs.nextLine))
			g.logs.nextLine = g.logs.nextLine.Add(120 * time.Millisecond)
		}
		gtx.Execute(op.InvalidateCmd{At: g.logs.nextLine})
	}
}

func (g *gallery) logsPage(gtx C) D {
	th := g.th
	return g.pageFrame(gtx, "Logs", nil, func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return layout.Inset{Left: 24, Right: 24, Bottom: 12}.Layout(gtx, func(gtx C) D {
					// Flow, not Row: at large text and control sizes the toolbar
					// wraps instead of squeezing the last buttons to nothing.
					return kit.Flow(gtx, 10, 8,
						kit.Fixed(260, func(gtx C) D {
							return ui.TextField{ID: "logs:filter", Placeholder: "Filter lines", Icon: kit.IconSearch}.Layout(gtx, th, &g.logs.filter)
						}),
						func(gtx C) D {
							return ui.Tabs{ID: "level", Items: levelChoices(), Selected: levels[g.logs.level].label, Style: ui.TabsSegmented}.Layout(gtx, th, &g.logs.levels)
						},
						func(gtx C) D {
							return ui.Switch{ID: "logs:stream", Label: "Stream", On: g.logs.streaming}.Layout(gtx, th, &g.logs.stream)
						},
						func(gtx C) D {
							return th.Text(gtx, fmt.Sprintf("%d of %d lines", g.logs.view.Count(&g.logs.buf), g.logs.buf.Len()), th.Type.Small, th.Dim, kit.Regular)
						},
						func(gtx C) D { return ui.Button{ID: "logs:add", Text: "Add 1000"}.Layout(gtx, th, &g.logs.addMany) },
						func(gtx C) D {
							return ui.Button{ID: "logs:clear", Text: "Clear", Style: ui.Ghost}.Layout(gtx, th, &g.logs.clear)
						},
					)
				})
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Flexed(1, func(gtx C) D { return x.LogView{ID: "logs", Buffer: &g.logs.buf}.Layout(gtx, th, &g.logs.view) }),
		)
	})
}
