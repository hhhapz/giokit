#include "av.h"

#include <errno.h>
#include <stdlib.h>
#include <string.h>

#include <libavcodec/avcodec.h>
#include <libavcodec/bsf.h>
#include <libavformat/avformat.h>
#include <libavutil/hwcontext.h>
#include <libavutil/opt.h>
#include <libavutil/pixdesc.h>
#include <libswscale/swscale.h>

#include "vpp.h"

// gk_fail_hwdevice makes every hardware device creation fail, so tests can
// check the fallback where no device name can make it fail (D3D11VA takes
// an adapter index and falls back to the default adapter).
static int gk_fail_hwdevice;

void gk_set_fail_hwdevice(int fail) { gk_fail_hwdevice = fail; }


struct gk_dec {
	AVCodecContext *ctx;
	AVBufferRef *hw;          // hardware device, or NULL
	int hwkind;               // GK_HW_VAAPI, _D3D11VA, _VIDEOTOOLBOX, or GK_HW_OFF
	enum AVPixelFormat hwfmt; // the device's frame format (AV_PIX_FMT_VAAPI...) when hw
	AVFrame *frame, *sw;
	AVPacket *pkt;
	struct SwsContext *sws;
	gk_vpp *vpp; // GPU colour conversion for DMA-BUF export, lazily created
};

void gk_err(int err, char *buf, int cap) { av_strerror(err, buf, cap); }
int gk_eagain(void) { return AVERROR(EAGAIN); }
int gk_eof(void) { return AVERROR_EOF; }

static enum AVPixelFormat get_format(AVCodecContext *ctx, const enum AVPixelFormat *fmts) {
	gk_dec *d = ctx->opaque;
	const enum AVPixelFormat *p;
	for (p = fmts; *p != AV_PIX_FMT_NONE; p++) {
		if (d->hw && *p == d->hwfmt) {
			return *p;
		}
	}
	// No hardware format offered (unsupported profile or size): fall back to
	// the first software format. Frames then arrive in system memory.
	for (p = fmts; *p != AV_PIX_FMT_NONE; p++) {
		const AVPixFmtDescriptor *desc = av_pix_fmt_desc_get(*p);
		if (!(desc->flags & AV_PIX_FMT_FLAG_HWACCEL)) {
			return *p;
		}
	}
	return AV_PIX_FMT_NONE;
}

int gk_hw_platform(void) {
#if defined(_WIN32)
	return GK_HW_D3D11VA;
#elif defined(__APPLE__)
	return GK_HW_VIDEOTOOLBOX;
#else
	return GK_HW_VAAPI;
#endif
}

int gk_zero_copy(void) { return gk_vpp_available(); }

// hw_kind resolves AUTO and GPU to the platform's API.
static int hw_kind(int hw) {
	return hw == GK_HW_AUTO || hw == GK_HW_GPU ? gk_hw_platform() : hw;
}

// device_type maps an API to FFmpeg's device type. The enum values exist in
// every FFmpeg build; creating a device of a type the build or platform
// lacks fails at run time. Readback needs no platform headers.
static enum AVHWDeviceType device_type(int kind) {
	switch (kind) {
	case GK_HW_VAAPI: return AV_HWDEVICE_TYPE_VAAPI;
	case GK_HW_D3D11VA: return AV_HWDEVICE_TYPE_D3D11VA;
	case GK_HW_VIDEOTOOLBOX: return AV_HWDEVICE_TYPE_VIDEOTOOLBOX;
	}
	return AV_HWDEVICE_TYPE_NONE;
}

static enum AVCodecID codec_id(int codec) {
	switch (codec) {
	case GK_H264: return AV_CODEC_ID_H264;
	case GK_H265: return AV_CODEC_ID_HEVC;
	case GK_AV1: return AV_CODEC_ID_AV1;
	case GK_VP8: return AV_CODEC_ID_VP8;
	case GK_VP9: return AV_CODEC_ID_VP9;
	}
	return AV_CODEC_ID_NONE;
}

static int codec_of(enum AVCodecID id) {
	switch (id) {
	case AV_CODEC_ID_H264: return GK_H264;
	case AV_CODEC_ID_HEVC: return GK_H265;
	case AV_CODEC_ID_AV1: return GK_AV1;
	case AV_CODEC_ID_VP8: return GK_VP8;
	case AV_CODEC_ID_VP9: return GK_VP9;
	default: return 0;
	}
}

