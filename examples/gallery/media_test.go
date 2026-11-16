package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

// Gallery coverage for the media primitives used by examples/music.
func TestMediaPrimitives(t *testing.T) {
	th := kit.Dark()
	var art kit.Image
	art.Set(image.NewNRGBA(image.Rect(0, 0, 20, 20)))
	r := kit.ReorderList{Items: []kit.Choice{{ID: "a", Label: "First"}, {ID: "b", Label: "Second"}, {ID: "c", Label: "Third"}}}
	l := kit.Lyrics{Follow: true}
	frame := func(gtx C) {
		kit.Col(gtx,
			kit.Rigid(func(gtx C) D { return th.Artwork(gtx, &art, "media:art", "Album cover", 60) }),
			kit.Rigid(func(gtx C) D {
				return th.StyledText(gtx, kit.TextSpan{Text: "Featured ", Strong: true}, kit.TextSpan{Text: "track", Muted: true})
			}),
			kit.Flexed(1, func(gtx C) D {
				return th.ReorderList(gtx, &r, "media:queue", func(gtx C, c kit.Choice) D { return th.Text(gtx, c.Label, th.Type.Body, th.Fg, kit.Regular) })
			}),
			kit.Flexed(1, func(gtx C) D {
				return th.Lyrics(gtx, &l, "media:lyrics", []kit.LyricLine{{At: 0, Text: "First line"}, {At: time.Second, Text: "Second line"}}, 2*time.Second)
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
