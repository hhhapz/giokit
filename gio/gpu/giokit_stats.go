package gpu

// Stats are a GPU's running totals, for finding drawing that defeats the
// renderer's caches (giokit).
type Stats struct {
	// PathBuilds counts the filled shapes (clip.Outline, clip.Path, and
	// shapes made of them, like rounded rectangles and text drawn as
	// paths) flattened and uploaded as a new vertex buffer. A shape is
	// cached by the operation that drew it, so a shape drawn straight
	// into a frame's ops is built again every frame it is drawn, while
	// one replayed from a recording kept across frames is built once.
	PathBuilds uint64
}

// ReadStats returns g's totals; zero for a GPU this package didn't make.
func ReadStats(g GPU) Stats {
	if g, ok := g.(*gpu); ok {
		return Stats{PathBuilds: g.drawOps.pathBuilds}
	}
	return Stats{}
}
