package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/unit"
)

// TokenError reports a design token that ThemeFromTokens could not read or
// Theme.Tokens could not write.
type TokenError struct {
	Path string // token path, such as "color.bg", "radius" or "$name"
	Err  error  // what is wrong with the value
}

// Error formats the error as "kit: tokens: <path>: <cause>".
func (e *TokenError) Error() string { return "kit: tokens: " + e.Path + ": " + e.Err.Error() }

// Unwrap returns the cause, for errors.Is and errors.As.
func (e *TokenError) Unwrap() error { return e.Err }

const (
	// maxAliasDepth bounds an alias chain, which also stops alias cycles.
	maxAliasDepth = 16
	// pxPerRem converts rem dimensions: design tools use a 16px root size.
	pxPerRem = 16
	// cssWeightNormal is the DTCG (CSS) weight of normal text. font.Weight
	// counts from normal, so DTCG 700 is font.Weight 300 (Bold).
	cssWeightNormal = 400
)

// colorToken names one Palette field for DTCG files (group "color").
type colorToken struct {
	name  string
	value *color.NRGBA
}

// tokens names each palette color for DTCG files.
func (p *Palette) tokens() []colorToken {
	return []colorToken{
		{name: "bg", value: &p.Bg},
		{name: "panel", value: &p.Panel},
		{name: "surface", value: &p.Surface},
		{name: "hover", value: &p.Hover},
		{name: "border", value: &p.Border},
		{name: "divider", value: &p.Divider},
		{name: "fg", value: &p.Fg},
		{name: "muted", value: &p.Muted},
		{name: "dim", value: &p.Dim},
		{name: "accent", value: &p.Accent},
		{name: "accent-bg", value: &p.AccentBg},
		{name: "accent-border", value: &p.AccentBorder},
		{name: "on-accent", value: &p.OnAccent},
		{name: "good", value: &p.Good},
		{name: "good-bg", value: &p.GoodBg},
		{name: "warn", value: &p.Warn},
		{name: "warn-bg", value: &p.WarnBg},
		{name: "bad", value: &p.Bad},
		{name: "bad-bg", value: &p.BadBg},
		{name: "focus", value: &p.Focus},
		{name: "overlay", value: &p.Overlay},
		{name: "scrim", value: &p.Scrim},
	}
}

// typeToken names one TypeScale step for DTCG files (group "font-size").
type typeToken struct {
	name  string
	value *unit.Sp
}

// tokens names each type step for DTCG files.
func (s *TypeScale) tokens() []typeToken {
	return []typeToken{
		{name: "caption", value: &s.Caption},
		{name: "small", value: &s.Small},
		{name: "body", value: &s.Body},
		{name: "text", value: &s.Text},
		{name: "stat", value: &s.Stat},
		{name: "title", value: &s.Title},
	}
}

