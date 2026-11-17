// Music is a no-audio app sketch for exercising media-oriented giokit APIs.
// go run ./examples/music [-shot /tmp/music.png]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"gioui.org/op"
	"gioui.org/unit"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kit/x"
	"github.com/hhhapz/giokit/kittest"
)

type track struct {
	id, title, artist, album string
	duration                 time.Duration
	art                      kit.Image
}

type music struct {
	th                              *kit.Theme
	tracks                          []track
	queue                           ui.ReorderListState
	queueItems                      []ui.Choice
	lyrics                          x.LyricsState
	seek                            slider
	play, next, prev, follow, theme kit.Clickable
	selectTrack                     kit.Pool[kit.Clickable]
	context                         kit.Pool[ui.MenuState]
	selected                        string
	playing                         bool
	position                        time.Duration
	lastTick                        time.Time
	status                          string
}

var demoLyrics = []x.LyricLine{
	{At: 0, Text: "A quiet room, a window full of light"},
	{At: 8 * time.Second, Text: "The city turns its colors into sound"},
	{At: 16 * time.Second, Text: "We find a rhythm in the passing lines"},
	{At: 25 * time.Second, Text: "And let the morning carry us around"},
	{At: 34 * time.Second, Text: "Hold on to what the daylight brings"},
	{At: 44 * time.Second, Text: "Follow every note until it sings"},
	{At: 55 * time.Second, Text: "The streets are moving underneath our feet"},
	{At: 65 * time.Second, Text: "A little time is all we really need"},
	{At: 76 * time.Second, Text: "Here in the spaces between every beat"},
	{At: 88 * time.Second, Text: "We start again, and let the song repeat"},
}

func newMusic() *music {
	m := &music{th: kit.Dark(), selected: "daylight", position: 25 * time.Second}
	m.tracks = []track{
		{id: "daylight", title: "Daylight Lines", artist: "The Still Room", album: "After the Rain", duration: 3*time.Minute + 42*time.Second},
		{id: "arcade", title: "Arcade Lights", artist: "Night Transit", album: "Blue Hour", duration: 4*time.Minute + 11*time.Second},
		{id: "signal", title: "Signals at Dawn", artist: "Northline", album: "Open Windows", duration: 3*time.Minute + 8*time.Second},
		{id: "echo", title: "Echoes in Glass", artist: "The Still Room", album: "After the Rain", duration: 5*time.Minute + 2*time.Second},
		{id: "gold", title: "Soft Gold", artist: "Luna Vale", album: "Horizon", duration: 2*time.Minute + 59*time.Second},
		{id: "steps", title: "Small Steps", artist: "Northline", album: "Open Windows", duration: 3*time.Minute + 37*time.Second},
	}
	colors := [][2]color.NRGBA{
		{{R: 59, G: 79, B: 112, A: 255}, {R: 216, G: 156, B: 122, A: 255}},
		{{R: 45, G: 47, B: 94, A: 255}, {R: 183, G: 82, B: 136, A: 255}},
		{{R: 31, G: 92, B: 92, A: 255}, {R: 195, G: 195, B: 119, A: 255}},
		{{R: 66, G: 75, B: 83, A: 255}, {R: 190, G: 139, B: 99, A: 255}},
		{{R: 122, G: 76, B: 72, A: 255}, {R: 245, G: 192, B: 111, A: 255}},
		{{R: 49, G: 75, B: 118, A: 255}, {R: 143, G: 191, B: 210, A: 255}},
	}
	for i := range m.tracks {
		m.tracks[i].art.Set(cover(colors[i][0], colors[i][1]))
		m.queueItems = append(m.queueItems, ui.Choice{ID: m.tracks[i].id, Label: m.tracks[i].title})
	}
	m.lyrics.Follow = true
	m.seek.Value = float32(m.position) / float32(m.current().duration)
	return m
}

func cover(a, b color.NRGBA) image.Image {
	const size = 160
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := float32(x+y) / (2 * size)
			c := kit.LerpColor(a, b, p)
			dx, dy := float32(x-80), float32(y-80)
			if dx*dx+dy*dy < 46*46 {
				c = kit.LerpColor(c, b, 0.45)
			}
			im.SetNRGBA(x, y, c)
		}
	}
	return im
}