// find_codec picks the decoder. For AV1 FFmpeg has two: libdav1d (software)
// and "av1", which decodes only through a hwaccel.
static const AVCodec *find_codec(enum AVCodecID id, int want_hw) {
	if (id == AV_CODEC_ID_AV1) {
		const AVCodec *c = avcodec_find_decoder_by_name(want_hw ? "av1" : "libdav1d");
		if (c) {
			return c;
		}
	}
	return avcodec_find_decoder(id);
}

static int open_ctx(gk_dec *d, const AVCodec *codec, int hw, const char *device, int low_delay) {
	d->ctx = avcodec_alloc_context3(codec);
	if (!d->ctx) {
		return AVERROR(ENOMEM);
	}
	d->ctx->opaque = d;
	if (low_delay) {
		// Output each frame as soon as it is decoded: no reorder delay, no
		// frame threads (each adds a frame of latency). Slices can still be
		// decoded in parallel.
		d->ctx->flags |= AV_CODEC_FLAG_LOW_DELAY;
		d->ctx->thread_type = FF_THREAD_SLICE;
	}
	d->ctx->thread_count = 0;
	d->hwkind = GK_HW_OFF;
	if (hw != GK_HW_OFF) {
		int kind = hw_kind(hw);
		enum AVHWDeviceType type = device_type(kind);
		for (int i = 0; type != AV_HWDEVICE_TYPE_NONE; i++) {
			const AVCodecHWConfig *cfg = avcodec_get_hw_config(codec, i);
			if (!cfg) {
				break;
			}
			// The config that decodes into the device's own frames
			// (AV_PIX_FMT_VAAPI, _D3D11, _VIDEOTOOLBOX): they stay on the
			// GPU until gk_dec_frame_rgba transfers or vpp exports them.
			if ((cfg->methods & AV_CODEC_HW_CONFIG_METHOD_HW_DEVICE_CTX) && cfg->device_type == type) {
				if (!gk_fail_hwdevice && av_hwdevice_ctx_create(&d->hw, type, device && *device ? device : NULL, NULL, 0) == 0) {
					d->hwkind = kind;
					d->hwfmt = cfg->pix_fmt;
					d->ctx->hw_device_ctx = av_buffer_ref(d->hw);
				}
				break;
			}
		}
		if (!d->hw) {
			// No device, or the codec has no decoder for it. AUTO retries
			// in software (gk_dec_open); a forced API fails.
			return AVERROR(ENODEV);
		}
		d->ctx->get_format = get_format;
	}
	return avcodec_open2(d->ctx, codec, NULL);
}

gk_dec *gk_dec_open(int codec, int hw, const char *device, int low_delay, int *err) {
	enum AVCodecID id = codec_id(codec);
	if (id == AV_CODEC_ID_NONE) {
		*err = AVERROR_DECODER_NOT_FOUND;
		return NULL;
	}
	gk_dec *d = calloc(1, sizeof *d);
	d->frame = av_frame_alloc();
	d->sw = av_frame_alloc();
	d->pkt = av_packet_alloc();
	const AVCodec *c = find_codec(id, hw != GK_HW_OFF);
	if (!c) {
		*err = AVERROR_DECODER_NOT_FOUND;
		gk_dec_close(d);
		return NULL;
	}
	*err = open_ctx(d, c, hw, device, low_delay);
	if (*err < 0 && hw == GK_HW_AUTO) {
		// The hardware decoder could not open; try software.
		avcodec_free_context(&d->ctx);
		av_buffer_unref(&d->hw);
		c = find_codec(id, 0);
		*err = c ? open_ctx(d, c, GK_HW_OFF, NULL, low_delay) : AVERROR_DECODER_NOT_FOUND;
	}
	if (*err < 0) {
		gk_dec_close(d);
		return NULL;
	}
	return d;
}

void gk_dec_close(gk_dec *d) {
	if (!d) {
		return;
	}
	gk_vpp_close(d->vpp);
	avcodec_free_context(&d->ctx);
	av_buffer_unref(&d->hw);
	av_frame_free(&d->frame);
	av_frame_free(&d->sw);
	av_packet_free(&d->pkt);
	sws_freeContext(d->sws);
	free(d);
}

