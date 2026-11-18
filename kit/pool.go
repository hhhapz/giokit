package kit

// Pool keeps per-item widget state keyed by a stable id (never a list index).
// The zero value is an empty pool. Values are allocated by the pool and
// keep their address while they live, so they may be event tags
// (Clickable); the pool itself must not be copied after first use.
//
// Entries live until Delete, or until Sweep finds them untouched. Components
// whose data can grow without bound (logs, file trees, chat) call Sweep once
// per frame after calling Get for every live id; ids that left the data are
// then dropped after 65 to 128 Sweeps untouched instead of leaking. Like
// all kit state, a Pool belongs to the UI goroutine.
type Pool[T any] struct {
	m     map[string]*T
	stamp map[string]uint32
	gen   uint32
}

// poolIdle is how many Sweep calls an entry may go untouched before it is
// dropped. The scan of every entry also runs only every poolIdle calls, so
// Sweep costs O(n/poolIdle) per call, amortized, for n entries.
const poolIdle = 64

// Get returns the state for id, creating a zero T on first use, and marks
// id as live for Sweep. The pointer stays valid until id is deleted or swept.
func (p *Pool[T]) Get(id string) *T {
	if p.m == nil {
		p.m = map[string]*T{}
		p.stamp = map[string]uint32{}
	}
	v := p.m[id]
	if v == nil {
		v = new(T)
		p.m[id] = v
	}
	p.stamp[id] = p.gen
	return v
}

// Delete drops the state for id.
func (p *Pool[T]) Delete(id string) {
	delete(p.m, id)
	delete(p.stamp, id)
}

// Len is the number of ids with state.
func (p *Pool[T]) Len() int { return len(p.m) }

// Sweep drops entries that no Get touched during the last poolIdle sweeps.
// Only call it from code that calls Get for every live id each frame;
// otherwise state for live but unvisited ids (say, rows scrolled out of view)
// is lost.
func (p *Pool[T]) Sweep() {
	p.gen++
	if p.gen%poolIdle != 0 {
		return
	}
	for id, at := range p.stamp {
		if p.gen-at > poolIdle {
			p.Delete(id)
		}
	}
}

// Each calls f for every id with state, in no particular order. f may
// Delete entries (the current one or others; deleted ones not yet visited
// are skipped), but must not call Get for new ids: whether Each visits them
// is unspecified.
func (p *Pool[T]) Each(f func(id string, v *T)) {
	for k, v := range p.m {
		f(k, v)
	}
}