func (m *music) current() *track {
	for i := range m.tracks {
		if m.tracks[i].id == m.selected {
			return &m.tracks[i]
		}
	}
	return &m.tracks[0]
}

func (m *music) choose(id string) {
	m.selected = id
	m.position = 0
	m.seek.Value = 0
}

func (m *music) step(direction int) {
	for i, c := range m.queueItems {
		if c.ID == m.selected {
			m.choose(m.queueItems[(i+direction+len(m.queueItems))%len(m.queueItems)].ID)
			return
		}
	}
}

func (m *music) handle(gtx kit.C) {
	if from, to, ok := m.queue.Update(gtx); ok && ui.Move(m.queueItems, from, to) {
		m.status = fmt.Sprintf("Queue moved from %d to %d", from+1, to+1)
	}
	if m.play.Clicked(gtx) {
		m.playing = !m.playing
		m.lastTick = gtx.Now
	}
	if m.next.Clicked(gtx) {
		m.step(1)
	}
	if m.prev.Clicked(gtx) {
		m.step(-1)
	}
	if m.follow.Clicked(gtx) {
		m.lyrics.Follow = !m.lyrics.Follow
	}
	if m.theme.Clicked(gtx) {
		if m.th.Dark {
			m.th = kit.Light()
		} else {
			m.th = kit.Dark()
		}
	}
	for _, c := range m.queueItems {
		if m.selectTrack.Get(c.ID).Clicked(gtx) {
			m.choose(c.ID)
		}
		if action, ok := m.context.Get(c.ID).Update(gtx); ok {
			switch action {
			case "play":
				m.choose(c.ID)
				m.playing = true
			case "remove":
				for i := range m.queueItems {
					if m.queueItems[i].ID == c.ID {
						m.queueItems = append(m.queueItems[:i], m.queueItems[i+1:]...)
						break
					}
				}
				if m.selected == c.ID && len(m.queueItems) > 0 {
					m.choose(m.queueItems[0].ID)
				}
			}
		}
	}
	if m.playing && !m.lastTick.IsZero() {
		delta := gtx.Now.Sub(m.lastTick)
		if delta > 0 && delta < time.Second {
			m.position += delta
		}
		if m.position >= m.current().duration {
			m.step(1)
		}
		m.seek.Value = float32(m.position) / float32(m.current().duration)
	}
	m.lastTick = gtx.Now
}

func (m *music) Frame(gtx kit.C) {
	m.handle(gtx)
	seek := m.seek.Value
	m.layout(gtx)
	if m.seek.Value != seek {
		m.position = time.Duration(float32(m.current().duration) * m.seek.Value)
		kit.Notify(gtx)
	}
	if m.playing {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(100 * time.Millisecond)})
	}
}

func (m *music) layout(gtx kit.C) kit.D {
	th := m.th
	return kit.Fill(gtx, th.Bg, func(gtx kit.C) kit.D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx kit.C) kit.D { return m.header(gtx) }),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Flexed(1, func(gtx kit.C) kit.D {
				if gtx.Constraints.Max.X < gtx.Dp(780) {
					return m.compact(gtx)
				}
				return kit.Row(gtx,
					kit.Flexed(3, m.hero),
					kit.Rigid(kit.VLine(th.Border)),
					kit.Flexed(2, m.queuePane),
				)
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Rigid(m.transport),
		)
	})
}

func (m *music) header(gtx kit.C) kit.D {
	th := m.th
	return kit.Pad(10, 20, func(gtx kit.C) kit.D {
		return kit.Row(gtx,
			kit.Rigid(func(gtx kit.C) kit.D { return th.Text(gtx, "Music Lab", th.Type.Title, th.Fg, kit.Bold) }),
			kit.Rigid(kit.Gap(12, 0)),
			kit.Rigid(func(gtx kit.C) kit.D { return ui.Badge{Status: kit.StatusInfo, Text: "UI prototype"}.Layout(gtx, th) }),
			kit.Flexed(1, kit.Grow),
			kit.Rigid(func(gtx kit.C) kit.D {
				return ui.Button{ID: "music:theme", Text: "Switch theme", Style: ui.Ghost}.Layout(gtx, th, &m.theme)
			}),
		)
	})(gtx)
}

