//go:build windows

package pad

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// The Windows source polls XInput: Xbox pads, and any controller Steam
// Input presents as one. XInput has no events, so a goroutine polls the
// four slots (connected ones every few milliseconds, empty ones once a
// second, as probing an empty slot is slow) and wakes the app on change.
// XInput doesn't expose the Guide button. PlayStation and Nintendo pads
// need Steam Input (or DS4Windows and the like) on Windows.

func platformSource() Source { return &xinputSource{} }

var (
	xinput = func() *syscall.LazyDLL {
		for _, n := range []string{"xinput1_4.dll", "xinput9_1_0.dll", "xinput1_3.dll"} {
			if d := syscall.NewLazyDLL(n); d.Load() == nil {
				return d
			}
		}
		return nil
	}()
)

const (
	pollConnected = 8 * time.Millisecond
	pollEmpty     = time.Second
)

type xinputSource struct {
	mu   sync.Mutex
	pads [4]*Pad
	stop chan struct{}
	done chan struct{}
}

func (s *xinputSource) Start(wake func()) error {
	if xinput == nil {
		return syscall.ERROR_MOD_NOT_FOUND
	}
	get := xinput.NewProc("XInputGetState")
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(s.done)
		var last [4]uint32
		var probed [4]time.Time
		t := time.NewTicker(pollConnected)
		defer t.Stop()
		for {
			changed := false
			for i := range uint32(4) {
				s.mu.Lock()
				connected := s.pads[i] != nil
				s.mu.Unlock()
				if !connected && time.Since(probed[i]) < pollEmpty {
					continue
				}
				probed[i] = time.Now()
				var st xinputState
				r, _, _ := get.Call(uintptr(i), uintptr(unsafe.Pointer(&st)))
				s.mu.Lock()
				switch {
				case r != 0 && s.pads[i] != nil:
					s.pads[i], changed = nil, true
				case r == 0 && (s.pads[i] == nil || st.packet != last[i]):
					p := xinputPad(int(i)+1, st)
					s.pads[i], last[i], changed = &p, st.packet, true
				}
				s.mu.Unlock()
			}
			if changed {
				wake()
			}
			select {
			case <-s.stop:
				return
			case <-t.C:
			}
		}
	}()
	return nil
}

func (s *xinputSource) Poll(dst []Pad) []Pad {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pads {
		if p != nil {
			dst = append(dst, *p)
		}
	}
	return dst
}

func (s *xinputSource) Close() error {
	close(s.stop)
	<-s.done
	return nil
}
