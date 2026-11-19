// Package audio plays a live stream of PCM through the system's default
// output, using miniaudio (github.com/gen2brain/malgo). miniaudio loads
// the sound system at run time (PipeWire or PulseAudio, then ALSA or JACK
// on Linux; the native APIs elsewhere), so a program using it links no
// audio library and needs none installed beyond what the desktop has.
//
//	p, err := audio.New(audio.Config{SampleRate: 48000, Channels: 2})
//	...
//	p.Write(pcm) // as packets arrive, from any goroutine
//	p.Close()
package audio

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// Config describes the stream and its buffering.
type Config struct {
	SampleRate int // frames per second, such as 48000
	Channels   int // interleaved channels, such as 2

	// Latency is how much audio is queued before playback starts, and
	// again after running dry: it absorbs packets arriving in bursts.
	// Zero is DefaultLatency.
	Latency time.Duration
	// MaxLatency caps the queue: when a live source runs ahead of the
	// output (their clocks drift; a stall delivers a backlog), the oldest
	// audio is dropped back to Latency rather than played late. Zero is
	// three times Latency.
	MaxLatency time.Duration
}

// DefaultLatency is the queue Config.Latency defaults to.
const DefaultLatency = 60 * time.Millisecond

// bytesPerSample is signed 16-bit PCM.
const bytesPerSample = 2

// Player plays interleaved signed 16-bit little-endian PCM. Its methods
// are safe from any goroutine, Close included.
type Player struct {
	ctx   *malgo.AllocatedContext
	dev   *malgo.Device
	close sync.Once // the device and context are released once, by Close

	mu     sync.Mutex // guards q and closed; the device callback takes it
	q      queue
	closed bool
}

// New opens the default output device for cfg's stream and starts it; it
// plays silence until Write queues Latency of audio.
func New(cfg Config) (*Player, error) {
	if cfg.SampleRate <= 0 || cfg.Channels <= 0 {
		return nil, errors.New("audio: SampleRate and Channels must be positive")
	}
	if cfg.Latency <= 0 {
		cfg.Latency = DefaultLatency
	}
	if cfg.MaxLatency < cfg.Latency {
		cfg.MaxLatency = 3 * cfg.Latency
	}
	frame := cfg.Channels * bytesPerSample
	bytesFor := func(d time.Duration) int {
		return int(d.Seconds()*float64(cfg.SampleRate)) * frame
	}
	p := &Player{q: newQueue(frame, bytesFor(cfg.Latency), bytesFor(cfg.MaxLatency))}

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio: no sound system: %w", err)
	}
	dc := malgo.DefaultDeviceConfig(malgo.Playback)
	dc.Playback.Format = malgo.FormatS16
	dc.Playback.Channels = uint32(cfg.Channels)
	dc.SampleRate = uint32(cfg.SampleRate)
	dc.PeriodSizeInMilliseconds = 10
	dev, err := malgo.InitDevice(ctx.Context, dc, malgo.DeviceCallbacks{
		Data: func(out, _ []byte, _ uint32) {
			p.mu.Lock()
			p.q.fill(out)
			p.mu.Unlock()
		},
	})
	if err != nil {
		ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: open output: %w", err)
	}
	if err := dev.Start(); err != nil {
		dev.Uninit()
		ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: start output: %w", err)
	}
	p.ctx, p.dev = ctx, dev
	return p, nil
}

// Write queues pcm, whole frames of interleaved samples. It never blocks
// on the device; a backlog past MaxLatency drops the oldest audio.
func (p *Player) Write(pcm []byte) {
	p.mu.Lock()
	if !p.closed {
		p.q.write(pcm)
	}
	p.mu.Unlock()
}

// Buffered is how much audio is queued.
func (p *Player) Buffered() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.q.len
}

// Close stops playback and releases the device. Writes after Close are
// dropped. Calling it again, or from several goroutines at once, is safe:
// every call returns once the device is released.
func (p *Player) Close() error {
	p.close.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.q.drop(p.q.len)
		p.mu.Unlock()
		// Uninit waits for the device callback, which takes mu: it must
		// run unlocked.
		if p.dev != nil {
			p.dev.Uninit()
		}
		if p.ctx != nil {
			p.ctx.Uninit()
			p.ctx.Free()
		}
	})
	return nil
}

// queue is the jitter buffer: a ring of PCM bytes the device drains.
type queue struct {
	frame       int // bytes per frame; reads and drops stay frame-aligned
	target, max int // bytes: start playing at target, drop back to it past max
	buf         []byte
	head, len   int
	playing     bool // false while (re)filling to target
}

func newQueue(frame, target, max int) queue {
	return queue{frame: frame, target: target, max: max, buf: make([]byte, max+frame)}
}

// write appends pcm. Past max, it keeps only the newest target bytes:
// latency drops back to the target instead of creeping up.
func (q *queue) write(pcm []byte) {
	pcm = pcm[:len(pcm)/q.frame*q.frame]
	if q.len+len(pcm) > q.max {
		if len(pcm) >= q.target { // the write alone fills the target
			q.drop(q.len)
			pcm = pcm[len(pcm)-q.target:]
		} else {
			q.drop(q.len + len(pcm) - q.target)
		}
	}
	tail := (q.head + q.len) % len(q.buf)
	n := copy(q.buf[tail:], pcm)
	copy(q.buf, pcm[n:])
	q.len += len(pcm)
}

// drop discards n bytes (frame-aligned, at most len) from the front.
func (q *queue) drop(n int) {
	n = min(q.len, (n+q.frame-1)/q.frame*q.frame)
	q.head = (q.head + n) % len(q.buf)
	q.len -= n
}

// fill writes the next len(out) bytes into out: queued audio once target
// is reached, silence otherwise. Running dry starts filling again.
func (q *queue) fill(out []byte) {
	if !q.playing && q.len >= q.target {
		q.playing = true
	}
	n := 0
	if q.playing {
		n = min(len(out), q.len)
		m := copy(out[:n], q.buf[q.head:])
		copy(out[m:n], q.buf)
		q.head = (q.head + n) % len(q.buf)
		q.len -= n
		if n < len(out) {
			q.playing = false // underrun: buffer up again
		}
	}
	clear(out[n:])
}