func (m *music) hero(gtx kit.C) kit.D {
	th, cur := m.th, m.current()
	return kit.Pad(20, 24, func(gtx kit.C) kit.D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx kit.C) kit.D { return th.Caption(gtx, "Now playing") }),
			kit.Rigid(kit.Gap(0, 14)),
			kit.Rigid(func(gtx kit.C) kit.D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx kit.C) kit.D {
						return ui.Artwork{ID: "music:art", Label: cur.album + " album art", Size: 132}.Layout(gtx, th, &cur.art)
					}),
					kit.Rigid(kit.Gap(18, 0)),
					kit.Flexed(1, func(gtx kit.C) kit.D {
						return kit.Col(gtx,
							kit.Rigid(func(gtx kit.C) kit.D { return th.Heading(gtx, cur.title) }),
							kit.Rigid(kit.Gap(0, 5)),
							kit.Rigid(func(gtx kit.C) kit.D { return th.Text(gtx, cur.artist, th.Type.Text, th.Muted, kit.Regular) }),
							kit.Rigid(kit.Gap(0, 5)),
							kit.Rigid(func(gtx kit.C) kit.D { return th.Text(gtx, cur.album, th.Type.Small, th.Dim, kit.Regular) }),
						)
					}),
				)
			}),
			kit.Rigid(kit.Gap(0, 24)),
			kit.Rigid(func(gtx kit.C) kit.D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx kit.C) kit.D { return th.Caption(gtx, "Follow-along lyrics") }),
					kit.Flexed(1, kit.Grow),
					kit.Rigid(func(gtx kit.C) kit.D {
						label := "Follow"
						if m.lyrics.Follow {
							label = "Following"
						}
						return ui.Button{ID: "music:follow", Text: label, Style: ui.Ghost}.Layout(gtx, th, &m.follow)
					}),
				)
			}),
			kit.Rigid(kit.Gap(0, 8)),
			kit.Flexed(1, func(gtx kit.C) kit.D {
				return kit.Surface{Bg: th.Panel, Radius: th.Radius}.Layout(gtx, func(gtx kit.C) kit.D {
					gtx.Constraints.Min = gtx.Constraints.Max
					return x.Lyrics{ID: "music:lyrics", Lines: demoLyrics, At: m.position}.Layout(gtx, th, &m.lyrics)
				})
			}),
		)
	})(gtx)
}

func (m *music) queuePane(gtx kit.C) kit.D {
	th := m.th
	return kit.Pad(16, 14, func(gtx kit.C) kit.D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx kit.C) kit.D { return th.Heading(gtx, "Up next") }),
			kit.Rigid(kit.Gap(0, 4)),
			kit.Rigid(func(gtx kit.C) kit.D {
				hint := m.status
				if hint == "" {
					hint = "Drag a handle or use the arrow buttons to reorder"
				}
				return th.Text(gtx, hint, th.Type.Small, th.Muted, kit.Regular)
			}),
			kit.Rigid(kit.Gap(0, 10)),
			kit.Flexed(1, func(gtx kit.C) kit.D {
				return ui.ReorderList{ID: "music:queue", Items: m.queueItems, Item: m.queueItem}.Layout(gtx, th, &m.queue)
			}),
		)
	})(gtx)
}

