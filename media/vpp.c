//go:build linux

#include "vpp.h"

#include <errno.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <unistd.h>

#include <libavutil/error.h>
#include <libavutil/hwcontext.h>
#include <libavutil/hwcontext_vaapi.h>
#include <va/va.h>
#include <va/va_drmcommon.h>

// Output surfaces in flight. The renderer holds at most the frame on screen
// and the one being bound; extra slots absorb timing slack.
#define SLOTS 6

// Buffer IDs: each output surface gets one, never reused.
static atomic_uint_fast64_t next_id = 1;

typedef struct {
	gk_vpp *vpp;
	uint64_t id;
	VASurfaceID surface;
	VADRMPRIMESurfaceDescriptor desc; // exported once; fds owned here
	atomic_int busy;
} slot;

struct gk_vpp {
	AVBufferRef *hwdev; // keeps the VADisplay alive
	VADisplay dpy;
	VAConfigID cfg;
	VAContextID ctx;
	int w, h;
	slot slots[SLOTS];
	atomic_int refs; // 1 for the owner + 1 per busy slot
};

static int va_err(VAStatus s) { return s == VA_STATUS_SUCCESS ? 0 : AVERROR_EXTERNAL; }

static void destroy(gk_vpp *v) {
	for (int i = 0; i < SLOTS; i++) {
		slot *s = &v->slots[i];
		for (uint32_t k = 0; k < s->desc.num_objects; k++) {
			close(s->desc.objects[k].fd);
		}
		if (s->surface != VA_INVALID_SURFACE) {
			vaDestroySurfaces(v->dpy, &s->surface, 1);
		}
	}
	if (v->ctx != VA_INVALID_ID) {
		vaDestroyContext(v->dpy, v->ctx);
	}
	if (v->cfg != VA_INVALID_ID) {
		vaDestroyConfig(v->dpy, v->cfg);
	}
	av_buffer_unref(&v->hwdev);
	free(v);
}

static void unref(gk_vpp *v) {
	if (atomic_fetch_sub(&v->refs, 1) == 1) {
		destroy(v);
	}
}

int gk_vpp_available(void) { return 1; }

gk_vpp *gk_vpp_open(AVBufferRef *hwdev, int w, int h, int *err) {
	AVHWDeviceContext *dc = (AVHWDeviceContext *)hwdev->data;
	AVVAAPIDeviceContext *va = dc->hwctx;
	gk_vpp *v = calloc(1, sizeof *v);
	v->hwdev = av_buffer_ref(hwdev);
	v->dpy = va->display;
	v->cfg = VA_INVALID_ID;
	v->ctx = VA_INVALID_ID;
	v->w = w;
	v->h = h;
	atomic_init(&v->refs, 1);
	VASurfaceID ids[SLOTS];
	for (int i = 0; i < SLOTS; i++) {
		v->slots[i].surface = VA_INVALID_SURFACE;
	}
	VASurfaceAttrib attr = {
		.type = VASurfaceAttribPixelFormat,
		.flags = VA_SURFACE_ATTRIB_SETTABLE,
		.value = {.type = VAGenericValueTypeInteger, .value.i = VA_FOURCC_RGBA},
	};
	if ((*err = va_err(vaCreateConfig(v->dpy, VAProfileNone, VAEntrypointVideoProc, NULL, 0, &v->cfg))) < 0 ||
	    (*err = va_err(vaCreateSurfaces(v->dpy, VA_RT_FORMAT_RGB32, w, h, ids, SLOTS, &attr, 1))) < 0) {
		destroy(v);
		return NULL;
	}
	uint64_t base = atomic_fetch_add(&next_id, SLOTS);
	for (int i = 0; i < SLOTS; i++) {
		v->slots[i].surface = ids[i];
		v->slots[i].vpp = v;
		v->slots[i].id = base + i;
	}
	if ((*err = va_err(vaCreateContext(v->dpy, v->cfg, w, h, VA_PROGRESSIVE, ids, SLOTS, &v->ctx))) < 0) {
		destroy(v);
		return NULL;
	}
	for (int i = 0; i < SLOTS; i++) {
		slot *s = &v->slots[i];
		VAStatus st = vaExportSurfaceHandle(v->dpy, s->surface, VA_SURFACE_ATTRIB_MEM_TYPE_DRM_PRIME_2,
		                                    VA_EXPORT_SURFACE_READ_ONLY | VA_EXPORT_SURFACE_COMPOSED_LAYERS, &s->desc);
		if ((*err = va_err(st)) < 0 || s->desc.num_layers != 1 || s->desc.layers[0].num_planes != 1) {
			if (*err == 0) {
				*err = AVERROR(ENOSYS); // not a single RGBA plane: the importer could not use it
			}
			destroy(v);
			return NULL;
		}
	}
	return v;
}

