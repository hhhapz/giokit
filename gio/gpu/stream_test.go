// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"testing"

	"github.com/hhhapz/giokit/gio/gpu/internal/driver"
	"github.com/hhhapz/giokit/gio/internal/stream"
)

// importDevice is a device that only imports DMA-BUFs. Calling any other
// Device method panics.
type importDevice struct {
	driver.Device
	imports int
}

func (d *importDevice) ImportDMABuf(b driver.DMABuf, filter driver.TextureFilter) (driver.Texture, error) {
	d.imports++
	return &importedTexture{fd: b.FD}, nil
}

// importedTexture records its release. Calling any other Texture method
// panics.
type importedTexture struct {
	driver.Texture
	fd       int
	released bool
}

func (t *importedTexture) Release() {
	if t.released {
		panic("texture released twice")
	}
	t.released = true
}

// dmaFrame is a frame of the DMA-BUF with the given ID; its Release
// increments *released.
func dmaFrame(id uint64, released *int) stream.Frame {
	return stream.Frame{
		DMA:     &stream.DMABuf{ID: id, FD: int(id), Width: 1, Height: 1},
		Release: func() { *released++ },
	}
}

// drawStream draws st once with r, as a frame would, and returns the bound
// texture.
func drawStream(t *testing.T, r *renderer, cache *textureCache, st *stream.Stream) *importedTexture {
	t.Helper()
	key := textureCacheKey{filter: filterLinear, handle: st}
	tex, ok := r.streamTex(cache, key, st).(*importedTexture)
	if !ok {
		t.Fatal("stream did not draw an imported texture")
	}
	return tex
}

func TestStreamImportEviction(t *testing.T) {
	dev := new(importDevice)
	r := &renderer{ctx: dev}
	cache := newTextureCache()
	var st stream.Stream
	released := 0
	const n = 2*maxStreamImports + 3
	for id := uint64(1); id <= n; id++ {
		st.Submit(dmaFrame(id, &released))
		tex := drawStream(t, r, cache, &st)
		if tex.released {
			t.Fatalf("buffer %d: bound texture was released by eviction", id)
		}
		if tex.fd != int(id) {
			t.Fatalf("buffer %d: bound the import of buffer %d", id, tex.fd)
		}
		cache.frame()
	}
	if dev.imports != n {
		t.Errorf("imported %d times, want %d", dev.imports, n)
	}
	if released != n-1 {
		t.Errorf("released %d frames, want all but the bound one (%d)", released, n-1)
	}
	// A producer cycling a few buffers reuses their imports.
	for id := uint64(n - 2); id <= n; id++ {
		st.Submit(dmaFrame(id, &released))
		drawStream(t, r, cache, &st)
		cache.frame()
	}
	if dev.imports != n {
		t.Errorf("recent buffers re-imported: %d imports, want %d", dev.imports, n)
	}
}

// Two windows draw the same stream: a DMA-BUF frame is released only after
// both have let go of it.
func TestStreamTwoRenderers(t *testing.T) {
	var st stream.Stream
	released := 0
	type window struct {
		r     *renderer
		cache *textureCache
	}
	var wins [2]window
	for i := range wins {
		wins[i] = window{r: &renderer{ctx: new(importDevice)}, cache: newTextureCache()}
	}
	st.Submit(dmaFrame(1, &released))
	for _, w := range wins {
		drawStream(t, w.r, w.cache, &st)
		w.cache.frame()
	}
	st.Submit(dmaFrame(2, &released))
	if released != 0 {
		t.Fatalf("bound frame released on replace: %d", released)
	}
	drawStream(t, wins[0].r, wins[0].cache, &st)
	wins[0].cache.frame()
	if released != 0 {
		t.Fatalf("frame released while the second window still samples it: %d", released)
	}
	// The second window stops drawing the stream: its cache drops the texture
	// at the end of the next frame.
	wins[1].cache.frame()
	if released != 1 {
		t.Fatalf("released %d frames after both windows moved on, want 1", released)
	}
	st.Close()
	if released != 1 {
		t.Fatalf("Close released the frame the first window still samples: %d", released)
	}
	wins[0].cache.frame()
	wins[0].cache.frame()
	if released != 2 {
		t.Fatalf("released %d frames after closing and dropping, want 2", released)
	}
}
