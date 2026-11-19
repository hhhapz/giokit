// SPDX-License-Identifier: Unlicense OR MIT

//go:build linux && !android

package opengl

// DMA-BUF import (giokit patch): a DMA-BUF becomes an EGLImage, the
// EGLImage the immutable storage of a texture (GL_EXT_EGL_image_storage),
// and an sRGB view of that texture (texture views) what Gio samples, so the
// image decodes from sRGB like every other Gio texture. Nothing is copied.

/*
#cgo pkg-config: egl
#include <stdint.h>
#include <string.h>
#include <EGL/egl.h>
#include <EGL/eglext.h>

typedef void (*gk_storage_fn)(unsigned int target, void *image, const int *attribs);
typedef void (*gk_view_fn)(unsigned int tex, unsigned int target, unsigned int orig,
                           unsigned int fmt, unsigned int minlevel, unsigned int numlevels,
                           unsigned int minlayer, unsigned int numlayers);

static PFNEGLCREATEIMAGEKHRPROC gk_create_image;
static PFNEGLDESTROYIMAGEKHRPROC gk_destroy_image;
static gk_storage_fn gk_storage;
static gk_view_fn gk_view;

// gk_dmabuf_load resolves the entry points; it needs a current context.
// Returns 0 when import is impossible, 1 when possible (2 with views).
static int gk_dmabuf_load(void) {
	EGLDisplay dpy = eglGetCurrentDisplay();
	if (dpy == EGL_NO_DISPLAY) {
		return 0;
	}
	const char *exts = eglQueryString(dpy, EGL_EXTENSIONS);
	if (!exts || !strstr(exts, "EGL_EXT_image_dma_buf_import")) {
		return 0;
	}
	gk_create_image = (PFNEGLCREATEIMAGEKHRPROC)eglGetProcAddress("eglCreateImageKHR");
	gk_destroy_image = (PFNEGLDESTROYIMAGEKHRPROC)eglGetProcAddress("eglDestroyImageKHR");
	gk_storage = (gk_storage_fn)eglGetProcAddress("glEGLImageTargetTexStorageEXT");
	gk_view = (gk_view_fn)eglGetProcAddress("glTextureViewOES");
	if (!gk_view) {
		gk_view = (gk_view_fn)eglGetProcAddress("glTextureViewEXT");
	}
	if (!gk_view) {
		gk_view = (gk_view_fn)eglGetProcAddress("glTextureView");
	}
	if (!gk_create_image || !gk_destroy_image || !gk_storage) {
		return 0;
	}
	return gk_view ? 2 : 1;
}

static void *gk_dmabuf_image(int w, int h, uint32_t fourcc, int fd, uint32_t off, uint32_t pitch, uint64_t mod) {
	EGLint a[32];
	int i = 0;
	a[i++] = EGL_WIDTH; a[i++] = w;
	a[i++] = EGL_HEIGHT; a[i++] = h;
	a[i++] = EGL_LINUX_DRM_FOURCC_EXT; a[i++] = (EGLint)fourcc;
	a[i++] = EGL_DMA_BUF_PLANE0_FD_EXT; a[i++] = fd;
	a[i++] = EGL_DMA_BUF_PLANE0_OFFSET_EXT; a[i++] = (EGLint)off;
	a[i++] = EGL_DMA_BUF_PLANE0_PITCH_EXT; a[i++] = (EGLint)pitch;
	if (mod != 0x00ffffffffffffffULL) { // DRM_FORMAT_MOD_INVALID: no modifier
		a[i++] = EGL_DMA_BUF_PLANE0_MODIFIER_LO_EXT; a[i++] = (EGLint)(mod & 0xffffffff);
		a[i++] = EGL_DMA_BUF_PLANE0_MODIFIER_HI_EXT; a[i++] = (EGLint)(mod >> 32);
	}
	a[i++] = EGL_NONE;
	EGLImageKHR img = gk_create_image(eglGetCurrentDisplay(), EGL_NO_CONTEXT, EGL_LINUX_DMA_BUF_EXT, NULL, a);
	return img == EGL_NO_IMAGE_KHR ? NULL : img;
}

static void gk_dmabuf_storage(void *img) { gk_storage(0x0DE1, img, NULL); } // GL_TEXTURE_2D

static void gk_dmabuf_view(unsigned int view, unsigned int orig, unsigned int fmt) {
	gk_view(view, 0x0DE1, orig, fmt, 0, 1, 0, 1);
}

static void gk_dmabuf_destroy(void *img) { gk_destroy_image(eglGetCurrentDisplay(), img); }
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/hhhapz/giokit/gio/gpu/internal/driver"
	"github.com/hhhapz/giokit/gio/internal/gl"
)

func init() {
	releaseDMABufImage = func(img unsafe.Pointer) { C.gk_dmabuf_destroy(img) }
}

// ImportDMABuf implements driver.DMABufImporter.
func (b *Backend) ImportDMABuf(d driver.DMABuf, filter driver.TextureFilter) (driver.Texture, error) {
	if b.gles && b.glver[0] < 3 {
		return nil, errors.New("opengl: DMA-BUF import needs OpenGL ES 3")
	}
	if !b.dmabuf.loaded {
		b.dmabuf.loaded = true
		b.dmabuf.level = int(C.gk_dmabuf_load())
	}
	srgb := b.srgbaTriple.internalFormat == gl.SRGB8_ALPHA8
	if b.dmabuf.level == 0 || (srgb && b.dmabuf.level < 2) {
		return nil, errors.New("opengl: DMA-BUF import unsupported (EGL_EXT_image_dma_buf_import, GL_EXT_EGL_image_storage and texture views needed)")
	}
	glErr(b.funcs)
	img := C.gk_dmabuf_image(C.int(d.Width), C.int(d.Height), C.uint32_t(d.Fourcc), C.int(d.FD),
		C.uint32_t(d.Offset), C.uint32_t(d.Pitch), C.uint64_t(d.Modifier))
	if img == nil {
		return nil, fmt.Errorf("opengl: eglCreateImageKHR failed for DMA-BUF %dx%d fourcc %#x modifier %#x", d.Width, d.Height, d.Fourcc, d.Modifier)
	}
	base := b.funcs.CreateTexture()
	b.glstate.bindTexture(b.funcs, 0, base)
	C.gk_dmabuf_storage(img)
	if err := glErr(b.funcs); err != nil {
		b.glstate.deleteTexture(b.funcs, base)
		C.gk_dmabuf_destroy(img)
		return nil, fmt.Errorf("opengl: glEGLImageTargetTexStorageEXT: %w", err)
	}
	t := &texture{backend: b, obj: base, width: d.Width, height: d.Height,
		triple: textureTriple{gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE}, bindings: driver.BufferBindingTexture,
		dmabufImage: img}
	if srgb {
		// Sample through an sRGB view of the same storage.
		view := b.funcs.CreateTexture()
		C.gk_dmabuf_view(C.uint(view.V), C.uint(base.V), C.uint(gl.SRGB8_ALPHA8))
		if err := glErr(b.funcs); err != nil {
			b.glstate.deleteTexture(b.funcs, view)
			t.Release()
			return nil, fmt.Errorf("opengl: texture view: %w", err)
		}
		t.dmabufBase, t.obj = base, view
		t.triple = b.srgbaTriple
	}
	b.glstate.bindTexture(b.funcs, 0, t.obj)
	f, _ := toTexFilter(filter)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, f)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, f)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	if err := glErr(b.funcs); err != nil {
		t.Release()
		return nil, err
	}
	return t, nil
}
