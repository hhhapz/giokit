package audio

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// frames makes n stereo frames whose samples count up from start.
func frames(start, n int) []byte {
	b := make([]byte, 0, n*4)
	for i := range n {
		v := byte(start + i)
		b = append(b, v, 0, v, 0)
	}
	return b
}

func TestQueueWaitsForTarget(t *testing.T) {
	q := newQueue(4, 40, 120) // start at 10 frames, cap 30
	out := make([]byte, 16)
	q.write(frames(0, 5))
	q.fill(out)
	if !bytes.Equal(out, make([]byte, 16)) {
		t.Fatal("played before the target was queued")
	}
	q.write(frames(5, 5))
	q.fill(out)
	if !bytes.Equal(out, frames(0, 4)) {
		t.Fatalf("got %v", out)
	}
}

func TestQueueUnderrunRefills(t *testing.T) {
	q := newQueue(4, 16, 64)
	q.write(frames(0, 4))
	out := make([]byte, 24) // wants 6 frames, 4 queued
	q.fill(out)
	if !bytes.Equal(out[:16], frames(0, 4)) || !bytes.Equal(out[16:], make([]byte, 8)) {
		t.Fatalf("underrun: %v", out)
	}
	q.write(frames(4, 2)) // below target again: silence until refilled
	q.fill(out[:8])
	if !bytes.Equal(out[:8], make([]byte, 8)) {
		t.Fatal("resumed before refilling to the target")
	}
}

func TestQueueDropsBacklog(t *testing.T) {
	q := newQueue(4, 16, 32) // target 4 frames, cap 8
	q.write(frames(0, 6))
	q.write(frames(6, 6)) // 12 frames > cap: back to the newest 4
	if q.len != 16 {
		t.Fatalf("queued %d bytes, want 16", q.len)
	}
	out := make([]byte, 16)
	q.fill(out)
	if !bytes.Equal(out, frames(8, 4)) {
		t.Fatalf("kept %v, want the newest frames", out)
	}
	q.write(frames(0, 100)) // one write bigger than the cap
	if q.len != 16 {
		t.Fatalf("huge write queued %d bytes", q.len)
	}
}

func TestQueueWrapsAround(t *testing.T) {
	q := newQueue(4, 4, 20)
	out := make([]byte, 12)
	for i := 0; i < 50; i += 3 {
		q.write(frames(i, 3))
		q.fill(out)
		if !bytes.Equal(out, frames(i, 3)) {
			t.Fatalf("round %d: %v", i, out)
		}
	}
}

// The default output drains what is written. It plays silence (zero
// samples): inaudible. Skipped without a sound system.
func TestPlayerDrains(t *testing.T) {
	p, err := New(Config{SampleRate: 48000, Channels: 2})
	if err != nil {
		t.Skipf("no audio output here: %v", err)
	}
	defer p.Close()
	p.Write(make([]byte, 4800*4)) // 100 ms of silence
	deadline := time.Now().Add(2 * time.Second)
	for p.Buffered() >= 4800*4 {
		if time.Now().After(deadline) {
			t.Fatal("the device never drained the queue")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A closed player drops writes (audit 16, CP-01).
func TestWriteAfterClose(t *testing.T) {
	p := &Player{q: newQueue(4, 16, 64)}
	p.Close()
	p.Write(make([]byte, 16))
	if n := p.Buffered(); n != 0 {
		t.Fatalf("closed player queued %d bytes", n)
	}
}

// Closing from two goroutines at once released the device twice and
// crashed (audit 16, CP-01).
func TestConcurrentClose(t *testing.T) {
	p, err := New(Config{SampleRate: 48000, Channels: 2})
	if err != nil {
		t.Skip(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			p.Close()
			p.Write(make([]byte, 64))
		}()
	}
	close(start)
	wg.Wait()
	if n := p.Buffered(); n != 0 {
		t.Fatalf("closed player queued %d bytes", n)
	}
}