int gk_dec_hw(gk_dec *d) { return d->hwkind; }

int gk_dec_send(gk_dec *d, const uint8_t *data, int size, int64_t pts, int key) {
	av_packet_unref(d->pkt);
	int r = av_new_packet(d->pkt, size);
	if (r < 0) {
		return r;
	}
	memcpy(d->pkt->data, data, size);
	d->pkt->pts = d->pkt->dts = pts;
	if (key) {
		d->pkt->flags |= AV_PKT_FLAG_KEY;
	}
	return avcodec_send_packet(d->ctx, d->pkt);
}

int gk_dec_flush(gk_dec *d) { return avcodec_send_packet(d->ctx, NULL); }

int gk_dec_receive(gk_dec *d) {
	av_frame_unref(d->frame);
	return avcodec_receive_frame(d->ctx, d->frame);
}

void gk_dec_frame_info(gk_dec *d, int *w, int *h, int64_t *pts, int *hw) {
	*w = d->frame->width;
	*h = d->frame->height;
	*pts = d->frame->pts;
	*hw = d->frame->format == d->hwfmt && d->hw;
}

// colour_table returns the YUV→RGB matrix for a frame. Unspecified streams
// follow the usual convention: BT.709 from 720 lines up, else BT.601.
static const int *colour_table(const AVFrame *f) {
	switch (f->colorspace) {
	case AVCOL_SPC_BT709: return sws_getCoefficients(SWS_CS_ITU709);
	case AVCOL_SPC_BT2020_NCL:
	case AVCOL_SPC_BT2020_CL: return sws_getCoefficients(SWS_CS_BT2020);
	case AVCOL_SPC_SMPTE170M:
	case AVCOL_SPC_BT470BG: return sws_getCoefficients(SWS_CS_ITU601);
	default: return sws_getCoefficients(f->height >= 720 ? SWS_CS_ITU709 : SWS_CS_ITU601);
	}
}

int gk_dec_frame_rgba(gk_dec *d, uint8_t *dst, int stride) {
	AVFrame *src = d->frame;
	if (d->hw && src->format == d->hwfmt) {
		av_frame_unref(d->sw);
		int r = av_hwframe_transfer_data(d->sw, src, 0);
		if (r < 0) {
			return r;
		}
		d->sw->colorspace = src->colorspace;
		d->sw->color_range = src->color_range;
		src = d->sw;
	}
	d->sws = sws_getCachedContext(d->sws, src->width, src->height, src->format,
	                              src->width, src->height, AV_PIX_FMT_RGBA,
	                              SWS_BILINEAR, NULL, NULL, NULL);
	if (!d->sws) {
		return AVERROR(EINVAL);
	}
	const int *table = colour_table(src);
	int full = src->color_range == AVCOL_RANGE_JPEG ||
	           src->format == AV_PIX_FMT_YUVJ420P || src->format == AV_PIX_FMT_YUVJ444P;
	sws_setColorspaceDetails(d->sws, table, full, sws_getCoefficients(SWS_CS_DEFAULT), 1, 0, 1 << 16, 1 << 16);
	uint8_t *planes[4] = {dst};
	int strides[4] = {stride};
	int r = sws_scale(d->sws, (const uint8_t *const *)src->data, src->linesize, 0, src->height, planes, strides);
	return r < 0 ? r : 0;
}

void *gk_dec_frame_dmabuf(gk_dec *d, gk_dmabuf *out, int *err) {
	if (d->hwkind != GK_HW_VAAPI || d->frame->format != d->hwfmt) {
		*err = AVERROR(ENOSYS);
		return NULL;
	}
	if (!d->vpp || !gk_vpp_fits(d->vpp, d->frame->width, d->frame->height)) {
		gk_vpp_close(d->vpp);
		d->vpp = gk_vpp_open(d->hw, d->frame->width, d->frame->height, err);
		if (!d->vpp) {
			return NULL;
		}
	}
	return gk_vpp_convert(d->vpp, d->frame, out, err);
}

void gk_dmabuf_release(void *h) { gk_vpp_release(h); }

// ---- demuxer ----

struct gk_demux {
	AVFormatContext *fmt;
	int stream;
	AVBSFContext *bsf; // mp4 → Annex B for H.264/H.265, else NULL
	AVPacket *pkt;
	AVRational tb;
};

