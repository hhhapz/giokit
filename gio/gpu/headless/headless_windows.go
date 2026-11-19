// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"os"
	"unsafe"

	"github.com/hhhapz/giokit/gio/gpu"
	"github.com/hhhapz/giokit/gio/internal/d3d11"
)

// driverTypeWARP is D3D_DRIVER_TYPE_WARP.
const driverTypeWARP = 5

type d3d11Context struct {
	dev *d3d11.Device
}

func init() {
	newContextPrimary = func() (context, error) {
		// giokit: fall back to WARP, the software rasterizer, when there is
		// no hardware device (CI runners without a GPU); GIOKIT_GPU=warp
		// forces it so tests can cover it on any machine.
		driver := uint32(d3d11.DRIVER_TYPE_HARDWARE)
		if os.Getenv("GIOKIT_GPU") == "warp" {
			driver = driverTypeWARP
		}
		dev, ctx, _, err := d3d11.CreateDevice(driver, 0)
		if err != nil && driver != driverTypeWARP {
			dev, ctx, _, err = d3d11.CreateDevice(driverTypeWARP, 0)
		}
		if err != nil {
			return nil, err
		}
		// Don't need it.
		d3d11.IUnknownRelease(unsafe.Pointer(ctx), ctx.Vtbl.Release)
		return &d3d11Context{dev: dev}, nil
	}
}

func (c *d3d11Context) API() gpu.API {
	return gpu.Direct3D11{Device: unsafe.Pointer(c.dev)}
}

func (c *d3d11Context) MakeCurrent() error {
	return nil
}

func (c *d3d11Context) ReleaseCurrent() {
}

func (c *d3d11Context) Release() {
	d3d11.IUnknownRelease(unsafe.Pointer(c.dev), c.dev.Vtbl.Release)
	c.dev = nil
}
