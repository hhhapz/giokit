package kit

import (
	"image"
	"strings"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/io/semantic"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/x/styledtext"
)

// TextSpan is a themed run in one wrapping paragraph. The zero-valued size
// uses body text; Strong and Muted select theme roles rather than raw colors.
type TextSpan struct {
	Text          string  // the run's text; spans join without separators
	Size          unit.Sp // 0 is the theme's body size
	Strong, Muted bool    // semibold weight; Muted instead of Fg colour
}

// StyledText renders a wrapping paragraph with mixed emphasis. Its area
// carries the whole text as one semantic label. It has no inline links: put
// a Link widget beside it.
func (t *Theme) StyledText(gtx layout.Context, spans ...TextSpan) layout.Dimensions {
	styles := make([]styledtext.SpanStyle, len(spans))
	var label strings.Builder
	for i, s := range spans {
		label.WriteString(s.Text)
		size := s.Size
		if size == 0 {
			size = t.Type.Body
		}
		fg := t.Fg
		if s.Muted {
			fg = t.Muted
		}
		weight := font.Normal
		if s.Strong {
			weight = font.SemiBold
		}
		styles[i] = styledtext.SpanStyle{
			Font: font.Font{Typeface: t.UI, Weight: weight}, Size: size,
			Color: fg, Content: s.Text,
		}
	}
	m := op.Record(gtx.Ops)
	d := styledtext.Text(t.Shaper, styles...).Layout(gtx, nil)
	call := m.Stop()
	defer clip.Rect{Max: image.Pt(d.Size.X, d.Size.Y)}.Push(gtx.Ops).Pop()
	semantic.LabelOp(label.String()).Add(gtx.Ops)
	call.Add(gtx.Ops)
	return d
}
