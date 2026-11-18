package main

import (
	"testing"
	"time"

	"github.com/hhhapz/giokit/kit/x"
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
	if m.queueItems[0].ID != "arcade" {
		t.Fatalf("queue after up: %+v", m.queueItems)
	}
	h.DragID("music:queue/arcade/drag", 0, 112)
	if m.queueItems[2].ID != "arcade" {
		t.Fatalf("queue after drag: %+v", m.queueItems)
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

func activeAt(at time.Duration) int { return x.ActiveLyric(demoLyrics, at) }

// Seeking applies in the handle pass: the position follows the slider in
// the same frame. The transport controls announce what they do.
func TestMusicSeekAndLabels(t *testing.T) {
	m := newMusic()
	h := kittest.NewT(t, 1120, 740, m.Frame)
	h.Theme = m.th
	h.ClickID("music:seek")
	want := time.Duration(float32(m.current().duration) * m.seek.Value)
	if m.seek.Value < 0.4 || m.seek.Value > 0.6 || m.position != want {
		t.Fatalf("seek to the middle: value %v, position %v, want %v", m.seek.Value, m.position, want)
	}
	labels := map[string]string{}
	var walk func([]kittest.A11yNode)
	walk = func(nodes []kittest.A11yNode) {
		for _, n := range nodes {
			labels[n.ID] = n.Label
			walk(n.Children)
		}
	}
	walk(h.A11yTree())
	for id, want := range map[string]string{
		"music:previous": "Previous track",
		"music:next":     "Next track",
		"music:seek":     "Seek",
	} {
		if got := labels[id]; got != want {
			t.Errorf("%s: label %q, want %q", id, got, want)
		}
	}
}

// Removing a track from its context menu takes it out of the queue, keeps
// the rest in order, and moves the selection off it.
func TestMusicRemoveFromQueue(t *testing.T) {
	m := newMusic()
	h := kittest.NewT(t, 1120, 740, m.Frame)
	h.Theme = m.th
	before := len(m.queueItems)
	removed := m.queueItems[1].ID
	next := m.queueItems[2].ID
	h.ClickID("music:track:" + removed)
	h.RightClickID("music:track:" + removed)
	h.ClickID("music:menu:" + removed + "/remove")
	if len(m.queueItems) != before-1 || m.queueItems[1].ID != next {
		t.Fatalf("queue after removing %q: %+v", removed, m.queueItems)
	}
	if m.selected == removed {
		t.Fatalf("the removed track %q is still selected", removed)
	}
	h.ExpectClean()
}