// ThemeFromTokens builds a theme from a W3C Design Tokens (DTCG) JSON file,
// the exchange format of Figma Variables (via Tokens Studio), Penpot and
// Style Dictionary. It starts from base (nil means Dark()), so a file only
// needs the tokens it changes; base itself is not modified.
//
// It reads these token paths:
//
//	color.<token>      bg, panel, surface, hover, border, divider, fg, muted, dim,
//	                   accent, accent-bg, accent-border, on-accent, good, good-bg,
//	                   warn, warn-bg, bad, bad-bg, focus, overlay, scrim
//	font-size.<step>   caption, small, body, text, stat, title
//	size.<metric>      icon, icon-lg, check, icon-button, switch-width,
//	                   switch-height, hit, pane-min, menu-min-width,
//	                   list-max-height, popover-max-width, dialog-max-width,
//	                   toast-width, reading-width, label-width, narrow-width
//	radius, stroke, ring          non-negative dimensions
//	elevation, density, motion    non-negative numbers
//	disabled-opacity              number from 0 to 1
//	heading-weight                DTCG font weight, 1 to 1000
//	controls                      "small", "medium" or "large"
//	dark                          boolean
//	name, fonts                   strings
//
// plus the file's top-level "$name", used when no name token changes the
// name. Every other path is ignored, even when it holds a broken alias, so a
// full design-system file with reference ramps works as-is. The font set is
// recorded by name only: apply it with th.UseFonts(fonts.ByName(th.Fonts)).
//
// Values may be aliases ("{color.blue.500}") to any token in the file.
// Colors are hex strings ("#rgb", "#rrggbb", "#rrggbbaa") or DTCG color
// objects in the srgb color space: {"colorSpace":"srgb","components":[r,g,b],
// "alpha":a} with r, g, b and a from 0 to 1, or {"hex":"#rrggbb","alpha":a}.
// Dimensions are numbers, strings ("6px", "1.5rem") or DTCG objects
// ({"value":6,"unit":"px"}); px, dp and sp are all one giokit unit, and a
// rem is 16 of them.
//
// A malformed value at a path it reads, including an alias that does not
// resolve, is an error. Every such problem is reported as a *TokenError
// naming its path, joined with errors.Join, and no theme is returned.
func ThemeFromTokens(data []byte, base *Theme) (*Theme, error) {
	if base == nil {
		base = Dark()
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("kit: tokens: %w", err)
	}
	tree := flattenTokens(root)
	t := base.Derive(func(*Theme) {})
	errs := t.decodeTokens(tree)
	if raw, ok := root["$name"]; ok {
		name, isString := raw.(string)
		switch {
		case !isString:
			errs = append(errs, &TokenError{Path: "$name", Err: fmt.Errorf("want a string, got %T", raw)})
		case t.Name == base.Name:
			t.Name = name
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return t, nil
}

// decodeTokens sets every theme field whose token is in tree. It returns
// one error per malformed token, leaving that field unchanged.
func (t *Theme) decodeTokens(tree tokenTree) []error {
	var errs []error
	keep := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	for _, tk := range t.Palette.tokens() {
		keep(decodeToken(tree, "color."+tk.name, parseColor, tk.value))
	}
	for _, tk := range t.Type.tokens() {
		keep(decodeToken(tree, "font-size."+tk.name, parseSp, tk.value))
	}
	for _, tk := range t.Metrics.tokens() {
		keep(decodeToken(tree, "size."+tk.name, parseDp, tk.value))
	}
	keep(decodeToken(tree, "radius", parseDp, &t.Radius))
	keep(decodeToken(tree, "stroke", parseDp, &t.Stroke))
	keep(decodeToken(tree, "ring", parseDp, &t.Ring))
	keep(decodeToken(tree, "elevation", parseScale, &t.Elevation))
	keep(decodeToken(tree, "density", parseScale, &t.Density))
	keep(decodeToken(tree, "motion", parseScale, &t.Motion))
	keep(decodeToken(tree, "disabled-opacity", parseOpacity, &t.DisabledAlpha))
	keep(decodeToken(tree, "heading-weight", parseFontWeight, &t.HeadingWeight))
	keep(decodeToken(tree, "controls", parseControlSize, &t.Controls))
	keep(decodeToken(tree, "dark", parseBool, &t.Dark))
	keep(decodeToken(tree, "fonts", parseString, &t.Fonts))
	keep(decodeToken(tree, "name", parseString, &t.Name))
	return errs
}

// decodeToken sets *dst from the token at path, if tree has one.
func decodeToken[T any](tree tokenTree, path string, parse func(any) (T, error), dst *T) error {
	raw, ok := tree[path]
	if !ok {
		return nil
	}
	v, err := tree.resolve(raw)
	if err != nil {
		return &TokenError{Path: path, Err: err}
	}
	x, err := parse(v)
	if err != nil {
		return &TokenError{Path: path, Err: err}
	}
	*dst = x
	return nil
}

// tokenTree maps each token path in a DTCG file ("color.bg") to its raw,
// unresolved $value.
type tokenTree map[string]any

// flattenTokens collects every token in a DTCG file. Groups nest with ".";
// keys starting with "$" are metadata, not groups.
func flattenTokens(root map[string]any) tokenTree {
	tree := tokenTree{}
	tree.add("", root)
	return tree
}

// add records the tokens of group, whose own path is path.
func (tree tokenTree) add(path string, group map[string]any) {
	if v, ok := group["$value"]; ok {
		tree[path] = v
		return
	}
	for k, v := range group {
		if strings.HasPrefix(k, "$") {
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			tree.add(strings.TrimPrefix(path+"."+k, "."), sub)
		}
	}
}

// resolve follows v's alias chain to a value that is not an alias.
func (tree tokenTree) resolve(v any) (any, error) {
	for depth := 0; ; depth++ {
		s, ok := v.(string)
		if !ok || !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
			return v, nil
		}
		if depth == maxAliasDepth {
			return nil, fmt.Errorf("alias cycle (or chain longer than %d) at %s", maxAliasDepth, s)
		}
		next, ok := tree[s[1:len(s)-1]]
		if !ok {
			return nil, fmt.Errorf("unknown alias %s", s)
		}
		v = next
	}
}

// parseColor reads a hex string or a DTCG color object.
func parseColor(v any) (color.NRGBA, error) {
	switch v := v.(type) {
	case string:
		return parseHex(v)
	case map[string]any:
		return parseColorObject(v)
	}
	return color.NRGBA{}, fmt.Errorf("want a color (hex string or color object), got %T", v)
}

func parseColorObject(v map[string]any) (color.NRGBA, error) {
	if cs, ok := v["colorSpace"]; ok && cs != "srgb" {
		return color.NRGBA{}, fmt.Errorf("color space %v unsupported (use srgb)", cs)
	}
	var c color.NRGBA
	if h, ok := v["hex"]; ok {
		s, isString := h.(string)
		if !isString {
			return color.NRGBA{}, fmt.Errorf("hex must be a string, got %T", h)
		}
		var err error
		if c, err = parseHex(s); err != nil {
			return color.NRGBA{}, err
		}
	} else {
		comps, ok := v["components"].([]any)
		if !ok || len(comps) != 3 {
			return color.NRGBA{}, errors.New("color object needs hex or 3 components")
		}
		var ch [3]uint8
		for i, x := range comps {
			f, ok := unitInterval(x)
			if !ok {
				return color.NRGBA{}, fmt.Errorf("component %d must be a number from 0 to 1, got %v", i, x)
			}
			ch[i] = channel(f)
		}
		c = color.NRGBA{R: ch[0], G: ch[1], B: ch[2], A: 255}
	}
	if a, ok := v["alpha"]; ok {
		f, ok := unitInterval(a)
		if !ok {
			return color.NRGBA{}, fmt.Errorf("alpha must be a number from 0 to 1, got %v", a)
		}
		c.A = channel(f)
	}
	return c, nil
}

// unitInterval returns v as a number if it is a JSON number from 0 to 1.
func unitInterval(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok && f >= 0 && f <= 1
}

// channel converts a 0..1 component to 8 bits.
func channel(f float64) uint8 { return uint8(f*255 + 0.5) }

// dimensionUnits are the units a dimension may carry, in giokit units (dp,
// or sp for font sizes).
var dimensionUnits = map[string]float64{"px": 1, "dp": 1, "sp": 1, "rem": pxPerRem}

// parseDimension reads a non-negative dimension: a number, a string such
// as "6px" or "1.5rem", or a DTCG {"value", "unit"} object.
func parseDimension(v any) (float64, error) {
	var f float64
	switch v := v.(type) {
	case float64:
		f = v
	case string:
		var err error
		if f, err = parseDimensionString(v); err != nil {
			return 0, err
		}
	case map[string]any:
		n, ok := v["value"].(float64)
		if !ok {
			return 0, fmt.Errorf("dimension value must be a number, got %v", v["value"])
		}
		u, _ := v["unit"].(string)
		scale, ok := dimensionUnits[u]
		if !ok {
			return 0, fmt.Errorf("dimension unit %v unsupported (use px, dp, sp or rem)", v["unit"])
		}
		f = n * scale
	default:
		return 0, fmt.Errorf(`want a dimension (number, "6px" or {"value", "unit"}), got %T`, v)
	}
	if err := checkDimension(f); err != nil {
		return 0, err
	}
	return f, nil
}

// parseDimensionString reads a number with an optional unit suffix.
func parseDimensionString(s string) (float64, error) {
	s = strings.TrimSpace(s)
	num, scale := s, 1.0
	for u, k := range dimensionUnits {
		if n, ok := strings.CutSuffix(s, u); ok {
			num, scale = n, k
			break
		}
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil {
		return 0, fmt.Errorf("dimension %q is not a number with unit px, dp, sp or rem", s)
	}
	return f * scale, nil
}

// checkDimension rejects sizes no theme can use.
func checkDimension(f float64) error {
	if err := checkFinite(f); err != nil {
		return err
	}
	if f < 0 {
		return fmt.Errorf("dimension must not be negative, got %v", f)
	}
	return nil
}

// checkFinite rejects NaN, infinities and values beyond float32's range,
// which theme fields store.
func checkFinite(f float64) error {
	if math.IsNaN(f) || math.Abs(f) > math.MaxFloat32 {
		return fmt.Errorf("must be a finite number, got %v", f)
	}
	return nil
}

func parseDp(v any) (unit.Dp, error) {
	f, err := parseDimension(v)
	return unit.Dp(f), err
}

func parseSp(v any) (unit.Sp, error) {
	f, err := parseDimension(v)
	return unit.Sp(f), err
}

// parseNumber reads a finite unitless number: a JSON number or a numeric
// string.
func parseNumber(v any) (float64, error) {
	var f float64
	switch v := v.(type) {
	case float64:
		f = v
	case string:
		var err error
		if f, err = strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return 0, fmt.Errorf("want a number, got %q", v)
		}
	default:
		return 0, fmt.Errorf("want a number, got %T", v)
	}
	return f, checkFinite(f)
}

// parseScale reads a non-negative multiplier (elevation, density, motion).
func parseScale(v any) (float32, error) {
	f, err := parseNumber(v)
	if err == nil && f < 0 {
		err = fmt.Errorf("must not be negative, got %v", f)
	}
	return float32(f), err
}

// parseOpacity reads an opacity from 0 to 1.
func parseOpacity(v any) (float32, error) {
	f, err := parseNumber(v)
	if err == nil && (f < 0 || f > 1) {
		err = fmt.Errorf("opacity must be 0 to 1, got %v", f)
	}
	return float32(f), err
}

// parseFontWeight reads a DTCG numeric font weight (400 is normal).
func parseFontWeight(v any) (font.Weight, error) {
	f, err := parseNumber(v)
	if err != nil {
		return 0, err
	}
	w := math.Round(f)
	if w < 1 || w > 1000 {
		return 0, fmt.Errorf("font weight must be 1 to 1000, got %v", f)
	}
	return font.Weight(w - cssWeightNormal), nil
}

func parseControlSize(v any) (ControlSize, error) {
	s, err := parseString(v)
	if err != nil {
		return 0, err
	}
	switch s {
	case "small":
		return ControlSmall, nil
	case "medium":
		return ControlDefault, nil // a theme's default size is medium
	case "large":
		return ControlLarge, nil
	}
	return 0, fmt.Errorf("unknown control size %q (use small, medium or large)", s)
}

// tokenName is the DTCG value of a theme's Controls.
func (s ControlSize) tokenName() (string, error) {
	switch s {
	case ControlSmall:
		return "small", nil
	case ControlDefault, ControlMedium:
		return "medium", nil
	case ControlLarge:
		return "large", nil
	}
	return "", fmt.Errorf("unknown control size %d", s)
}

func parseBool(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("want a boolean, got %T", v)
	}
	return b, nil
}

