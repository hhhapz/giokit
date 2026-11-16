package kittest

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type snap struct {
	label string
	img   *image.RGBA
}

// Snap records the current frame under a label for SaveSheet.
func (h *Harness) Snap(label string) { h.snaps = append(h.snaps, snap{label, h.Image()}) }

// SaveSheet writes every Snap into one labeled grid PNG, downscaled by an
// integer factor (2 halves each frame). One image read instead of many:
// the cheap way for an agent to review several UI states.
func (h *Harness) SaveSheet(path string, cols, downscale int) error {
	if len(h.snaps) == 0 {
		return nil
	}
	cols = max(1, min(cols, len(h.snaps)))
	downscale = max(1, downscale)
	cw, ch := h.Size.X/downscale, h.Size.Y/downscale
	const label = 18
	rows := int(math.Ceil(float64(len(h.snaps)) / float64(cols)))
	sheet := image.NewRGBA(image.Rect(0, 0, cols*cw, rows*(ch+label)))
	draw.Draw(sheet, sheet.Rect, image.NewUniform(color.RGBA{0x10, 0x10, 0x10, 0xff}), image.Point{}, draw.Src)
	face := basicfont.Face7x13
	for i, s := range h.snaps {
		x, y := (i%cols)*cw, (i/cols)*(ch+label)
		d := font.Drawer{Dst: sheet, Src: image.White, Face: face, Dot: fixed.P(x+6, y+13)}
		d.DrawString(s.label)
		shrink(sheet, image.Pt(x, y+label), s.img, downscale)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}

// shrink box-filters src by factor k into dst at off.
func shrink(dst *image.RGBA, off image.Point, src *image.RGBA, k int) {
	w, h := src.Rect.Dx()/k, src.Rect.Dy()/k
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, n int
			for dy := 0; dy < k; dy++ {
				for dx := 0; dx < k; dx++ {
					i := src.PixOffset(x*k+dx, y*k+dy)
					r, g, b, n = r+int(src.Pix[i]), g+int(src.Pix[i+1]), b+int(src.Pix[i+2]), n+1
				}
			}
			dst.SetRGBA(off.X+x, off.Y+y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 0xff})
		}
	}
}
