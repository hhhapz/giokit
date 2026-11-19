//go:build !linux

// Without VA-API there is no GPU conversion: frames take the RGBA path.
#include "vpp.h"

#include <errno.h>
#include <libavutil/error.h>

int gk_vpp_available(void) { return 0; }
gk_vpp *gk_vpp_open(AVBufferRef *hwdev, int w, int h, int *err) {
	*err = AVERROR(ENOSYS);
	return NULL;
}
int gk_vpp_fits(gk_vpp *v, int w, int h) { return 0; }
void *gk_vpp_convert(gk_vpp *v, const AVFrame *f, gk_dmabuf *out, int *err) {
	*err = AVERROR(ENOSYS);
	return NULL;
}
void gk_vpp_release(void *h) {}
void gk_vpp_close(gk_vpp *v) {}