func (m *music) queueItem(gtx kit.C, c ui.Choice) kit.D {
	th := m.th
	var tr *track
	for i := range m.tracks {
		if m.tracks[i].id == c.ID {
			tr = &m.tracks[i]
			break
		}
	}
	if tr == nil {
		return kit.D{}
	}
	return ui.ContextMenu{ID: "music:menu:" + c.ID, Items: []ui.MenuItem{{ID: "play", Label: "Play this track"}, {ID: "remove", Label: "Remove from queue"}}, Content: func(gtx kit.C) kit.D {
		return m.selectTrack.Get(c.ID).Press(gtx, "music:track:"+c.ID, func(gtx kit.C, st kit.State) kit.D {
			bg := th.Panel
			if c.ID == m.selected {
				bg = th.AccentBg
			} else if st.Hovered {
				bg = th.Surface
			}
			return kit.Surface{Bg: bg, Radius: th.Radius}.Layout(gtx, kit.Pad(5, 8, func(gtx kit.C) kit.D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx kit.C) kit.D {
						return ui.Artwork{ID: "music:cover:" + c.ID, Label: tr.album + " cover", Size: 38}.Layout(gtx, th, &tr.art)
					}),
					kit.Rigid(kit.Gap(8, 0)),
					kit.Flexed(1, func(gtx kit.C) kit.D {
						return th.StyledText(gtx,
							kit.TextSpan{Text: tr.title + "\n", Strong: true},
							kit.TextSpan{Text: tr.artist, Size: th.Type.Small, Muted: true},
						)
					}),
				)
			}))
		})
	}}.Layout(gtx, th, m.context.Get(c.ID))
}

func (m *music) compact(gtx kit.C) kit.D {
	th := m.th
	return kit.Col(gtx,
		kit.Flexed(1, m.hero),
		kit.Rigid(kit.HLine(th.Border)),
		kit.Flexed(1, m.queuePane),
	)
}

func (m *music) transport(gtx kit.C) kit.D {
	th := m.th
	cur := m.current()
	return kit.Pad(10, 18, func(gtx kit.C) kit.D {
		return kit.Col(gtx,
			kit.Rigid(func(gtx kit.C) kit.D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx kit.C) kit.D {
						return ui.IconButton{ID: "music:previous", Icon: kit.IconChevronLeft, Label: "Previous track"}.Layout(gtx, th, &m.prev)
					}),
					kit.Rigid(kit.Gap(8, 0)),
					kit.Rigid(func(gtx kit.C) kit.D {
						label := "Play"
						if m.playing {
							label = "Pause"
						}
						return ui.Button{ID: "music:play", Text: label, Style: ui.Solid}.Layout(gtx, th, &m.play)
					}),
					kit.Rigid(kit.Gap(8, 0)),
					kit.Rigid(func(gtx kit.C) kit.D {
						return ui.IconButton{ID: "music:next", Icon: kit.IconChevronRight, Label: "Next track"}.Layout(gtx, th, &m.next)
					}),
					kit.Rigid(kit.Gap(16, 0)),
					kit.Flexed(1, func(gtx kit.C) kit.D { return m.seek.layout(gtx, th, "music:seek", "Seek") }),
					kit.Rigid(kit.Gap(10, 0)),
					kit.Rigid(func(gtx kit.C) kit.D {
						return th.Text(gtx, clock(m.position)+" / "+clock(cur.duration), th.Type.Small, th.Muted, kit.Regular)
					}),
				)
			}),
		)
	})(gtx)
}

func clock(d time.Duration) string { s := int(d.Seconds()); return fmt.Sprintf("%d:%02d", s/60, s%60) }

func main() {
	shot := flag.String("shot", "", "render offscreen to a PNG and exit")
	width := flag.Int("width", 1120, "window or screenshot width")
	height := flag.Int("height", 740, "window or screenshot height")
	flag.Parse()
	m := newMusic()
	if *shot != "" {
		h, err := kittest.New(*width, *height, 2, m.Frame)
		if err == nil {
			defer h.Release()
			err = h.Save(*shot)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	kit.NewApp("giokit-music", "Music Lab", unit.Dp(*width), unit.Dp(*height)).Run(m.Frame)
}

// slider pairs a ui.Slider's value with its state, for sliders that keep
// their own value.
type slider struct {
	Value float32
	state ui.SliderState
}

func (s *slider) layout(gtx kit.C, th *kit.Theme, id, label string) kit.D {
	if v, ok := s.state.Update(gtx); ok {
		s.Value = v
	}
	return ui.Slider{ID: id, Label: label, Value: s.Value}.Layout(gtx, th, &s.state)
}