int gk_vpp_fits(gk_vpp *v, int w, int h) { return v->w == w && v->h == h; }

static VAProcColorStandardType standard(const AVFrame *f) {
	switch (f->colorspace) {
	case AVCOL_SPC_BT709: return VAProcColorStandardBT709;
	case AVCOL_SPC_SMPTE170M:
	case AVCOL_SPC_BT470BG: return VAProcColorStandardBT601;
	case AVCOL_SPC_BT2020_NCL:
	case AVCOL_SPC_BT2020_CL: return VAProcColorStandardBT2020;
	default: return f->height >= 720 ? VAProcColorStandardBT709 : VAProcColorStandardBT601;
	}
}

void *gk_vpp_convert(gk_vpp *v, const AVFrame *f, gk_dmabuf *out, int *err) {
	slot *s = NULL;
	for (int i = 0; i < SLOTS; i++) {
		int idle = 0;
		if (atomic_compare_exchange_strong(&v->slots[i].busy, &idle, 1)) {
			s = &v->slots[i];
			break;
		}
	}
	if (!s) {
		*err = AVERROR(EAGAIN); // the renderer holds every surface
		return NULL;
	}
	atomic_fetch_add(&v->refs, 1);
	VAProcPipelineParameterBuffer p = {0};
	p.surface = (VASurfaceID)(uintptr_t)f->data[3];
	// Same standard in and out: convert YUV to RGB with the stream's matrix
	// and nothing else. Different standards make the driver remap primaries
	// too, which desaturates (the software path does not, and neither does
	// any player).
	p.surface_color_standard = standard(f);
	p.output_color_standard = p.surface_color_standard;
	p.input_color_properties.color_range =
	    f->color_range == AVCOL_RANGE_JPEG ? VA_SOURCE_RANGE_FULL : VA_SOURCE_RANGE_REDUCED;
	p.output_color_properties.color_range = VA_SOURCE_RANGE_FULL;
	p.output_background_color = 0xff000000;
	VABufferID buf = VA_INVALID_ID;
	VAStatus st = vaCreateBuffer(v->dpy, v->ctx, VAProcPipelineParameterBufferType, sizeof p, 1, &p, &buf);
	if (st == VA_STATUS_SUCCESS) {
		st = vaBeginPicture(v->dpy, v->ctx, s->surface);
	}
	if (st == VA_STATUS_SUCCESS) {
		st = vaRenderPicture(v->dpy, v->ctx, &buf, 1);
		VAStatus end = vaEndPicture(v->dpy, v->ctx);
		if (st == VA_STATUS_SUCCESS) {
			st = end;
		}
	}
	if (buf != VA_INVALID_ID) {
		vaDestroyBuffer(v->dpy, buf);
	}
	if (st == VA_STATUS_SUCCESS) {
		// Wait for the conversion: the importer samples the buffer next.
		st = vaSyncSurface(v->dpy, s->surface);
	}
	if (st != VA_STATUS_SUCCESS) {
		*err = AVERROR_EXTERNAL;
		gk_vpp_release(s);
		return NULL;
	}
	const VADRMPRIMESurfaceDescriptor *d = &s->desc;
	int obj = d->layers[0].object_index[0];
	out->fd = d->objects[obj].fd;
	out->offset = d->layers[0].offset[0];
	out->pitch = d->layers[0].pitch[0];
	out->fourcc = d->layers[0].drm_format;
	out->modifier = d->objects[obj].drm_format_modifier;
	out->width = v->w;
	out->height = v->h;
	out->id = s->id;
	return s;
}

void gk_vpp_release(void *h) {
	slot *s = h;
	gk_vpp *v = s->vpp;
	atomic_store(&s->busy, 0);
	unref(v);
}

void gk_vpp_close(gk_vpp *v) {
	if (v) {
		unref(v);
	}
}
