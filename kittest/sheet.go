package kittest

import (
	"image"
	"image/color"
	"image/draw"
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

// Snap records the current frame under a label for SaveSheet. Every snap is
// a full-size copy held until the harness is dropped, so snap the states
// worth reviewing, not every frame.
func (h *Harness) Snap(label string) {
	h.tb.Helper()
	h.snaps = append(h.snaps, snap{label: label, img: h.Image()})
}

// Contact sheet layout, in sheet pixels.
const (
	sheetLabelHeight = 18 // the caption strip above each frame
	sheetLabelInset  = 6  // caption text from the cell's left edge
	sheetBaseline    = 13 // caption baseline from the strip's top (basicfont 7x13)
)

// sheetBackground fills the gaps between frames.
var sheetBackground = color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}

// SaveSheet writes every Snap into one labeled grid PNG, cols frames wide,
// each frame downscaled by an integer factor (2 halves it). It creates the
// file's directory and writes nothing when there are no snaps. One image
// read instead of many: the cheap way for an agent to review several UI
// states.
func (h *Harness) SaveSheet(path string, cols, downscale int) error {
	h.tb.Helper()
	if len(h.snaps) == 0 {
		return nil
	}
	cols = max(1, min(cols, len(h.snaps)))
	downscale = max(1, downscale)
	cw, ch := ceilDiv(h.Size.X, downscale), ceilDiv(h.Size.Y, downscale)
	rows := ceilDiv(len(h.snaps), cols)
	sheet := image.NewRGBA(image.Rect(0, 0, cols*cw, rows*(ch+sheetLabelHeight)))
	draw.Draw(sheet, sheet.Rect, image.NewUniform(sheetBackground), image.Point{}, draw.Src)
	for i, s := range h.snaps {
		x, y := (i%cols)*cw, (i/cols)*(ch+sheetLabelHeight)
		d := font.Drawer{Dst: sheet, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(x+sheetLabelInset, y+sheetBaseline)}
		d.DrawString(s.label)
		shrink(sheet, image.Pt(x, y+sheetLabelHeight), s.img, downscale)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writePNG(path, sheet)
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// shrink box-filters src by factor k into dst at off. A partial block at the
// right or bottom edge averages the pixels it has, so no row or column of
// src is dropped.
func shrink(dst *image.RGBA, off image.Point, src *image.RGBA, k int) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	for y := 0; y < ceilDiv(sh, k); y++ {
		for x := 0; x < ceilDiv(sw, k); x++ {
			var r, g, b, n int
			for sy := y * k; sy < min((y+1)*k, sh); sy++ {
				for sx := x * k; sx < min((x+1)*k, sw); sx++ {
					i := src.PixOffset(src.Rect.Min.X+sx, src.Rect.Min.Y+sy)
					r += int(src.Pix[i])
					g += int(src.Pix[i+1])
					b += int(src.Pix[i+2])
					n++
				}
			}
			dst.SetRGBA(off.X+x, off.Y+y, color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 0xff})
		}
	}
}
