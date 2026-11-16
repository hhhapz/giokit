package main

import (
	"testing"
	"time"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kittest"
)

func TestMusicInteractions(t *testing.T) {
	m := newMusic()
	h := kittest.NewT(t, 1120, 740, m.Frame)
	h.Theme = m.th
	h.ExpectID("music:art")
	h.ExpectText("Daylight Lines")
	h.Hover("music:track:arcade")
	h.ClickID("music:track:arcade")
	if m.selected != "arcade" {
		t.Fatalf("selected = %q", m.selected)
	}
	h.ClickID("music:play")
	if !m.playing {
		t.Fatal("play control did not start mock timeline")
	}
	h.Wait(time.Second)
	if m.position <= 0 {
		t.Fatal("timeline did not advance")
	}
	h.ClickID("music:play")
	h.ClickID("music:queue/arcade/up")
	if m.queue.Items[0].ID != "arcade" {
		t.Fatalf("queue after up: %+v", m.queue.Items)
	}
	h.DragID("music:queue/arcade/drag", 0, 112)
	if m.queue.Items[2].ID != "arcade" {
		t.Fatalf("queue after drag: %+v", m.queue.Items)
	}
	h.ClickID("music:follow")
	if m.lyrics.Follow {
		t.Fatal("follow did not toggle")
	}
	h.ClickID("music:theme")
	h.Theme = m.th
	if m.th.Dark {
		t.Fatal("theme did not change")
	}
	h.ExpectClean()
	h.ExpectTabReachable()
}

func TestMusicNarrow(t *testing.T) {
	m := newMusic()
	h := kittest.NewT(t, 590, 860, m.Frame)
	h.Theme = m.th
	h.ExpectID("music:art")
	h.ExpectID("music:queue")
	h.ScrollID("music:lyrics", 140)
	if m.lyrics.Follow {
		t.Fatal("manual lyric scroll did not pause follow")
	}
	h.ExpectClean()
}

func TestActiveLyricBoundary(t *testing.T) {
	if n := activeAt(-time.Second); n != -1 {
		t.Fatalf("before first line = %d", n)
	}
	if n := activeAt(25 * time.Second); n != 3 {
		t.Fatalf("exact timestamp = %d", n)
	}
}

func activeAt(at time.Duration) int { return kit.ActiveLyric(demoLyrics, at) }
