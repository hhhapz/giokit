package pad

import "sync"

// Fake is a Source for tests: pads appear, press and move only when told.
// Its methods are safe from any goroutine. The zero value has no pads.
type Fake struct {
	mu   sync.Mutex
	pads []Pad
	wake func()
	next int
}

var _ Source = (*Fake)(nil)

// Start implements Source.
func (f *Fake) Start(wake func()) error {
	f.mu.Lock()
	f.wake = wake
	f.mu.Unlock()
	return nil
}

// Poll implements Source.
func (f *Fake) Poll(dst []Pad) []Pad {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(dst, f.pads...)
}

// Close implements Source.
func (f *Fake) Close() error { return nil }

// Connect adds a pad and returns its id.
func (f *Fake) Connect(name string, family Family) int {
	f.mu.Lock()
	f.next++
	id := f.next
	f.pads = append(f.pads, Pad{ID: id, Name: name, Family: family})
	f.mu.Unlock()
	f.poke()
	return id
}

// Disconnect removes pad id.
func (f *Fake) Disconnect(id int) {
	f.edit(id, func(_ *Pad) {})
	f.mu.Lock()
	for i, p := range f.pads {
		if p.ID == id {
			f.pads = append(f.pads[:i], f.pads[i+1:]...)
			break
		}
	}
	f.mu.Unlock()
}

// Set holds (down) or releases b on pad id. The change shows at the next
// Pads.Frame.
func (f *Fake) Set(id int, b Button, down bool) {
	f.edit(id, func(p *Pad) {
		if down {
			p.Buttons |= 1 << b
		} else {
			p.Buttons &^= 1 << b
		}
	})
}

// SetAxis sets axis a on pad id.
func (f *Fake) SetAxis(id int, a Axis, v float32) {
	f.edit(id, func(p *Pad) { p.Axes[a] = v })
}

func (f *Fake) edit(id int, do func(p *Pad)) {
	f.mu.Lock()
	for i := range f.pads {
		if f.pads[i].ID == id {
			do(&f.pads[i])
		}
	}
	f.mu.Unlock()
	f.poke()
}

func (f *Fake) poke() {
	f.mu.Lock()
	w := f.wake
	f.mu.Unlock()
	if w != nil {
		w()
	}
}
