// Command scrolltest is a bench for wheel/touchpad scroll smoothness.
//
// Two lists scroll side by side, and each draws a live graph of how far its
// content moved per frame. A skip shows as a red spike and counts in the
// lane header.
//
//   - Left, the control: plain text rows, no components.
//
//   - Right: a component on every row (buttons, switches, slider, tabs,
//     spinner…). Press C to swap them for plain blocks of identical height.
//     If skips follow C, they're component-linked. If they don't, the
//     component link was a coincidence.
//
//   - U toggles Gio's wheel scaling between giokit's patch and upstream (the
//     ~17px-per-click skip on high-resolution wheels, docs/pitfalls.md #22),
//     for an A/B test in one build.
//
//   - R resets positions and counters.
//
//     go run ./examples/scrolltest
//     GIOKIT_TRACE=1 go run ./examples/scrolltest 2>trace.log  # plus the raw events
package main

import (
	"fmt"
	"image"
	"sort"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
	_ "github.com/hhhapz/giokit/kit/forktrace" // Gio window events in GIOKIT_TRACE
	"github.com/hhhapz/giokit/kit/ui"
)

type C = layout.Context
type D = layout.Dimensions

const rows = 160

// lane is one scrolling list plus its continuity stats.
type lane struct {
	id      string
	list    kit.List
	height  func(i int) unit.Dp // fixed row heights, so absolute offset is exact
	abs     int                 // content scrolled, px
	started bool
	last    time.Time // last frame that moved
	recent  []int     // |Δ| of recent frames in this gesture
	deltas  []int     // graph: Δ per moving frame, newest last
	flagged []bool
	skips   int
	lastD   int
}

func (l *lane) reset() {
	l.list.Position = layout.Position{}
	l.abs, l.started, l.recent, l.deltas, l.flagged, l.skips, l.lastD = 0, false, nil, nil, nil, 0, 0
}

// measure records how far the content moved this frame and flags a skip: one
// frame moving far more than the frames around it in the same gesture. It
// runs after the list lays out, below the header that shows its numbers, so
// a change asks for another frame (rule: notify after a mid-frame mutation).
func (l *lane) measure(gtx C) {
	abs := l.list.Position.Offset
	for i := 0; i < l.list.Position.First; i++ {
		abs += gtx.Dp(l.height(i))
	}
	if !l.started {
		l.abs, l.started = abs, true
		return
	}
	d := abs - l.abs
	l.abs = abs
	if d == 0 {
		return
	}
	kit.Notify(gtx)
	if gtx.Now.Sub(l.last) > 250*time.Millisecond {
		l.recent = l.recent[:0] // a new gesture
	}
	l.last = gtx.Now
	ad := max(d, -d)
	skip := false
	if len(l.recent) >= 4 {
		s := append([]int(nil), l.recent...)
		sort.Ints(s)
		med := s[len(s)/2]
		skip = ad >= 3*med && ad-med >= gtx.Dp(6)
	}
	l.recent = append(l.recent, ad)
	if len(l.recent) > 12 {
		l.recent = l.recent[1:]
	}
	if skip {
		l.skips++
	}
	l.lastD = d
	l.deltas = append(l.deltas, d)
	l.flagged = append(l.flagged, skip)
	if len(l.deltas) > 600 {
		l.deltas, l.flagged = l.deltas[1:], l.flagged[1:]
	}
}

type bench struct {
	th          *kit.Theme
	keys        kit.Keymap
	plain, comp lane
	placeholder bool

	clicks  kit.Pool[kit.Clickable]
	sliders kit.Pool[slider]
	tabs    kit.Pool[ui.TabsState]
	on      map[string]bool
	tab     map[string]int
}

var compHeights = []unit.Dp{40, 48, 48, 44, 48, 44, 64, 40} // per row kind

func newBench() *bench {
	b := &bench{th: kit.Dark(), on: map[string]bool{}, tab: map[string]int{}}
	b.plain = lane{id: "plain", height: func(int) unit.Dp { return 36 }}
	b.comp = lane{id: "comp", height: func(i int) unit.Dp { return compHeights[i%len(compHeights)] }}
	b.keys.MustBind("U", "Toggle upstream wheel scaling", func() {
		app.GiokitUpstreamWheel.Store(!app.GiokitUpstreamWheel.Load())
	})
	b.keys.MustBind("C", "Toggle components / placeholders", func() { b.placeholder = !b.placeholder })
	b.keys.MustBind("R", "Reset", func() { b.plain.reset(); b.comp.reset() })
	return b
}

func (b *bench) Frame(gtx C) {
	b.handle(gtx)
	b.layout(gtx)
	b.keys.Attach(gtx)
}

func (b *bench) handle(gtx C) {
	b.clicks.Each(func(id string, c *kit.Clickable) {
		if c.Clicked(gtx) {
			b.on[id] = !b.on[id]
		}
	})
	b.tabs.Each(func(id string, t *ui.TabsState) {
		if i, ok := t.Update(gtx); ok {
			b.tab[id] = i
		}
	})
}

