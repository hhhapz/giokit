// SPDX-License-Identifier: Unlicense OR MIT

//go:build linux || freebsd

package vk

// giokit patch: Vulkan 1.1 and DMA-BUF import (see docs/gio-patches.md).
//
// A DMA-BUF becomes a VkImage with the buffer's DRM format modifier, bound
// to memory imported from the buffer's file descriptor; nothing is copied.
// The renderer takes the image from the foreign queue family before
// sampling it and gives it back after (gpu/internal/vulkan), and waits for
// the producer's pending writes through a sync file exported from the
// buffer, where the kernel supports that (Linux 6.0).

/*
#define VK_NO_PROTOTYPES 1
#define VK_DEFINE_NON_DISPATCHABLE_HANDLE(object) typedef uint64_t object;
#include <vulkan/vulkan.h>
#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/ioctl.h>

// The instance- and physical-device-level entry points this file uses,
// resolved from the loader.
typedef struct {
	PFN_vkEnumerateInstanceVersion enumerateInstanceVersion;
	PFN_vkGetDeviceProcAddr getDeviceProcAddr;
	PFN_vkGetPhysicalDeviceProperties getPhysicalDeviceProperties;
	PFN_vkEnumerateDeviceExtensionProperties enumerateDeviceExtensionProperties;
	PFN_vkGetPhysicalDeviceFormatProperties2 getPhysicalDeviceFormatProperties2;
	PFN_vkGetPhysicalDeviceImageFormatProperties2 getPhysicalDeviceImageFormatProperties2;
	PFN_vkGetPhysicalDeviceExternalSemaphoreProperties getPhysicalDeviceExternalSemaphoreProperties;
	PFN_vkCreateImage createImage;
	PFN_vkDestroyImage destroyImage;
	PFN_vkGetImageMemoryRequirements getImageMemoryRequirements;
	PFN_vkAllocateMemory allocateMemory;
	PFN_vkFreeMemory freeMemory;
	PFN_vkBindImageMemory bindImageMemory;
	PFN_vkCreateImageView createImageView;
} gk_fns;

// The device-level entry points of the enabled extensions.
typedef struct {
	PFN_vkGetMemoryFdPropertiesKHR getMemoryFdProperties;
	PFN_vkImportSemaphoreFdKHR importSemaphoreFd;
} gk_dev_fns;

static uint32_t gk_instance_version(gk_fns *f) {
	uint32_t v = VK_API_VERSION_1_0;
	if (f->enumerateInstanceVersion == NULL || f->enumerateInstanceVersion(&v) != VK_SUCCESS) {
		return VK_API_VERSION_1_0;
	}
	return v;
}

static uint32_t gk_device_version(gk_fns *f, VkPhysicalDevice pd) {
	VkPhysicalDeviceProperties p;
	f->getPhysicalDeviceProperties(pd, &p);
	return p.apiVersion;
}

static VkResult gk_device_extensions(gk_fns *f, VkPhysicalDevice pd, uint32_t *n, VkExtensionProperties *props) {
	return f->enumerateDeviceExtensionProperties(pd, NULL, n, props);
}

static void gk_device_fns(gk_fns *f, VkDevice d, gk_dev_fns *out) {
	out->getMemoryFdProperties = (PFN_vkGetMemoryFdPropertiesKHR)f->getDeviceProcAddr(d, "vkGetMemoryFdPropertiesKHR");
	out->importSemaphoreFd = (PFN_vkImportSemaphoreFdKHR)f->getDeviceProcAddr(d, "vkImportSemaphoreFdKHR");
}

// gk_sync_fd_importable reports whether binary semaphores accept sync files.
static int gk_sync_fd_importable(gk_fns *f, VkPhysicalDevice pd) {
	VkPhysicalDeviceExternalSemaphoreInfo info = {
		.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_EXTERNAL_SEMAPHORE_INFO,
		.handleType = VK_EXTERNAL_SEMAPHORE_HANDLE_TYPE_SYNC_FD_BIT,
	};
	VkExternalSemaphoreProperties props = {.sType = VK_STRUCTURE_TYPE_EXTERNAL_SEMAPHORE_PROPERTIES};
	f->getPhysicalDeviceExternalSemaphoreProperties(pd, &info, &props);
	return (props.externalSemaphoreFeatures & VK_EXTERNAL_SEMAPHORE_FEATURE_IMPORTABLE_BIT) != 0;
}

// gk_modifier_planes returns the memory planes of modifier for format when
// images with it can be sampled, or 0.
static uint32_t gk_modifier_planes(gk_fns *f, VkPhysicalDevice pd, VkFormat format, uint64_t modifier) {
	VkDrmFormatModifierPropertiesListEXT list = {.sType = VK_STRUCTURE_TYPE_DRM_FORMAT_MODIFIER_PROPERTIES_LIST_EXT};
	VkFormatProperties2 props = {.sType = VK_STRUCTURE_TYPE_FORMAT_PROPERTIES_2, .pNext = &list};
	f->getPhysicalDeviceFormatProperties2(pd, format, &props);
	if (list.drmFormatModifierCount == 0) {
		return 0;
	}
	list.pDrmFormatModifierProperties = calloc(list.drmFormatModifierCount, sizeof(VkDrmFormatModifierPropertiesEXT));
	if (list.pDrmFormatModifierProperties == NULL) {
		return 0;
	}
	f->getPhysicalDeviceFormatProperties2(pd, format, &props);
	uint32_t planes = 0;
	for (uint32_t i = 0; i < list.drmFormatModifierCount; i++) {
		VkDrmFormatModifierPropertiesEXT m = list.pDrmFormatModifierProperties[i];
		if (m.drmFormatModifier == modifier && (m.drmFormatModifierTilingFeatures & VK_FORMAT_FEATURE_SAMPLED_IMAGE_BIT)) {
			planes = m.drmFormatModifierPlaneCount;
			break;
		}
	}
	free(list.pDrmFormatModifierProperties);
	return planes;
}

// gk_dmabuf_import creates an image of format, viewable as viewFormat, on
// the single-plane DMA-BUF fd, and binds it to memory imported from a
// duplicate of fd (Vulkan owns the duplicate; the caller keeps fd). On
// failure *why says which step failed.
static VkResult gk_dmabuf_import(gk_fns *f, gk_dev_fns *df, VkPhysicalDevice pd, VkDevice d,
                                 VkFormat format, VkFormat viewFormat, uint32_t w, uint32_t h,
                                 int fd, uint64_t offset, uint64_t pitch, uint64_t modifier,
                                 VkImage *outImg, VkDeviceMemory *outMem, const char **why) {
	*why = NULL;
	uint32_t planes = gk_modifier_planes(f, pd, format, modifier);
	if (planes == 0) {
		*why = "the format and modifier cannot be sampled";
		return VK_ERROR_FORMAT_NOT_SUPPORTED;
	}
	if (planes != 1) {
		*why = "the modifier has several memory planes (compressed); the buffer carries one";
		return VK_ERROR_FORMAT_NOT_SUPPORTED;
	}
	int mutable = viewFormat != format;
	VkFormat formats[2] = {format, viewFormat};
	VkImageFormatListCreateInfo list = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_FORMAT_LIST_CREATE_INFO,
		.viewFormatCount = 2,
		.pViewFormats = formats,
	};
	VkImageCreateFlags flags = mutable ? VK_IMAGE_CREATE_MUTABLE_FORMAT_BIT : 0;

	VkPhysicalDeviceImageDrmFormatModifierInfoEXT modInfo = {
		.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_IMAGE_DRM_FORMAT_MODIFIER_INFO_EXT,
		.pNext = mutable ? &list : NULL,
		.drmFormatModifier = modifier,
		.sharingMode = VK_SHARING_MODE_EXCLUSIVE,
	};
	VkPhysicalDeviceExternalImageFormatInfo extInfo = {
		.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_EXTERNAL_IMAGE_FORMAT_INFO,
		.pNext = &modInfo,
		.handleType = VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT,
	};
	VkPhysicalDeviceImageFormatInfo2 fmtInfo = {
		.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_IMAGE_FORMAT_INFO_2,
		.pNext = &extInfo,
		.format = format,
		.type = VK_IMAGE_TYPE_2D,
		.tiling = VK_IMAGE_TILING_DRM_FORMAT_MODIFIER_EXT,
		.usage = VK_IMAGE_USAGE_SAMPLED_BIT,
		.flags = flags,
	};
	VkExternalImageFormatProperties extProps = {.sType = VK_STRUCTURE_TYPE_EXTERNAL_IMAGE_FORMAT_PROPERTIES};
	VkImageFormatProperties2 fmtProps = {.sType = VK_STRUCTURE_TYPE_IMAGE_FORMAT_PROPERTIES_2, .pNext = &extProps};
	VkResult r = f->getPhysicalDeviceImageFormatProperties2(pd, &fmtInfo, &fmtProps);
	if (r != VK_SUCCESS) {
		*why = "vkGetPhysicalDeviceImageFormatProperties2: the image cannot be imported";
		return r;
	}
	if (!(extProps.externalMemoryProperties.externalMemoryFeatures & VK_EXTERNAL_MEMORY_FEATURE_IMPORTABLE_BIT)) {
		*why = "DMA-BUF memory is not importable for this format and modifier";
		return VK_ERROR_FORMAT_NOT_SUPPORTED;
	}
	VkExtent3D max = fmtProps.imageFormatProperties.maxExtent;
	if (w > max.width || h > max.height) {
		*why = "the image is larger than the device allows";
		return VK_ERROR_FORMAT_NOT_SUPPORTED;
	}

	VkSubresourceLayout plane = {.offset = offset, .rowPitch = pitch};
	VkImageDrmFormatModifierExplicitCreateInfoEXT explicit = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_DRM_FORMAT_MODIFIER_EXPLICIT_CREATE_INFO_EXT,
		.drmFormatModifier = modifier,
		.drmFormatModifierPlaneCount = 1,
		.pPlaneLayouts = &plane,
	};
	VkExternalMemoryImageCreateInfo extImg = {
		.sType = VK_STRUCTURE_TYPE_EXTERNAL_MEMORY_IMAGE_CREATE_INFO,
		.pNext = &explicit,
		.handleTypes = VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT,
	};
	list.pNext = &extImg;
	VkImageCreateInfo ici = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO,
		.pNext = mutable ? (void *)&list : (void *)&extImg,
		.flags = flags,
		.imageType = VK_IMAGE_TYPE_2D,
		.format = format,
		.extent = {w, h, 1},
		.mipLevels = 1,
		.arrayLayers = 1,
		.samples = VK_SAMPLE_COUNT_1_BIT,
		.tiling = VK_IMAGE_TILING_DRM_FORMAT_MODIFIER_EXT,
		.usage = VK_IMAGE_USAGE_SAMPLED_BIT,
		.sharingMode = VK_SHARING_MODE_EXCLUSIVE,
		.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED,
	};
	VkImage img;
	r = f->createImage(d, &ici, NULL, &img);
	if (r != VK_SUCCESS) {
		*why = "vkCreateImage";
		return r;
	}

	VkMemoryFdPropertiesKHR fdProps = {.sType = VK_STRUCTURE_TYPE_MEMORY_FD_PROPERTIES_KHR};
	r = df->getMemoryFdProperties(d, VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT, fd, &fdProps);
	if (r != VK_SUCCESS) {
		f->destroyImage(d, img, NULL);
		*why = "vkGetMemoryFdPropertiesKHR";
		return r;
	}
	VkMemoryRequirements reqs;
	f->getImageMemoryRequirements(d, img, &reqs);
	uint32_t types = reqs.memoryTypeBits & fdProps.memoryTypeBits;
	off_t size = lseek(fd, 0, SEEK_END);
	if (types == 0 || size < 0 || (VkDeviceSize)size < reqs.size) {
		f->destroyImage(d, img, NULL);
		*why = types == 0 ? "no memory type fits both the image and the buffer" : "the buffer is smaller than the image needs";
		return VK_ERROR_INVALID_EXTERNAL_HANDLE;
	}
	int dup = fcntl(fd, F_DUPFD_CLOEXEC, 0);
	if (dup < 0) {
		f->destroyImage(d, img, NULL);
		*why = "dup";
		return VK_ERROR_TOO_MANY_OBJECTS;
	}
	VkImportMemoryFdInfoKHR imp = {
		.sType = VK_STRUCTURE_TYPE_IMPORT_MEMORY_FD_INFO_KHR,
		.handleType = VK_EXTERNAL_MEMORY_HANDLE_TYPE_DMA_BUF_BIT_EXT,
		.fd = dup,
	};
	VkMemoryDedicatedAllocateInfo dedicated = {
		.sType = VK_STRUCTURE_TYPE_MEMORY_DEDICATED_ALLOCATE_INFO,
		.pNext = &imp,
		.image = img,
	};
	VkMemoryAllocateInfo alloc = {
		.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO,
		.pNext = &dedicated,
		.allocationSize = (VkDeviceSize)size,
		.memoryTypeIndex = (uint32_t)__builtin_ctz(types),
	};
	VkDeviceMemory mem;
	r = f->allocateMemory(d, &alloc, NULL, &mem);
	if (r != VK_SUCCESS) {
		close(dup); // ownership passes to Vulkan only on success
		f->destroyImage(d, img, NULL);
		*why = "vkAllocateMemory (import)";
		return r;
	}
	r = f->bindImageMemory(d, img, mem, 0);
	if (r != VK_SUCCESS) {
		f->freeMemory(d, mem, NULL);
		f->destroyImage(d, img, NULL);
		*why = "vkBindImageMemory";
		return r;
	}
	*outImg = img;
	*outMem = mem;
	return VK_SUCCESS;
}

static VkResult gk_dmabuf_view(gk_fns *f, VkDevice d, VkImage img, VkFormat format, int opaque, VkImageView *out) {
	VkImageViewCreateInfo inf = {
		.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO,
		.image = img,
		.viewType = VK_IMAGE_VIEW_TYPE_2D,
		.format = format,
		.components = {
			.r = VK_COMPONENT_SWIZZLE_IDENTITY,
			.g = VK_COMPONENT_SWIZZLE_IDENTITY,
			.b = VK_COMPONENT_SWIZZLE_IDENTITY,
			.a = opaque ? VK_COMPONENT_SWIZZLE_ONE : VK_COMPONENT_SWIZZLE_IDENTITY,
		},
		.subresourceRange = {
			.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT,
			.levelCount = 1,
			.layerCount = 1,
		},
	};
	return f->createImageView(d, &inf, NULL, out);
}

// gk_sync_file mirrors struct dma_buf_export_sync_file (linux/dma-buf.h,
// Linux 6.0), declared here so older kernel headers build.
struct gk_sync_file {
	uint32_t flags;
	int32_t fd;
};

// gk_dmabuf_write_fence returns a sync file that signals when the
// buffer's pending writes are done (DMA_BUF_IOCTL_EXPORT_SYNC_FILE with
// DMA_BUF_SYNC_READ), or -errno.
static int gk_dmabuf_write_fence(int fd) {
#ifdef __linux__
	struct gk_sync_file a = {.flags = 1, .fd = -1};
	if (ioctl(fd, _IOWR('b', 2, struct gk_sync_file), &a) != 0) {
		return -errno;
	}
	return a.fd;
#else
	return -ENOTTY;
#endif
}

// gk_import_sync_file makes sem's next wait wait for the sync file fd, and
// takes ownership of fd on success.
static VkResult gk_import_sync_file(gk_dev_fns *df, VkDevice d, VkSemaphore sem, int fd) {
	VkImportSemaphoreFdInfoKHR inf = {
		.sType = VK_STRUCTURE_TYPE_IMPORT_SEMAPHORE_FD_INFO_KHR,
		.semaphore = sem,
		.flags = VK_SEMAPHORE_IMPORT_TEMPORARY_BIT,
		.handleType = VK_EXTERNAL_SEMAPHORE_HANDLE_TYPE_SYNC_FD_BIT,
		.fd = fd,
	};
	return df->importSemaphoreFd(d, &inf);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// gkFns is in C memory, so C helpers may take it. Written once, by vkInit.
var gkFns = (*C.gk_fns)(C.calloc(1, C.size_t(unsafe.Sizeof(C.gk_fns{}))))

// QUEUE_FAMILY_FOREIGN_EXT names the owner of an image shared with another
// driver or API (VK_EXT_queue_family_foreign).
const QUEUE_FAMILY_FOREIGN_EXT = uint32(C.VK_QUEUE_FAMILY_FOREIGN_EXT)

// FORMAT_B8G8R8A8_UNORM is the storage format of imported BGRA buffers.
const FORMAT_B8G8R8A8_UNORM Format = C.VK_FORMAT_B8G8R8A8_UNORM

func init() {
	loadFuncs = append(loadFuncs, func(dlopen func(name string) *[0]byte) {
		f := gkFns
		f.enumerateInstanceVersion = C.PFN_vkEnumerateInstanceVersion(dlopen("vkEnumerateInstanceVersion"))
		f.getDeviceProcAddr = C.PFN_vkGetDeviceProcAddr(dlopen("vkGetDeviceProcAddr"))
		f.getPhysicalDeviceProperties = C.PFN_vkGetPhysicalDeviceProperties(dlopen("vkGetPhysicalDeviceProperties"))
		f.enumerateDeviceExtensionProperties = C.PFN_vkEnumerateDeviceExtensionProperties(dlopen("vkEnumerateDeviceExtensionProperties"))
		f.getPhysicalDeviceFormatProperties2 = C.PFN_vkGetPhysicalDeviceFormatProperties2(dlopen("vkGetPhysicalDeviceFormatProperties2"))
		f.getPhysicalDeviceImageFormatProperties2 = C.PFN_vkGetPhysicalDeviceImageFormatProperties2(dlopen("vkGetPhysicalDeviceImageFormatProperties2"))
		f.getPhysicalDeviceExternalSemaphoreProperties = C.PFN_vkGetPhysicalDeviceExternalSemaphoreProperties(dlopen("vkGetPhysicalDeviceExternalSemaphoreProperties"))
		f.createImage = C.PFN_vkCreateImage(dlopen("vkCreateImage"))
		f.destroyImage = C.PFN_vkDestroyImage(dlopen("vkDestroyImage"))
		f.getImageMemoryRequirements = C.PFN_vkGetImageMemoryRequirements(dlopen("vkGetImageMemoryRequirements"))
		f.allocateMemory = C.PFN_vkAllocateMemory(dlopen("vkAllocateMemory"))
		f.freeMemory = C.PFN_vkFreeMemory(dlopen("vkFreeMemory"))
		f.bindImageMemory = C.PFN_vkBindImageMemory(dlopen("vkBindImageMemory"))
		f.createImageView = C.PFN_vkCreateImageView(dlopen("vkCreateImageView"))
	})
}

// instanceAPIVersion is the API version CreateInstance asks for: 1.1 where
// the loader has it (DMA-BUF import needs it), else 1.0. Call it after
// vkInit.
func instanceAPIVersion() C.uint32_t {
	if C.gk_instance_version(gkFns) >= C.VK_API_VERSION_1_1 {
		return C.VK_API_VERSION_1_1
	}
	return C.VK_API_VERSION_1_0
}

// dmabufExtensions are the device extensions DMA-BUF import needs, beyond
// Vulkan 1.1. VK_KHR_external_semaphore_fd, for waiting on the producer's
// writes, is enabled too where present (dmabufSyncExtension).
var dmabufExtensions = []string{
	"VK_KHR_external_memory_fd",
	"VK_EXT_external_memory_dma_buf",
	"VK_KHR_image_format_list",
	"VK_EXT_image_drm_format_modifier",
	"VK_EXT_queue_family_foreign",
}

const dmabufSyncExtension = "VK_KHR_external_semaphore_fd"

// DMABufDeviceExtensions returns the device extensions to enable for
// DMA-BUF import on pd, or nil when pd, the loader or the instance (made by
// CreateInstance) cannot import DMA-BUFs.
func DMABufDeviceExtensions(pd PhysicalDevice) []string {
	f := gkFns
	if f.getPhysicalDeviceFormatProperties2 == nil || f.getPhysicalDeviceImageFormatProperties2 == nil ||
		f.getPhysicalDeviceExternalSemaphoreProperties == nil || f.getDeviceProcAddr == nil ||
		instanceAPIVersion() < C.VK_API_VERSION_1_1 || C.gk_device_version(f, pd) < C.VK_API_VERSION_1_1 {
		return nil
	}
	var n C.uint32_t
	if C.gk_device_extensions(f, pd, &n, nil) != C.VK_SUCCESS || n == 0 {
		return nil
	}
	props := make([]C.VkExtensionProperties, n)
	if C.gk_device_extensions(f, pd, &n, &props[0]) != C.VK_SUCCESS {
		return nil
	}
	have := make(map[string]bool, n)
	for _, p := range props[:n] {
		have[C.GoString(&p.extensionName[0])] = true
	}
	for _, e := range dmabufExtensions {
		if !have[e] {
			return nil
		}
	}
	exts := append([]string(nil), dmabufExtensions...)
	if have[dmabufSyncExtension] {
		exts = append(exts, dmabufSyncExtension)
	}
	return exts
}

// DMABufDevice imports DMA-BUFs into a device created with the extensions
// DMABufDeviceExtensions returned. It is immutable after creation.
type DMABufDevice struct {
	pd   PhysicalDevice
	dev  Device
	fns  *C.gk_dev_fns // C memory: the importer passes it to C
	sync bool          // semaphores can wait on sync files
}

// NewDMABufDevice resolves dev's DMA-BUF entry points.
func NewDMABufDevice(pd PhysicalDevice, dev Device) (*DMABufDevice, error) {
	fns := (*C.gk_dev_fns)(C.calloc(1, C.size_t(unsafe.Sizeof(C.gk_dev_fns{}))))
	C.gk_device_fns(gkFns, dev, fns)
	if fns.getMemoryFdProperties == nil {
		C.free(unsafe.Pointer(fns))
		return nil, errors.New("vulkan: VK_KHR_external_memory_fd is not enabled")
	}
	d := &DMABufDevice{pd: pd, dev: dev, fns: fns}
	d.sync = fns.importSemaphoreFd != nil && C.gk_sync_fd_importable(gkFns, pd) != 0
	return d, nil
}

// Release frees d's resources; d must not be used after.
func (d *DMABufDevice) Release() {
	C.free(unsafe.Pointer(d.fns))
	d.fns = nil
}

// ImportDMABuf creates a sampled image of format on the single-plane
// DMA-BUF fd (which the caller keeps) and a view of it as viewFormat
// (alpha forced to 1 when opaque). The image starts in the foreign queue
// family's ownership.
func (d *DMABufDevice) ImportDMABuf(format, viewFormat Format, opaque bool, width, height, fd int, offset, pitch uint32, modifier uint64) (Image, DeviceMemory, ImageView, error) {
	var (
		img Image
		mem DeviceMemory
		why *C.char
	)
	r := C.gk_dmabuf_import(gkFns, d.fns, d.pd, d.dev, format, viewFormat, C.uint32_t(width), C.uint32_t(height),
		C.int(fd), C.uint64_t(offset), C.uint64_t(pitch), C.uint64_t(modifier), &img, &mem, &why)
	if err := vkErr(r); err != nil {
		return 0, 0, 0, fmt.Errorf("vulkan: DMA-BUF %dx%d format %d modifier %#x: %s: %w", width, height, format, modifier, C.GoString(why), err)
	}
	op := C.int(0)
	if opaque {
		op = 1
	}
	var view ImageView
	if err := vkErr(C.gk_dmabuf_view(gkFns, d.dev, img, viewFormat, op, &view)); err != nil {
		FreeMemory(d.dev, mem)
		DestroyImage(d.dev, img)
		return 0, 0, 0, fmt.Errorf("vulkan: DMA-BUF view: %w", err)
	}
	return img, mem, view, nil
}

// WaitForWrites returns a new semaphore that, waited on, waits for the
// pending writes to the DMA-BUF fd. It returns 0 and no error when that
// cannot be expressed (no VK_KHR_external_semaphore_fd, or a kernel before
// Linux 6.0): the producer must then have finished writing before handing
// the buffer over. The caller destroys the semaphore once the wait ran.
func (d *DMABufDevice) WaitForWrites(fd int) (Semaphore, error) {
	if !d.sync {
		return 0, nil
	}
	sf := C.gk_dmabuf_write_fence(C.int(fd))
	if sf < 0 {
		if e := syscall.Errno(-sf); e == syscall.ENOTTY || e == syscall.EINVAL {
			return 0, nil // the kernel predates sync file export
		}
		return 0, fmt.Errorf("vulkan: DMA_BUF_IOCTL_EXPORT_SYNC_FILE: %w", syscall.Errno(-sf))
	}
	sem, err := CreateSemaphore(d.dev)
	if err != nil {
		syscall.Close(int(sf))
		return 0, err
	}
	if err := vkErr(C.gk_import_sync_file(d.fns, d.dev, sem, sf)); err != nil {
		syscall.Close(int(sf))
		DestroySemaphore(d.dev, sem)
		return 0, fmt.Errorf("vulkan: vkImportSemaphoreFdKHR: %w", err)
	}
	return sem, nil
}

// BuildQueueTransferBarrier is BuildImageMemoryBarrier for the single-level
// image img that also moves it between queue families srcQueue and
// dstQueue.
func BuildQueueTransferBarrier(img Image, srcMask, dstMask AccessFlags, oldLayout, newLayout ImageLayout, srcQueue, dstQueue uint32) ImageMemoryBarrier {
	b := BuildImageMemoryBarrier(img, srcMask, dstMask, oldLayout, newLayout, 0, 1)
	b.srcQueueFamilyIndex = C.uint32_t(srcQueue)
	b.dstQueueFamilyIndex = C.uint32_t(dstQueue)
	return b
}