void gk_demux_close(gk_demux *m) {
	if (!m) {
		return;
	}
	av_bsf_free(&m->bsf);
	av_packet_free(&m->pkt);
	avformat_close_input(&m->fmt);
	free(m);
}

gk_demux *gk_demux_open(const char *path, int *err) {
	gk_demux *m = calloc(1, sizeof *m);
	m->pkt = av_packet_alloc();
	if ((*err = avformat_open_input(&m->fmt, path, NULL, NULL)) < 0 ||
	    (*err = avformat_find_stream_info(m->fmt, NULL)) < 0 ||
	    (*err = av_find_best_stream(m->fmt, AVMEDIA_TYPE_VIDEO, -1, -1, NULL, 0)) < 0) {
		gk_demux_close(m);
		return NULL;
	}
	m->stream = *err;
	*err = 0;
	AVStream *st = m->fmt->streams[m->stream];
	m->tb = st->time_base;
	const char *filter = NULL;
	if (st->codecpar->codec_id == AV_CODEC_ID_H264) {
		filter = "h264_mp4toannexb";
	} else if (st->codecpar->codec_id == AV_CODEC_ID_HEVC) {
		filter = "hevc_mp4toannexb";
	}
	if (filter) {
		const AVBitStreamFilter *f = av_bsf_get_by_name(filter);
		if ((*err = av_bsf_alloc(f, &m->bsf)) < 0 ||
		    (*err = avcodec_parameters_copy(m->bsf->par_in, st->codecpar)) < 0 ||
		    (*err = av_bsf_init(m->bsf)) < 0) {
			gk_demux_close(m);
			return NULL;
		}
	}
	return m;
}

void gk_demux_info(gk_demux *m, int *codec, int *w, int *h, int *fps_num, int *fps_den) {
	AVStream *st = m->fmt->streams[m->stream];
	*codec = codec_of(st->codecpar->codec_id);
	*w = st->codecpar->width;
	*h = st->codecpar->height;
	AVRational r = st->avg_frame_rate.num ? st->avg_frame_rate : st->r_frame_rate;
	*fps_num = r.num;
	*fps_den = r.den ? r.den : 1;
}

int gk_demux_config(gk_demux *m, uint8_t *buf, int cap) {
	AVCodecParameters *par = m->fmt->streams[m->stream]->codecpar;
	const uint8_t *data = par->extradata;
	int size = par->extradata_size;
	if (m->bsf) {
		// Annex B parameter sets, as the filter emits them in-band.
		data = m->bsf->par_out->extradata;
		size = m->bsf->par_out->extradata_size;
	} else if (par->codec_id == AV_CODEC_ID_AV1 && size > 4 && (data[0] & 0x80)) {
		// av1C: a 4-byte header, then the configuration OBUs.
		data += 4;
		size -= 4;
	}
	if (size <= cap && size > 0) {
		memcpy(buf, data, size);
	}
	return size;
}

int gk_demux_read(gk_demux *m, const uint8_t **data, int64_t *pts_us, int *key) {
	for (;;) {
		av_packet_unref(m->pkt);
		if (m->bsf) {
			int r = av_bsf_receive_packet(m->bsf, m->pkt);
			if (r == 0) {
				break;
			}
			if (r != AVERROR(EAGAIN)) {
				return r == AVERROR_EOF ? 0 : r;
			}
		}
		int r = av_read_frame(m->fmt, m->pkt);
		if (r == AVERROR_EOF) {
			if (m->bsf) {
				av_bsf_send_packet(m->bsf, NULL);
				continue;
			}
			return 0;
		}
		if (r < 0) {
			return r;
		}
		if (m->pkt->stream_index != m->stream) {
			continue;
		}
		if (!m->bsf) {
			break;
		}
		if ((r = av_bsf_send_packet(m->bsf, m->pkt)) < 0) {
			return r;
		}
	}
	int64_t ts = m->pkt->pts != AV_NOPTS_VALUE ? m->pkt->pts : m->pkt->dts;
	*pts_us = ts == AV_NOPTS_VALUE ? 0 : av_rescale_q(ts, m->tb, (AVRational){1, 1000000});
	*key = (m->pkt->flags & AV_PKT_FLAG_KEY) != 0;
	*data = m->pkt->data;
	return m->pkt->size;
}
