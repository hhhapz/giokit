package fonts_test

import (
	"errors"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/fonts"
	"github.com/hhhapz/giokit/kittest"
)

// With an installed CJK font as fallback, Chinese, Japanese and Korean
// draw without missing glyphs, and Latin text keeps the theme's own face.
func TestSystemCJK(t *testing.T) {
	cjk, err := fonts.SystemCJK()
	if errors.Is(err, fonts.ErrNoSystemFont) {
		t.Skip("no CJK font installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	th := kit.Dark()
	f := fonts.Go()
	f.Faces = append(f.Faces, cjk...)
	th.UseFonts(f)
	h := kittest.NewT(t, 400, 120, func(gtx layout.Context) {
		th.Text(gtx, "Settings 设置 設定 설정", th.Type.Body, th.Fg, kit.Regular)
	})
	h.Theme = th
	h.ExpectClean() // reports missing glyphs
	if cjk[0].Font.Typeface == "Go" {
		t.Fatal("the fallback must not take the theme's typeface name")
	}
}