func parseString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("want a string, got %T", v)
	}
	return s, nil
}

// tokenLeaf is one token in a DTCG file.
type tokenLeaf struct {
	Type  string `json:"$type"`
	Value any    `json:"$value"`
}

// dimensionValue is a DTCG dimension. Fields are declared in key order, so
// they encode like the sorted maps around them.
type dimensionValue struct {
	Unit  string  `json:"unit"`
	Value float32 `json:"value"`
}

// colorValue is a DTCG color, with a hex fallback for tools that predate
// color objects. Alpha is omitted when the color is opaque.
type colorValue struct {
	Alpha      *float64   `json:"alpha,omitempty"`
	ColorSpace string     `json:"colorSpace"`
	Components [3]float64 `json:"components"`
	Hex        string     `json:"hex"`
}

// tokenEncoder builds Tokens output, collecting the values ThemeFromTokens
// would reject.
type tokenEncoder struct {
	errs []error
}

func (e *tokenEncoder) check(path string, err error) {
	if err != nil {
		e.errs = append(e.errs, &TokenError{Path: path, Err: err})
	}
}

func (e *tokenEncoder) dimension(path string, v float32) tokenLeaf {
	e.check(path, checkDimension(float64(v)))
	return tokenLeaf{Type: "dimension", Value: dimensionValue{Unit: "px", Value: v}}
}

