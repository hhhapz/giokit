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

// Text draws a single line of text in the UI typeface.
func (t *Theme) Text(gtx C, s string, size unit.Sp, c color.NRGBA, w font.Weight) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Weight = w
	l.MaxLines = 1
	return hug(gtx, l)
}

// Paragraph draws wrapping body text.
func (t *Theme) Paragraph(gtx C, s string, c color.NRGBA) D {
	l := material.Label(t.mat, t.Type.Body, s)
	l.Color = c
	l.LineHeightScale = 1.35
	return hug(gtx, l)
}

// ParagraphLine is wrapping body text with a selectable font weight.
func (t *Theme) ParagraphLine(gtx C, s string, c color.NRGBA, w font.Weight) D {
	l := material.Label(t.mat, t.Type.Body, s)
	l.Color = c
	l.Font.Weight = w
	l.LineHeightScale = 1.35
	return hug(gtx, l)
}

// Code draws a single line in the monospace typeface (logs, ids, paths).
func (t *Theme) Code(gtx C, s string, size unit.Sp, c color.NRGBA) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	l.MaxLines = 1
	return hug(gtx, l)
}

// CodeBlock is Code that wraps: monospace paragraphs (previews, output).
func (t *Theme) CodeBlock(gtx C, s string, size unit.Sp, c color.NRGBA) D {
	l := material.Label(t.mat, size, s)
	l.Color = c
	l.Font.Typeface = t.Mono
	return hug(gtx, l)
}

// Heading is a page or section title.
func (t *Theme) Heading(gtx C, s string) D {
	return t.Text(gtx, s, t.Type.Title, t.Fg, t.HeadingWeight)
}

// Caption is the small uppercase label used for section and column headings.
func (t *Theme) Caption(gtx C, s string) D {
	return t.Text(gtx, strings.ToUpper(s), t.Type.Caption, t.Dim, Bold)
}

// Pill is a rounded status badge.
func (t *Theme) Pill(gtx C, s string, fg, bg color.NRGBA) D {
	return t.Styled(Surface{Bg: bg, Radius: t.RadiusFull()}).Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: 2, Bottom: 3, Left: 9, Right: 9}.Layout(gtx, func(gtx C) D {
			return t.Text(gtx, s, t.Type.Small, fg, Medium)
		})
	})
}

// Card is a padded panel with a hairline outline.
func (t *Theme) Card(gtx C, w W) D {
	return t.Styled(Surface{Bg: t.Panel, Border: t.Divider, Radius: t.RadiusLg()}).Layout(gtx, Pad(t.Space(Space3p5), t.Space(Space4), w))
}

// Bar is a full-width panel strip (title bar, status bar, toolbars).
func (t *Theme) Bar(gtx C, v, h unit.Dp, w W) D {
	return Fill(gtx, t.Panel, Stretch(Pad(t.Space(v), t.Space(h), w)))
}

// hug lays l out so its area, and so its semantic bounds, hug the text: it
// measures with gtx's constraints, then lays out again with that size as the
// maximum. Stock Gio clips a label to its maximum constraints, so without
// this a label reports bounds as large as its container (wrong for screen
// readers, and kittest could not find its pixels). The shaper caches the
// layout, so the second pass is cheap.
func hug(gtx C, l material.LabelStyle) D {
	m := op.Record(gtx.Ops)
	d := l.Layout(gtx)
	m.Stop()
	gtx.Constraints.Max = d.Size
	gtx.Constraints.Min = image.Pt(min(gtx.Constraints.Min.X, d.Size.X), min(gtx.Constraints.Min.Y, d.Size.Y))
	return l.Layout(gtx)
}
