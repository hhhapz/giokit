package headless

import "github.com/hhhapz/giokit/gio/gpu"

// Stats returns the window's GPU totals (giokit); zero before its first
// frame.
func (w *Window) Stats() gpu.Stats {
	if w.gpu == nil {
		return gpu.Stats{}
	}
	return gpu.ReadStats(w.gpu)
}
