package kit

import (
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// DrawCache records drawing once and replays it on later frames until its
// key changes, so the GPU reuses the geometry instead of rebuilding it.
//
// The renderer keeps a filled shape's vertex buffer only while the
// operation that drew it is the same one, in the same op list; the frame's
// list is reset every frame, so a clip.Outline or clip.Path drawn straight
// into gtx.Ops is flattened and uploaded again every frame (a GPU buffer
// per shape, plus garbage for the collector). Replaying a DrawCache's
// recording is the same operation each time and hits the cache. The cache
// key ignores translation, so a recording replayed under op.Offset at a
// new position each frame (a sprite's shadow in flight) still hits;
// scaling, rotation and shear re-flatten.
//
// Use it for drawing that changes rarely: backgrounds, decorations, text
// drawn as shapes, anything whose inputs fit a comparable key. Draw only:
// input handlers recorded inside are replayed as recorded, with whatever
// state they captured, so keep clickables and tags outside.
//
// The zero value is empty and records on first use. A DrawCache belongs to
// the UI goroutine and must not be copied after first use.
type DrawCache[K comparable] struct {
	ops  op.Ops
	call op.CallOp
	key  K
	ok   bool
}

// Draw adds the recording to gtx.Ops, first recording it with draw when
// the cache is empty or was recorded with a different key. draw gets gtx
// with its Ops replaced by the cache's own list; everything else is as
// passed.
func (c *DrawCache[K]) Draw(gtx layout.Context, key K, draw func(gtx layout.Context)) {
	c.Record(gtx, key, draw).Add(gtx.Ops)
}

// Record is Draw without adding: it returns the recording, made with draw
// if key differs from the cached one, for the caller to add (once or
// several times, under its own transforms).
func (c *DrawCache[K]) Record(gtx layout.Context, key K, draw func(gtx layout.Context)) op.CallOp {
	if !c.ok || c.key != key {
		c.ops.Reset()
		rec := gtx
		rec.Ops = &c.ops
		m := op.Record(&c.ops)
		draw(rec)
		c.call = m.Stop()
		c.key, c.ok = key, true
	}
	return c.call
}

// Invalidate empties the cache, so the next Draw or Record records again
// whatever the key.
func (c *DrawCache[K]) Invalidate() { c.ok = false }
