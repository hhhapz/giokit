// C side of package media: thin wrappers over libavcodec/libavformat so the
// Go side never touches FFmpeg structs directly.
#ifndef GK_AV_H
#define GK_AV_H

#include <stdint.h>

typedef struct gk_dec gk_dec;
typedef struct gk_demux gk_demux;

// Codec ids match media.Codec.
enum { GK_H264 = 1, GK_H265 = 2, GK_AV1 = 3, GK_VP8 = 4, GK_VP9 = 5 };
// Hardware modes match media.Hardware. AUTO uses the platform's API and
// falls back to software; GPU uses it or fails; VAAPI, D3D11VA and
// VIDEOTOOLBOX force one API or fail with AVERROR(ENODEV).
enum {
	GK_HW_AUTO = 0,
	GK_HW_OFF = 1,
	GK_HW_VAAPI = 2,
	GK_HW_D3D11VA = 3,
	GK_HW_VIDEOTOOLBOX = 4,
	GK_HW_GPU = 5,
};
// gk_hw_platform is the API AUTO and GPU use on this build: D3D11VA on
// Windows, VideoToolbox on macOS, VA-API elsewhere.
int gk_hw_platform(void);
// gk_zero_copy is 1 if this build can export hardware frames as DMA-BUFs
// (VA-API on Linux).
int gk_zero_copy(void);

// Decoder. Errors are negative AVERROR codes; gk_err formats them.
gk_dec *gk_dec_open(int codec, int hw, const char *device, int low_delay, int *err);
void gk_set_fail_hwdevice(int fail); // tests: every hwdevice creation fails
void gk_dec_close(gk_dec *d);
// gk_dec_hw is the hardware API the decoder opened (GK_HW_VAAPI, ...), or
// GK_HW_OFF in software.
int gk_dec_hw(gk_dec *d);
int gk_dec_send(gk_dec *d, const uint8_t *data, int size, int64_t pts, int key);
int gk_dec_flush(gk_dec *d); // signal end of stream
// gk_dec_receive returns 0 with a frame ready, AVERROR(EAGAIN) when it needs
// more input, AVERROR_EOF after a flush has drained.
int gk_dec_receive(gk_dec *d);
void gk_dec_frame_info(gk_dec *d, int *w, int *h, int64_t *pts, int *hw);
// gk_dec_frame_rgba converts the received frame to RGBA in dst (w*h, stride).
int gk_dec_frame_rgba(gk_dec *d, uint8_t *dst, int stride);
// gk_dec_frame_dmabuf maps the received hardware frame as one RGBA DMA-BUF
// plane, converting on the GPU (VA-API only). It fills the descriptor and returns an
// opaque handle to release with gk_dmabuf_release once the buffer is no
// longer sampled. Returns NULL (and *err) if unavailable.
typedef struct {
	int fd;
	uint32_t offset, pitch, fourcc;
	uint64_t modifier;
	int width, height;
	uint64_t id; // unique per buffer for the life of the process
} gk_dmabuf;
void *gk_dec_frame_dmabuf(gk_dec *d, gk_dmabuf *out, int *err);
void gk_dmabuf_release(void *h);

// Demuxer: packets from a file, in the shape a scrcpy device sends them
// (Annex B for H.264/H.265, codec config first).
gk_demux *gk_demux_open(const char *path, int *err);
void gk_demux_close(gk_demux *m);
void gk_demux_info(gk_demux *m, int *codec, int *w, int *h, int *fps_num, int *fps_den);
// gk_demux_config copies the codec configuration (SPS/PPS, AV1 sequence
// header) into buf and returns its size, or the size needed if larger.
int gk_demux_config(gk_demux *m, uint8_t *buf, int cap);
// gk_demux_read reads the next video packet. Returns its size (0 at the
// end), or a negative error. The data stays valid until the next call.
int gk_demux_read(gk_demux *m, const uint8_t **data, int64_t *pts_us, int *key);

void gk_err(int err, char *buf, int cap);
int gk_eagain(void);
int gk_eof(void);

#endif