func (e *tokenEncoder) number(path string, v float32, parse func(any) (float32, error)) tokenLeaf {
	_, err := parse(float64(v))
	e.check(path, err)
	return tokenLeaf{Type: "number", Value: v}
}

func colorLeaf(c color.NRGBA) tokenLeaf {
	v := colorValue{
		ColorSpace: "srgb",
		Components: [3]float64{round3(float64(c.R) / 255), round3(float64(c.G) / 255), round3(float64(c.B) / 255)},
		Hex:        Hex(WithAlpha(c, 255)),
	}
	if c.A != 255 {
		a := round3(float64(c.A) / 255)
		v.Alpha = &a
	}
	return tokenLeaf{Type: "color", Value: v}
}

func stringLeaf(s string) tokenLeaf { return tokenLeaf{Type: "string", Value: s} }

// Tokens exports the theme as DTCG JSON, which ThemeFromTokens reads back
// to an equal theme, with one exception: the controls token writes both
// ControlDefault and ControlMedium as "medium", which reads back as
// ControlDefault, so ControlMedium comes back as ControlDefault (the same
// size). Keys are sorted, so files
// are stable and diff well.
//
// It fails when a field holds a value ThemeFromTokens would reject: a
// non-finite or negative size or multiplier, a DisabledAlpha outside 0..1,
// an unknown ControlSize or a HeadingWeight outside DTCG's 1..1000. Each
// such field is reported as a *TokenError, joined with errors.Join.
func (t *Theme) Tokens() ([]byte, error) {
	var e tokenEncoder
	colors := map[string]tokenLeaf{}
	for _, tk := range t.Palette.tokens() {
		colors[tk.name] = colorLeaf(*tk.value)
	}
	sizes := map[string]tokenLeaf{}
	for _, tk := range t.Type.tokens() {
		sizes[tk.name] = e.dimension("font-size."+tk.name, float32(*tk.value))
	}
	metrics := map[string]tokenLeaf{}
	for _, tk := range t.Metrics.tokens() {
		metrics[tk.name] = e.dimension("size."+tk.name, float32(*tk.value))
	}
	controls, err := t.Controls.tokenName()
	e.check("controls", err)
	weight := int(t.HeadingWeight) + cssWeightNormal
	_, err = parseFontWeight(float64(weight))
	e.check("heading-weight", err)
	out := map[string]any{
		"$name":            t.Name,
		"name":             stringLeaf(t.Name),
		"dark":             tokenLeaf{Type: "boolean", Value: t.Dark},
		"color":            colors,
		"font-size":        sizes,
		"size":             metrics,
		"radius":           e.dimension("radius", float32(t.Radius)),
		"stroke":           e.dimension("stroke", float32(t.Stroke)),
		"ring":             e.dimension("ring", float32(t.Ring)),
		"elevation":        e.number("elevation", t.Elevation, parseScale),
		"density":          e.number("density", t.Density, parseScale),
		"motion":           e.number("motion", t.Motion, parseScale),
		"disabled-opacity": e.number("disabled-opacity", t.DisabledAlpha, parseOpacity),
		"heading-weight":   tokenLeaf{Type: "fontWeight", Value: weight},
		"controls":         stringLeaf(controls),
		"fonts":            stringLeaf(t.Fonts),
	}
	if err := errors.Join(e.errs...); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("kit: tokens: %w", err)
	}
	return b, nil
}

// round3 rounds a color component to three decimals, enough to round-trip
// 8-bit channels.
func round3(f float64) float64 { return math.Round(f*1000) / 1000 }
