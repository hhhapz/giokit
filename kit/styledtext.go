package kit

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/x/styledtext"
)

// TextSpan is a themed run in one wrapping paragraph. The zero-valued size
// uses body text; Strong and Muted select theme roles rather than raw colors.
type TextSpan struct {
	Text          string
	Size          unit.Sp
	Strong, Muted bool
}

// StyledText renders a wrapping paragraph with mixed emphasis. Links remain
// separate Link widgets until keyboard and semantic support for inline links
// can be provided alongside pointer interaction.
func (t *Theme) StyledText(gtx C, spans ...TextSpan) D {
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
	d := styledtext.Text(t.Material().Shaper, styles...).Layout(gtx, nil)
	call := m.Stop()
	defer clip.Rect{Max: image.Pt(d.Size.X, d.Size.Y)}.Push(gtx.Ops).Pop()
	semantic.LabelOp(label.String()).Add(gtx.Ops)
	call.Add(gtx.Ops)
	return d
}
