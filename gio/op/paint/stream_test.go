// SPDX-License-Identifier: Unlicense OR MIT

package paint

import (
	"image"
	"sync"
	"testing"

	"github.com/hhhapz/giokit/gio/op"
)

// Windows draw one stream on their own goroutines while its frame size
// changes; run with -race. giokit patch.
func TestStreamAddFromWindows(t *testing.T) {
	s := NewStream()
	s.Submit(StreamFrame{Image: image.NewRGBA(image.Rect(0, 0, 2, 2))})
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var o op.Ops
			for i := range 2000 {
				s.Submit(StreamFrame{Image: image.NewRGBA(image.Rect(0, 0, 2+i%2, 2))})
				o.Reset()
				s.Add(&o)
			}
		}()
	}
	close(start)
	wg.Wait()
}
