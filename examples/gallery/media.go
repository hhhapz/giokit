package main

import (
	"fmt"
	"image"
	"time"

	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// mediaState is the Video demo: a moving test pattern pushed through a
// paint.Stream, the way a decoder would push frames.
type mediaState struct {
	vstream  *paint.Stream
	video    ui.VideoState
	fit      int
	fitTabs  ui.TabsState
	play     kit.Clickable
	playing  bool
	pool     []*image.RGBA // frames handed back by the stream
	frames   int
	lastTick time.Time
}

var fitNames = []string{"Contain", "Cover", "Fill", "1:1"}

func (g *gallery) initMedia() {
	g.vstream = paint.NewStream()
	g.playing = true
	g.submitPattern(0)
}

func (g *gallery) handleMedia(gtx C) {
	if i, ok := g.fitTabs.Update(gtx); ok {
		g.fit = i
	}
	if g.play.Clicked(gtx) {
		g.playing = !g.playing
	}
}

// submitPattern draws frame n of the pattern into a pooled buffer and
// submits it. A real producer does this on its own goroutine.
func (g *gallery) submitPattern(n int) {
	const w, h = 320, 180
	var img *image.RGBA
	if k := len(g.pool); k > 0 {
		img, g.pool = g.pool[k-1], g.pool[:k-1]
	} else {
		img = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	bars := [...][3]uint8{{235, 235, 235}, {235, 235, 16}, {16, 235, 235}, {16, 235, 16}, {235, 16, 235}, {235, 16, 16}, {16, 16, 235}}
	sweep := n * 4 % w
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := bars[x*len(bars)/w]
			if y > h*3/4 { // a gray ramp with a moving marker
				v := uint8(x * 255 / w)
				c = [3]uint8{v, v, v}
				if x >= sweep && x < sweep+6 {
					c = [3]uint8{255, 64, 0}
				}
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c[0], c[1], c[2], 255
		}
	}
	g.vstream.Submit(paint.StreamFrame{Image: img, Release: func() { g.pool = append(g.pool, img) }})
}

func (g *gallery) demoVideo(gtx C) D {
	th := g.th
	if g.playing {
		kit.Loop(gtx, time.Second) // keep frames coming while visible
		if gtx.Now.Sub(g.lastTick) >= time.Second/30 {
			g.lastTick = gtx.Now
			g.frames++
			g.submitPattern(g.frames)
		}
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D {
					return ui.Tabs{ID: "video-fit", Labels: fitNames, Selected: g.fit, Style: ui.TabsSegmented}.Layout(gtx, th, &g.fitTabs)
				}),
				kit.Rigid(kit.Gap(kit.Space3, 0)),
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: "video-play", Label: "Play", On: g.playing}.Layout(gtx, th, &g.play)
				}),
				kit.Flexed(1, kit.Grow),
				kit.Rigid(func(gtx C) D {
					sz := g.vstream.Size()
					return th.Text(gtx, fmt.Sprintf("%d×%d · frame %d", sz.X, sz.Y, g.frames), th.Type.Small, th.Muted, kit.Regular)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, kit.Space3)),
		kit.Rigid(func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(220)
			gtx.Constraints.Min = gtx.Constraints.Max
			return kit.Fill(gtx, th.Surface, func(gtx C) D {
				return ui.Video{ID: "video-screen", Stream: g.vstream, Fit: ui.Fit(g.fit), Label: "Test pattern"}.Layout(gtx, th, &g.video)
			})
		}),
	)
}
