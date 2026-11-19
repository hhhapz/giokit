package main

import (
	"fmt"
	"image"
	"slices"
	"time"

	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kit/x/video"
)

// mediaState is the Video demo: a moving test pattern pushed through a
// paint.Stream, the way a decoder would push frames.
type mediaState struct {
	vstream  *paint.Stream
	video    video.State
	fit      int
	fitTabs  ui.TabsState
	play     kit.Clickable
	playing  bool
	pool     []*image.RGBA // frames handed back by the stream
	frames   int
	lastTick time.Time
	shown    bool // the demo was laid out last frame
}

// patternInterval is the test pattern's frame period (30 fps).
const patternInterval = time.Second / 30

var fitNames = []string{"Contain", "Cover", "Fill", "1:1"}

func (g *gallery) initMedia() {
	g.media.vstream = paint.NewStream()
	g.media.playing = true
	g.submitPattern(0)
}

func (g *gallery) handleMedia(gtx C) {
	if id, ok := g.media.fitTabs.Update(gtx); ok {
		g.media.fit = slices.Index(fitNames, id)
	}
	if g.media.play.Clicked(gtx) {
		g.media.playing = !g.media.playing
	}
	// Produce the next pattern frame while the demo is on screen; its
	// kit.Loop keeps these frames coming.
	if g.media.playing && g.media.shown && gtx.Now.Sub(g.media.lastTick) >= patternInterval {
		g.media.lastTick = gtx.Now
		g.media.frames++
		g.submitPattern(g.media.frames)
	}
	g.media.shown = false
}

// submitPattern draws frame n of the pattern into a pooled buffer and
// submits it. A real producer does this on its own goroutine.
func (g *gallery) submitPattern(n int) {
	const w, h = 320, 180
	var img *image.RGBA
	if k := len(g.media.pool); k > 0 {
		img, g.media.pool = g.media.pool[k-1], g.media.pool[:k-1]
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
	g.media.vstream.Submit(paint.StreamFrame{Image: img, Release: func() { g.media.pool = append(g.media.pool, img) }})
}

func (g *gallery) demoVideo(gtx C) D {
	th := g.th
	g.media.shown = true
	if g.media.playing {
		kit.Loop(gtx, time.Second) // keep frames coming while visible
	}
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return kit.Row(gtx,
				kit.Rigid(func(gtx C) D {
					return ui.Tabs{ID: "video-fit", Items: choices(fitNames), Selected: fitNames[g.media.fit], Style: ui.TabsSegmented}.Layout(gtx, th, &g.media.fitTabs)
				}),
				kit.Rigid(kit.Gap(kit.Space3, 0)),
				kit.Rigid(func(gtx C) D {
					return ui.Switch{ID: "video-play", Label: "Play", On: g.media.playing}.Layout(gtx, th, &g.media.play)
				}),
				kit.Flexed(1, kit.Grow),
				kit.Rigid(func(gtx C) D {
					sz := g.media.vstream.Size()
					return th.Text(gtx, fmt.Sprintf("%d×%d · frame %d", sz.X, sz.Y, g.media.frames), th.Type.Small, th.Muted, kit.Regular)
				}),
			)
		}),
		kit.Rigid(kit.Gap(0, kit.Space3)),
		kit.Rigid(func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(220)
			gtx.Constraints.Min = gtx.Constraints.Max
			return kit.Fill(gtx, th.Surface, func(gtx C) D {
				return video.Video{ID: "video-screen", Stream: g.media.vstream, Fit: video.Fit(g.media.fit), Label: "Test pattern"}.Layout(gtx, th, &g.media.video)
			})
		}),
	)
}