func (b *bench) layout(gtx C) D {
	th := b.th
	scaling := "PATCHED (even)"
	sc := th.Good
	if app.GiokitUpstreamWheel.Load() {
		scaling, sc = "UPSTREAM (skips on hi-res wheels)", th.Warn
	}
	return kit.Fill(gtx, th.Bg, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return th.Bar(gtx, 8, 16, func(gtx C) D {
					return kit.Row(gtx,
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "Scroll bench", th.Type.Text, th.Fg, kit.Bold) }),
						kit.Rigid(kit.Gap(16, 0)),
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "Wheel scaling: "+scaling, th.Type.Body, sc, kit.Bold) }),
						kit.Flexed(1, kit.Grow),
						b.hint("U", "scaling"), b.hint("C", "components"), b.hint("R", "reset"),
					)
				})
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Flexed(1, func(gtx C) D {
				return kit.Row(gtx,
					kit.Flexed(1, func(gtx C) D { return b.lane(gtx, &b.plain, "Plain rows (control)", b.plainRow) }),
					kit.Rigid(kit.VLine(th.Border)),
					kit.Flexed(1, func(gtx C) D {
						title := "Components"
						if b.placeholder {
							title = "Placeholders (same heights)"
						}
						return b.lane(gtx, &b.comp, title, b.compRow)
					}),
				)
			}),
		)
	})
}

func (b *bench) hint(k, label string) layout.FlexChild {
	th := b.th
	return kit.Rigid(func(gtx C) D {
		return layout.Inset{Left: 12}.Layout(gtx, func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D { return ui.Kbd{Chord: k}.Layout(gtx, th) }),
				kit.Rigid(kit.Gap(4, 0)),
				kit.Rigid(func(gtx C) D { return th.Text(gtx, label, th.Type.Small, th.Muted, kit.Regular) }),
			)
		})
	})
}

func (b *bench) lane(gtx C, l *lane, title string, row func(gtx C, i int) D) D {
	th := b.th
	gtx.Constraints.Min = gtx.Constraints.Max
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: 10, Bottom: 6, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
				sk := th.Muted
				if l.skips > 0 {
					sk = th.Bad
				}
				return kit.Row(gtx,
					kit.Rigid(func(gtx C) D { return th.Text(gtx, title, th.Type.Body, th.Fg, kit.Bold) }),
					kit.Flexed(1, kit.Grow),
					kit.Rigid(func(gtx C) D {
						return th.Text(gtx, fmt.Sprintf("skips %d", l.skips), th.Type.Small, sk, kit.Bold)
					}),
					kit.Rigid(func(gtx C) D {
						return th.Text(gtx, fmt.Sprintf("  ·  list jumps %d  ·  last Δ %+dpx  ·  offset %dpx",
							l.list.Jumps(), l.lastD, l.abs), th.Type.Small, th.Muted, kit.Regular)
					}),
				)
			})
		}),
		kit.Rigid(func(gtx C) D { return b.graph(gtx, l) }),
		kit.Rigid(kit.HLine(th.Border)),
		kit.Flexed(1, func(gtx C) D {
			d := th.List(gtx, &l.list, l.id, rows, func(gtx C, i int) D {
				h := gtx.Dp(l.height(i))
				gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return row(gtx, i)
			})
			l.measure(gtx)
			return d
		}),
	)
}

// graph draws Δ per moving frame, newest at the right: up for scrolling down,
// down for up, red where a skip was flagged.
func (b *bench) graph(gtx C, l *lane) D {
	th := b.th
	w, h := gtx.Constraints.Max.X, gtx.Dp(72)
	mid := h / 2
	paint.FillShape(gtx.Ops, th.Panel, clip.Rect(image.Rect(0, 0, w, h)).Op())
	paint.FillShape(gtx.Ops, th.Divider, clip.Rect(image.Rect(0, mid, w, mid+1)).Op())
	bw := max(2, gtx.Dp(2))
	step := bw + max(1, gtx.Dp(1))
	n := min(len(l.deltas), w/step)
	for k := 0; k < n; k++ {
		j := len(l.deltas) - n + k
		d := l.deltas[j]
		v := min(max(d, -d), mid-2) // 1px of bar per px moved, capped
		c := th.Accent
		if l.flagged[j] {
			c = th.Bad
		}
		x := w - (n-k)*step
		r := image.Rect(x, mid-v, x+bw, mid)
		if d < 0 {
			r = image.Rect(x, mid+1, x+bw, mid+1+v)
		}
		paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
	}
	return D{Size: image.Pt(w, h)}
}

