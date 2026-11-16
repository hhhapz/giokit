package main

import (
	"fmt"
	"math/rand"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

type logsState struct {
	logBuf    kit.LogBuffer
	logView   kit.LogView
	logFilter widget.Editor
	levels    kit.Tabs
	stream    kit.Clickable
	addMany   kit.Clickable
	clear     kit.Clickable
	streaming bool
	nextLine  time.Time
	rng       *rand.Rand
	level     int
}

var levelNames = []string{"All", "Info", "Warn", "Error"}

var (
	logSources = []string{"http", "db", "auth", "cache", "worker"}
	logMsgs    = []string{
		"GET /api/items 200 in 12ms", "connection pool at 8/16", "token refreshed for user 42",
		"cache miss key=items:all", "job sync-inventory finished in 1.3s", "slow query (412ms): SELECT * FROM items",
		"retrying upstream request (attempt 2)", "failed to reach payments: timeout after 5s", "POST /api/items 201 in 31ms",
		"evicted 120 entries", "scheduled job cleanup in 10m", "websocket client connected",
	}
)

func unitDp(f float32) unit.Dp { return unit.Dp(f) }
func unitSp(f float32) unit.Sp { return unit.Sp(f) }

func (g *gallery) initLogs() {
	g.rng = rand.New(rand.NewSource(7))
	g.logBuf.Cap = 50000
	g.logFilter.SingleLine = true
	g.levels.Style = kit.TabsSegmented
	base := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	for i := 0; i < 200; i++ {
		g.logBuf.Append(g.fakeLine(base.Add(time.Duration(i) * 137 * time.Millisecond)))
	}
}

func (g *gallery) fakeLine(t time.Time) kit.LogLine {
	i := g.rng.Intn(len(logMsgs))
	lvl := kit.LogInfo
	switch {
	case i == 5 || i == 6:
		lvl = kit.LogWarn
	case i == 7:
		lvl = kit.LogError
	case i == 1 || i == 3 || i == 9:
		lvl = kit.LogDebug
	}
	return kit.LogLine{Time: t, Level: lvl, Source: logSources[g.rng.Intn(len(logSources))], Text: logMsgs[i]}
}

func (g *gallery) handleLogs(gtx C) {
	if g.stream.Clicked(gtx) {
		g.streaming = !g.streaming
		g.nextLine = gtx.Now
	}
	if g.addMany.Clicked(gtx) {
		for i := 0; i < 1000; i++ {
			g.logBuf.Append(g.fakeLine(gtx.Now))
		}
	}
	if g.clear.Clicked(gtx) {
		g.logBuf.Clear()
	}
	if i, ok := g.levels.Clicked(gtx); ok {
		g.level = i
	}
	for {
		if _, ok := g.logFilter.Update(gtx); !ok {
			break
		}
	}
	g.logView.Filter = g.logFilter.Text()
	g.logView.MinLevel = []kit.LogLevel{kit.LogDebug, kit.LogInfo, kit.LogWarn, kit.LogError}[g.level]
	// Streaming without a goroutine: emit lines on a timer driven by the frame
	// clock. (A real tail would run in a goroutine and hand lines over with
	// app.Post; see kit.Spawn.)
	if g.streaming {
		for !gtx.Now.Before(g.nextLine) {
			g.logBuf.Append(g.fakeLine(g.nextLine))
			g.nextLine = g.nextLine.Add(120 * time.Millisecond)
		}
		gtx.Execute(op.InvalidateCmd{At: g.nextLine})
	}
}

func (g *gallery) logs(gtx C) D {
	th := g.th
	return g.pageFrame(gtx, "Logs", nil, func(gtx C) D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return layout.Inset{Left: 24, Right: 24, Bottom: 12}.Layout(gtx, func(gtx C) D {
					// Flow, not Row: at large text and control sizes the toolbar
					// wraps instead of squeezing the last buttons to nothing.
					return kit.Flow(gtx, 10, 8,
						kit.Fixed(260, func(gtx C) D {
							return th.TextFieldIcon(gtx, &g.logFilter, "logs:filter", kit.IconSearch, "Filter lines")
						}),
						func(gtx C) D { return th.Tabs(gtx, &g.levels, "level", levelNames, g.level) },
						func(gtx C) D { return th.Switch(gtx, &g.stream, "logs:stream", "Stream", g.streaming) },
						func(gtx C) D {
							return th.Text(gtx, fmt.Sprintf("%d of %d lines", g.logView.Count(&g.logBuf), g.logBuf.Len()), th.Type.Small, th.Dim, kit.Regular)
						},
						func(gtx C) D { return th.Button(gtx, &g.addMany, "logs:add", "Add 1000", kit.Secondary) },
						func(gtx C) D { return th.Button(gtx, &g.clear, "logs:clear", "Clear", kit.Ghost) },
					)
				})
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Flexed(1, func(gtx C) D { return th.LogView(gtx, &g.logView, &g.logBuf, "logs") }),
		)
	})
}
