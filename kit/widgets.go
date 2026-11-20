package kit

import (
	"image/color"
	"strings"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
	"github.com/hhhapz/giokit/gio/op/paint"
	"github.com/hhhapz/giokit/gio/unit"
	"github.com/hhhapz/giokit/gio/widget"
)

// Text draws a single line of text in the UI typeface, truncated to the
// width available. Its area hugs the text.
func (t *Theme) Text(gtx layout.Context, s string, size unit.Sp, c color.NRGBA, w font.Weight) layout.Dimensions {
	return t.label(gtx, s, font.Font{Typeface: t.UI, Weight: w}, size, c, 1, 0)
}

// Paragraph draws wrapping body text at the regular weight.
func (t *Theme) Paragraph(gtx layout.Context, s string, c color.NRGBA) layout.Dimensions {
	return t.label(gtx, s, font.Font{Typeface: t.UI}, t.Type.Body, c, 0, bodyLineHeight)
}

// ParagraphLine is Paragraph with a font weight: it wraps too. It suits a
// block of text whose emphasis changes with state, such as the current line
// of lyrics.
func (t *Theme) ParagraphLine(gtx layout.Context, s string, c color.NRGBA, w font.Weight) layout.Dimensions {
	return t.label(gtx, s, font.Font{Typeface: t.UI, Weight: w}, t.Type.Body, c, 0, bodyLineHeight)
}

// Code draws a single line in the monospace typeface (logs, ids, paths).
func (t *Theme) Code(gtx layout.Context, s string, size unit.Sp, c color.NRGBA) layout.Dimensions {
	return t.label(gtx, s, font.Font{Typeface: t.Mono}, size, c, 1, 0)
}

// CodeBlock is Code that wraps: monospace paragraphs (previews, output).
func (t *Theme) CodeBlock(gtx layout.Context, s string, size unit.Sp, c color.NRGBA) layout.Dimensions {
	return t.label(gtx, s, font.Font{Typeface: t.Mono}, size, c, 0, 0)
}

// Heading is a page or section title.
func (t *Theme) Heading(gtx layout.Context, s string) layout.Dimensions {
	return t.Text(gtx, s, t.Type.Title, t.Fg, t.HeadingWeight)
}

// Caption is the small uppercase label used for section and column headings.
// It uppercases s with strings.ToUpper, Unicode's language-independent
// mapping: for a language with its own casing rules (Turkish dotted i), pass
// s already uppercased.
func (t *Theme) Caption(gtx layout.Context, s string) layout.Dimensions {
	return t.Text(gtx, strings.ToUpper(s), t.Type.Caption, t.Dim, Bold)
}

// Pill padding, in dp, unscaled by density so badges keep their shape. Text
// sits a little high in its line box, so a pill has more padding below.
const (
	pillPadTop    unit.Dp = 2
	pillPadBottom unit.Dp = 3
	pillPadX      unit.Dp = 9
)

// Pill is a rounded status badge.
func (t *Theme) Pill(gtx layout.Context, s string, fg, bg color.NRGBA) layout.Dimensions {
	inset := layout.Inset{Top: pillPadTop, Bottom: pillPadBottom, Left: pillPadX, Right: pillPadX}
	return t.Styled(Surface{Bg: bg, Radius: t.RadiusFull()}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return t.Text(gtx, s, t.Type.Small, fg, Medium)
		})
	})
}

// Card is a padded panel with a hairline outline.
func (t *Theme) Card(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return t.Styled(Surface{Bg: t.Panel, Border: t.Divider, Radius: t.RadiusLg()}).Layout(gtx, Pad(t.Space(Space3p5), t.Space(Space4), w))
}

// Bar is a full-width panel strip (title bar, status bar, toolbars).
func (t *Theme) Bar(gtx layout.Context, v, h unit.Dp, w layout.Widget) layout.Dimensions {
	return Fill(gtx, t.Panel, Stretch(Pad(t.Space(v), t.Space(h), w)))
}

// label draws s with the theme's shaper in f at size and color c: at most
// maxLines lines (0 is unlimited; a cut line ends in an ellipsis), with
// lineHeight times the font's line height (0 is the font's own). The
// fork's widget.Label clips to the laid-out text, so its area hugs the
// text.
func (t *Theme) label(gtx layout.Context, s string, f font.Font, size unit.Sp, c color.NRGBA, maxLines int, lineHeight float32) layout.Dimensions {
	return widget.Label{MaxLines: maxLines, LineHeightScale: lineHeight}.Layout(gtx, t.Shaper, f, size, s, colorOp(gtx, c))
}

// colorOp records a paint.ColorOp for c, the material widgets take a text
// color as.
func colorOp(gtx layout.Context, c color.NRGBA) op.CallOp {
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	return m.Stop()
}