// rowFrame draws the shared row chrome: a divider on top and the row index on
// the right, so continuity is easy to follow by eye.
func (b *bench) rowFrame(gtx C, i int, body kit.W) D {
	th := b.th
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, th.Divider, clip.Rect(image.Rect(0, 0, size.X, max(1, gtx.Dp(1)))).Op())
	layout.Inset{Left: 12, Right: 52}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.W.Layout(gtx, body)
	})
	layout.Inset{Right: 20}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.E.Layout(gtx, func(gtx C) D {
			return th.Text(gtx, fmt.Sprint(i), th.Type.Small, th.Dim, kit.Regular)
		})
	})
	return D{Size: size}
}

func (b *bench) plainRow(gtx C, i int) D {
	th := b.th
	return b.rowFrame(gtx, i, func(gtx C) D {
		return th.Text(gtx, fmt.Sprintf("Row %d: plain text, no components", i), th.Type.Body, th.Fg, kit.Regular)
	})
}

func (b *bench) compRow(gtx C, i int) D {
	th := b.th
	if b.placeholder {
		return b.rowFrame(gtx, i, func(gtx C) D {
			s := image.Pt(gtx.Constraints.Max.X*3/5, gtx.Constraints.Max.Y-gtx.Dp(16))
			r := gtx.Dp(6)
			paint.FillShape(gtx.Ops, th.Panel, clip.UniformRRect(image.Rectangle{Max: s}, r).Op(gtx.Ops))
			return D{Size: s}
		})
	}
	id := func(name string) string { return fmt.Sprintf("%d:%s", i, name) }
	click := func(name string) *kit.Clickable { return b.clicks.Get(id(name)) }
	row := func(children ...layout.FlexChild) D {
		gtx := gtx
		gtx.Constraints.Min.Y = 0
		return kit.Row(gtx, children...)
	}
	gap := kit.Rigid(kit.Gap(8, 0))
	return b.rowFrame(gtx, i, func(gtx C) D {
		switch i % len(compHeights) {
		case 0:
			return th.Text(gtx, "BUTTONS (SIZE FOLLOWS THE RADIOS BELOW)", th.Type.Caption, th.Dim, kit.Bold)
		case 1:
			return row(
				kit.Rigid(func(gtx C) D { return ui.Button{ID: id("sec"), Text: "Secondary"}.Layout(gtx, th, click("sec")) }), gap,
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: id("pri"), Text: "Primary", Style: ui.Primary}.Layout(gtx, th, click("pri"))
				}), gap,
				kit.Rigid(func(gtx C) D {
					return ui.Button{ID: id("ghost"), Text: "Ghost", Style: ui.Ghost}.Layout(gtx, th, click("ghost"))
				}), gap,
				kit.Rigid(func(gtx C) D {
					return ui.IconButton{ID: id("icon"), Icon: kit.IconSun, Label: id("icon")}.Layout(gtx, th, click("icon"))
				}),
			)
		case 2:
			return row(
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: id("sw"), Label: "Switch", On: b.on[id("sw")]}.Layout(gtx, th, click("sw"))
				}), gap,
				kit.Rigid(func(gtx C) D {
					return ui.Checkbox{ID: id("cb"), Label: "Checkbox", Checked: b.on[id("cb")]}.Layout(gtx, th, click("cb"))
				}), gap,
				kit.Rigid(func(gtx C) D {
					return ui.Radio{ID: id("ra"), Label: "Radio", Selected: b.on[id("ra")]}.Layout(gtx, th, click("ra"))
				}),
			)
		case 3:
			return kit.Fixed(240, func(gtx C) D { return b.sliders.Get(id("sl")).layout(gtx, th, id("sl"), "Slider") })(gtx)
		case 4:
			return ui.Tabs{ID: id("tabs"), Labels: []string{"One", "Two", "Three"}, Selected: b.tab[id("tabs")]}.Layout(gtx, th, b.tabs.Get(id("tabs")))
		case 5:
			return row(
				kit.Rigid(func(gtx C) D { return th.Spinner(gtx, 18) }), gap,
				kit.Rigid(kit.Fixed(160, func(gtx C) D { return th.Progress(gtx, float32(i%10)/10) })), gap,
				kit.Rigid(func(gtx C) D { return ui.Badge{Status: kit.StatusInfo, Text: "Badge"}.Layout(gtx, th) }),
			)
		case 6:
			return th.Paragraph(gtx, "A paragraph that wraps. Text is shaped the first time it is laid out, "+
				"so rows entering the view for the first time cost more than rows coming back.", th.Muted)
		default:
			return row(
				kit.Rigid(func(gtx C) D { return ui.Kbd{Chord: "Ctrl+K"}.Layout(gtx, th) }), gap,
				kit.Rigid(func(gtx C) D { return th.Text(gtx, "Keyboard: command palette", th.Type.Body, th.Fg, kit.Regular) }),
			)
		}
	})
}

func main() {
	kit.NewApp("giokit-scrolltest", "giokit scroll bench", 1100, 760).Run(newBench().Frame)
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
