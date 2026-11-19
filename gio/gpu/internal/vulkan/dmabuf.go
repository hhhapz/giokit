// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux || freebsd) && !novulkan

package vulkan

// DMA-BUF import (giokit patch). An imported image is shared with the
// buffer's producer, so it belongs to the foreign queue family between
// frames: PrepareTexture acquires it (waiting for the producer's pending
// writes where the kernel can export them), and EndFrame releases it. Gio
// waits for a frame's fence before building the next frame, and the
// renderer hands a buffer back to its producer only once a later frame has
// bound a newer one, so the producer never writes a buffer the GPU is still
// reading.

import (
	"errors"
	"syscall"

	"github.com/hhhapz/giokit/gio/gpu/internal/driver"
	"github.com/hhhapz/giokit/gio/internal/vk"
)

// dmabufImage is the DMA-BUF state of an imported Texture.
type dmabufImage struct {
	fd       int  // a duplicate of the buffer's descriptor, for sync file export
	acquired bool // owned by the renderer's queue: acquired this frame
}

func (d *dmabufImage) release() {
	syscall.Close(d.fd)
}

// DRM fourccs of the single-plane 8-bit RGB formats importable here.
const (
	fourccABGR8888 = 'A' | 'B'<<8 | '2'<<16 | '4'<<24 // bytes R, G, B, A
	fourccXBGR8888 = 'X' | 'B'<<8 | '2'<<16 | '4'<<24 // bytes R, G, B, x
	fourccARGB8888 = 'A' | 'R'<<8 | '2'<<16 | '4'<<24 // bytes B, G, R, A
	fourccXRGB8888 = 'X' | 'R'<<8 | '2'<<16 | '4'<<24 // bytes B, G, R, x
)

// ImportDMABuf implements driver.DMABufImporter. The image's storage
// format is UNORM, sampled through an sRGB view so it decodes like every
// other Gio texture.
func (b *Backend) ImportDMABuf(d driver.DMABuf, filter driver.TextureFilter) (driver.Texture, error) {
	if b.dmabuf == nil {
		return nil, errors.New("vulkan: DMA-BUF import unsupported (Vulkan 1.1 with VK_EXT_external_memory_dma_buf, VK_EXT_image_drm_format_modifier and VK_EXT_queue_family_foreign needed)")
	}
	var format, view vk.Format
	opaque := false
	switch d.Fourcc {
	case fourccXBGR8888:
		opaque = true
		fallthrough
	case fourccABGR8888:
		format, view = vk.FORMAT_R8G8B8A8_UNORM, vk.FORMAT_R8G8B8A8_SRGB
	case fourccXRGB8888:
		opaque = true
		fallthrough
	case fourccARGB8888:
		format, view = vk.FORMAT_B8G8R8A8_UNORM, vk.FORMAT_B8G8R8A8_SRGB
	default:
		return nil, errors.New("vulkan: DMA-BUF import supports ABGR8888, XBGR8888, ARGB8888 and XRGB8888 only")
	}
	if d.Modifier == drmFormatModInvalid {
		return nil, errors.New("vulkan: DMA-BUF import needs an explicit format modifier")
	}
	fd, err := dupCloexec(d.FD)
	if err != nil {
		return nil, err
	}
	img, mem, iv, err := b.dmabuf.ImportDMABuf(format, view, opaque, d.Width, d.Height, d.FD, d.Offset, d.Pitch, d.Modifier)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	f := vk.FILTER_LINEAR
	if filter == driver.FilterNearest {
		f = vk.FILTER_NEAREST
	}
	sampler, err := vk.CreateSampler(b.dev, f, f, vk.SAMPLER_MIPMAP_MODE_NEAREST)
	if err != nil {
		vk.DestroyImageView(b.dev, iv)
		vk.DestroyImage(b.dev, img)
		vk.FreeMemory(b.dev, mem)
		syscall.Close(fd)
		return nil, mapErr(err)
	}
	return &Texture{
		backend: b, img: img, mem: mem, view: iv, sampler: sampler,
		format: view, mipmaps: 1, width: d.Width, height: d.Height,
		layout: vk.IMAGE_LAYOUT_GENERAL, passLayout: vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
		dmabuf: &dmabufImage{fd: fd},
	}, nil
}

// drmFormatModInvalid is DRM_FORMAT_MOD_INVALID: the layout is implied by
// the producer's driver, which Vulkan cannot import.
const drmFormatModInvalid = 0x00ffffffffffffff

// acquireDMABuf takes t from its producer for sampling in this frame's
// fragment shaders.
func (b *Backend) acquireDMABuf(cmdBuf vk.CommandBuffer, t *Texture) {
	// A failed export leaves no wait; the producer is expected to have
	// finished writing (paint.DMABuf), as it must where the kernel cannot
	// export the buffer's fences.
	if sem, err := b.dmabuf.WaitForWrites(t.dmabuf.fd); err == nil && sem != 0 {
		b.waitSems = append(b.waitSems, sem)
		b.waitStages = append(b.waitStages, vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT)
		b.deferFunc(func(d vk.Device) { vk.DestroySemaphore(d, sem) })
	}
	// The producer leaves the image in the GENERAL layout; transitioning
	// from UNDEFINED could discard its contents.
	bar := vk.BuildQueueTransferBarrier(t.img, 0, vk.ACCESS_SHADER_READ_BIT,
		vk.IMAGE_LAYOUT_GENERAL, vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
		vk.QUEUE_FAMILY_FOREIGN_EXT, uint32(b.queueFamily))
	vk.CmdPipelineBarrier(cmdBuf, vk.PIPELINE_STAGE_TOP_OF_PIPE_BIT, vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT, 0, nil, nil, []vk.ImageMemoryBarrier{bar})
	t.layout = vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL
	t.scope.stage = vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT
	t.scope.access = vk.ACCESS_SHADER_READ_BIT
	t.dmabuf.acquired = true
	b.dmabufUsed = append(b.dmabufUsed, t)
}

// releaseDMABufs gives the images acquired this frame back to their
// producers, at the end of the frame's last command buffer.
func (b *Backend) releaseDMABufs() {
	if len(b.dmabufUsed) == 0 {
		return
	}
	var bars []vk.ImageMemoryBarrier
	for _, t := range b.dmabufUsed {
		if t.dmabuf == nil {
			continue // released during the frame
		}
		bars = append(bars, vk.BuildQueueTransferBarrier(t.img, vk.ACCESS_SHADER_READ_BIT, 0,
			vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, vk.IMAGE_LAYOUT_GENERAL,
			uint32(b.queueFamily), vk.QUEUE_FAMILY_FOREIGN_EXT))
		t.layout = vk.IMAGE_LAYOUT_GENERAL
		t.scope.stage, t.scope.access = 0, 0
		t.dmabuf.acquired = false
	}
	clear(b.dmabufUsed)
	b.dmabufUsed = b.dmabufUsed[:0]
	if len(bars) > 0 {
		vk.CmdPipelineBarrier(b.ensureCmdBuf(), vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT, vk.PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, 0, nil, nil, bars)
	}
}

func dupCloexec(fd int) (int, error) {
	r, _, e := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_DUPFD_CLOEXEC, 0)
	if e != 0 {
		return -1, e
	}
	return int(r), nil
}
