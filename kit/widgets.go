package kit

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Text draws a single line of text in the UI typeface, truncated to the
// width available. Its area hugs the text.
func (t *Theme) Text(gtx layout.Context, s string, size unit.Sp, c color.NRGBA, w font.Weight) layout.Dimensions {
	l := material.Label(t.Material(), size, s)
	l.Color = c
	l.Font.Weight = w
	l.MaxLines = 1
	return hug(gtx, l)
}

// Paragraph draws wrapping body text at the regular weight.
func (t *Theme) Paragraph(gtx layout.Context, s string, c color.NRGBA) layout.Dimensions {
	l := material.Label(t.Material(), t.Type.Body, s)
	l.Color = c
	l.LineHeightScale = bodyLineHeight
	return hug(gtx, l)
}

// ParagraphLine is Paragraph with a font weight: it wraps too. It suits a
// block of text whose emphasis changes with state, such as the current line
// of lyrics.
func (t *Theme) ParagraphLine(gtx layout.Context, s string, c color.NRGBA, w font.Weight) layout.Dimensions {
	l := material.Label(t.Material(), t.Type.Body, s)
	l.Color = c
	l.Font.Weight = w
	l.LineHeightScale = bodyLineHeight
	return hug(gtx, l)
}

// Code draws a single line in the monospace typeface (logs, ids, paths).
func (t *Theme) Code(gtx layout.Context, s string, size unit.Sp, c color.NRGBA) layout.Dimensions {
	l := material.Label(t.Material(), size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	l.MaxLines = 1
	return hug(gtx, l)
}

// CodeBlock is Code that wraps: monospace paragraphs (previews, output).
func (t *Theme) CodeBlock(gtx layout.Context, s string, size unit.Sp, c color.NRGBA) layout.Dimensions {
	l := material.Label(t.Material(), size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	return hug(gtx, l)
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

// hug lays l out so its area, and so its semantic bounds, hug the text: it
// measures with gtx's constraints, then lays out again with that size as the
// maximum. Stock Gio clips a label to its maximum constraints, so without
// this a label reports bounds as large as its container (wrong for screen
// readers, and kittest could not find its pixels). The shaper caches the
// layout, so the second pass is cheap.
func hug(gtx layout.Context, l material.LabelStyle) layout.Dimensions {
	m := op.Record(gtx.Ops)
	d := l.Layout(gtx)
	m.Stop()
	gtx.Constraints.Max = d.Size
	gtx.Constraints.Min = image.Pt(min(gtx.Constraints.Min.X, d.Size.X), min(gtx.Constraints.Min.Y, d.Size.Y))
	return l.Layout(gtx)
}
