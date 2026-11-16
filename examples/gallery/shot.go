//go:build !js

package main

import "github.com/hhhapz/giokit/kittest"

// renderShot renders the gallery offscreen (EGL) to a PNG. Not in the
// browser build: it needs a headless GPU context.
func renderShot(g *gallery, path string) error {
	h, err := kittest.New(1100, 720, 2, g.Frame)
	if err != nil {
		return err
	}
	defer h.Release()
	return h.Save(path)
}
