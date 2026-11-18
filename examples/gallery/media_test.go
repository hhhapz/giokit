package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
	"github.com/hhhapz/giokit/kit/x"
	"github.com/hhhapz/giokit/kittest"
)

// Gallery coverage for the media primitives used by examples/music.
func TestMediaPrimitives(t *testing.T) {
	th := kit.Dark()
	var art kit.Image
	art.Set(image.NewNRGBA(image.Rect(0, 0, 20, 20)))
	var rs ui.ReorderListState
	r := struct{ Items []ui.Choice }{[]ui.Choice{{ID: "a", Label: "First"}, {ID: "b", Label: "Second"}, {ID: "c", Label: "Third"}}}
	l := x.LyricsState{Follow: true}
	frame := func(gtx C) {
		if from, to, ok := rs.Update(gtx); ok {
			ui.Move(r.Items, from, to)
		}
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return ui.Artwork{ID: "media:art", Label: "Album cover", Size: 60}.Layout(gtx, th, &art)
			}),
			kit.Rigid(func(gtx C) D {
				return th.StyledText(gtx, kit.TextSpan{Text: "Featured ", Strong: true}, kit.TextSpan{Text: "track", Muted: true})
			}),
			kit.Flexed(1, func(gtx C) D {
				return ui.ReorderList{ID: "media:queue", Items: r.Items, Item: func(gtx C, c ui.Choice) D { return th.Text(gtx, c.Label, th.Type.Body, th.Fg, kit.Regular) }}.Layout(gtx, th, &rs)
			}),
			kit.Flexed(1, func(gtx C) D {
				return x.Lyrics{ID: "media:lyrics", Lines: []x.LyricLine{{At: 0, Text: "First line"}, {At: time.Second, Text: "Second line"}}, At: 2 * time.Second}.Layout(gtx, th, &l)
			}),
		)
	}
	h := kittest.NewT(t, 420, 430, frame)
	h.Theme = th
	h.ExpectID("media:art")
	h.ExpectText("Featured track")
	h.Hover("media:queue/a/up")
	h.ClickID("media:queue/b/up")
	if r.Items[0].ID != "b" {
		t.Fatalf("order: %+v", r.Items)
	}
	h.DragID("media:queue/b/drag", 0, 56)
	if r.Items[1].ID != "b" {
		t.Fatalf("drag order: %+v", r.Items)
	}
	for i := 0; i < 20 && h.FocusedID() != "media:queue/b/drag"; i++ {
		h.Tab(false)
	}
	if h.FocusedID() != "media:queue/b/drag" {
		t.Fatal("reorder handle is not keyboard reachable")
	}
	h.Key(key.NameDownArrow, 0)
	if r.Items[2].ID != "b" {
		t.Fatalf("keyboard order: %+v", r.Items)
	}
	h.ExpectText("Second line")
	h.ExpectClean()
	h.ExpectTabReachable()
}

func TestAnchoredListKeepsVisibleItemAfterPrepend(t *testing.T) {
	th := kit.Dark()
	var feed kit.AnchoredList
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = fmt.Sprintf("item-%02d", i)
	}
	frame := func(gtx C) {
		th.AnchoredList(gtx, &feed, "media:feed", ids, func(gtx C, i int) D {
			return kit.Box(gtx, 40, kit.Surface{Bg: th.Panel}, func(gtx C) D { return th.Text(gtx, ids[i], th.Type.Body, th.Fg, kit.Regular) })
		})
	}
	h := kittest.NewT(t, 180, 180, frame)
	h.Theme = th
	h.ScrollID("media:feed", 180)
	first := feed.List.Position.First
	if first == 0 {
		t.Fatal("list did not scroll")
	}
	anchor, offset := ids[first], feed.List.Position.Offset
	ids = append([]string{"new"}, ids...)
	h.Settle()
	if ids[feed.List.Position.First] != anchor || feed.List.Position.Offset != offset {
		t.Fatalf("anchor moved: got %s at %d, offset %d; want %s offset %d", ids[feed.List.Position.First], feed.List.Position.First, feed.List.Position.Offset, anchor, offset)
	}
	h.ExpectClean()
}

func TestVideoDemo(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "video")
	r, ok := h.Find("video-screen")
	if !ok {
		t.Fatal("video not on the page")
	}
	// The first bar is near-white: the stream's frame reached the screen.
	s := min(float64(r.Dx())/320, float64(r.Dy())/180)
	fw := int(320 * s)
	c := h.Pixel(image.Pt(r.Min.X+(r.Dx()-fw)/2+fw/14, r.Min.Y+r.Dy()/3))
	if c.R < 200 || c.G < 200 || c.B < 200 {
		t.Fatalf("first bar = %v, want near white", c)
	}
	before := g.media.frames
	h.Wait(200 * time.Millisecond)
	if g.media.frames == before {
		t.Fatal("pattern did not advance")
	}
	h.ClickID("video-play")
	if g.media.playing {
		t.Fatal("play switch did not pause")
	}
	h.ClickID("video-fit/Fill")
	if got := g.media.video.Content(); got.Dx() != r.Dx() || got.Dy() != r.Dy() {
		t.Fatalf("Fill content %v, want the whole %v", got, r.Size())
	}
	h.ExpectClean()
}
