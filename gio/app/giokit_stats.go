package app

import (
	"sync/atomic"
	"time"

	"github.com/hhhapz/giokit/gio/gpu"
)

// giokit: frame statistics for performance overlays. The renderer may run
// on another goroutine than the one handling events, so it publishes them
// through atomics.

// GPUStats are a window's figures from its renderer.
type GPUStats struct {
	// Render is how long the last frame took to encode and submit to the
	// GPU, on the CPU; not the GPU's own time, which no backend reports.
	Render time.Duration
	// PathBuilds is the running count of filled shapes flattened and
	// uploaded (see gpu.Stats); the difference between two readings is
	// the shapes built in between, the renderer's cache misses.
	PathBuilds uint64
}

type gpuStats struct {
	render atomic.Int64 // ns
	paths  atomic.Uint64
}

// GPUStats returns the renderer's figures for the last frame drawn; zero
// before the first frame. Safe from any goroutine.
func (w *Window) GPUStats() GPUStats {
	return GPUStats{Render: time.Duration(w.giokitGPU.render.Load()), PathBuilds: w.giokitGPU.paths.Load()}
}

// recordGPU publishes a frame's figures; the renderer calls it.
func (w *Window) recordGPU(render time.Duration) {
	w.giokitGPU.render.Store(int64(render))
	if w.gpu != nil {
		w.giokitGPU.paths.Store(gpu.ReadStats(w.gpu).PathBuilds)
	}
}
