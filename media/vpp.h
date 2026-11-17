// VA-API video processing: converts decoded (NV12) surfaces to RGBA
// surfaces on the GPU and exports them as DMA-BUFs, without a CPU copy.
#ifndef GK_VPP_H
#define GK_VPP_H

#include <libavutil/buffer.h>
#include <libavutil/frame.h>

#include "av.h"

typedef struct gk_vpp gk_vpp;

gk_vpp *gk_vpp_open(AVBufferRef *hwdev, int w, int h, int *err);
int gk_vpp_fits(gk_vpp *v, int w, int h);
// gk_vpp_convert converts f into a free output surface and describes it in
// out. The returned handle keeps the surface reserved until gk_vpp_release.
void *gk_vpp_convert(gk_vpp *v, const AVFrame *f, gk_dmabuf *out, int *err);
void gk_vpp_release(void *h);
// gk_vpp_close drops the caller's reference; surfaces still held stay valid
// until released.
void gk_vpp_close(gk_vpp *v);

#endif
