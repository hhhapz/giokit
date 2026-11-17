package kit

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/unit"
)

// Design tokens in the W3C Design Tokens (DTCG) JSON format: the exchange
// format of Figma Variables (via Tokens Studio), Penpot and Style Dictionary.
//
// giokit reads these paths (anything else in the file is ignored, so a full
// design-system file with reference ramps works as-is):
//
//	color.<token>         bg, panel, surface, hover, border, divider, fg, muted, dim,
//	                      accent, accent-bg, accent-border, on-accent, good, good-bg,
//	                      warn, warn-bg, bad, bad-bg, focus, overlay, scrim
//	font-size.<step>      caption, small, body, text, stat, title
//	radius, density, motion, dark, name
//
// Values may be aliases ("{color.blue.500}"). Colors may be hex strings or
// DTCG 2025 objects ({"colorSpace":"srgb","components":[r,g,b],"alpha":1}).
// Dimensions may be "6px" or {"value":6,"unit":"px"}.

func (p *Palette) tokens() []struct {
	name string
	c    *color.NRGBA
} {
	return []struct {
		name string
		c    *color.NRGBA
	}{
		{"bg", &p.Bg}, {"panel", &p.Panel}, {"surface", &p.Surface}, {"hover", &p.Hover},
		{"border", &p.Border}, {"divider", &p.Divider}, {"fg", &p.Fg}, {"muted", &p.Muted}, {"dim", &p.Dim},
		{"accent", &p.Accent}, {"accent-bg", &p.AccentBg}, {"accent-border", &p.AccentBorder}, {"on-accent", &p.OnAccent},
		{"good", &p.Good}, {"good-bg", &p.GoodBg}, {"warn", &p.Warn}, {"warn-bg", &p.WarnBg},
		{"bad", &p.Bad}, {"bad-bg", &p.BadBg}, {"focus", &p.Focus}, {"overlay", &p.Overlay}, {"scrim", &p.Scrim},
	}
}

func (s *TypeScale) tokens() []struct {
	name string
	v    *unit.Sp
} {
	return []struct {
		name string
		v    *unit.Sp
	}{{"caption", &s.Caption}, {"small", &s.Small}, {"body", &s.Body}, {"text", &s.Text}, {"stat", &s.Stat}, {"title", &s.Title}}
}

// ThemeFromTokens builds a theme from DTCG JSON, starting from base (nil =
// Dark()), so a file only needs the tokens it changes.
func ThemeFromTokens(data []byte, base *Theme) (*Theme, error) {
	if base == nil {
		base = Dark()
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("kit: tokens: %w", err)
	}
	flat := map[string]any{} // "color.bg" → $value
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		if v, ok := m["$value"]; ok {
			flat[prefix] = v
			return
		}
		for k, v := range m {
			if strings.HasPrefix(k, "$") {
				continue
			}
			if sub, ok := v.(map[string]any); ok {
				walk(strings.TrimPrefix(prefix+"."+k, "."), sub)
			}
		}
	}
	walk("", root)

	var resolve func(v any, depth int) (any, error)
	resolve = func(v any, depth int) (any, error) {
		s, ok := v.(string)
		if !ok || !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
			return v, nil
		}
		if depth > 16 {
			return nil, fmt.Errorf("alias cycle at %s", s)
		}
		ref, ok := flat[s[1:len(s)-1]]
		if !ok {
			return nil, fmt.Errorf("unknown alias %s", s)
		}
		return resolve(ref, depth+1)
	}
	get := func(path string) (any, bool, error) {
		v, ok := flat[path]
		if !ok {
			return nil, false, nil
		}
		r, err := resolve(v, 0)
		if err != nil {
			return nil, true, fmt.Errorf("kit: tokens: %s: %w", path, err)
		}
		return r, true, nil
	}

	t := base.Derive(func(*Theme) {})
	for _, tk := range t.Palette.tokens() {
		v, ok, err := get("color." + tk.name)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		c, err := parseColorToken(v)
		if err != nil {
			return nil, fmt.Errorf("kit: tokens: color.%s: %w", tk.name, err)
		}
		*tk.c = c
	}
	for _, tk := range t.Type.tokens() {
		v, ok, err := get("font-size." + tk.name)
		if err != nil {
			return nil, err
		}
		if ok {
			f, err := parseDimension(v)
			if err != nil {
				return nil, fmt.Errorf("kit: tokens: font-size.%s: %w", tk.name, err)
			}
			*tk.v = unit.Sp(f)
		}
	}
	num := func(path string, set func(float64)) error {
		v, ok, err := get(path)
		if err != nil || !ok {
			return err
		}
		f, err := parseDimension(v)
		if err != nil {
			return fmt.Errorf("kit: tokens: %s: %w", path, err)
		}
		set(f)
		return nil
	}
	for _, e := range []error{
		num("radius", func(f float64) { t.Radius = unit.Dp(f) }),
		num("stroke", func(f float64) { t.Stroke = unit.Dp(f) }),
		num("ring", func(f float64) { t.Ring = unit.Dp(f) }),
		num("elevation", func(f float64) { t.Elevation = float32(f) }),
		num("heading-weight", func(f float64) { t.HeadingWeight = font.Weight(f - 400) }),
		num("density", func(f float64) { t.Density = float32(f) }),
		num("motion", func(f float64) { t.Motion = float32(f) }),
		num("disabled-opacity", func(f float64) { t.DisabledAlpha = float32(f) }),
	} {
		if e != nil {
			return nil, e
		}
	}
	if v, ok, _ := get("dark"); ok {
		if b, ok := v.(bool); ok {
			t.Dark = b
		}
	}
	if v, ok, _ := get("controls"); ok {
		if s, ok := v.(string); ok {
			switch s {
			case "small":
				t.Controls = ControlSmall
			case "large":
				t.Controls = ControlLarge
			default: // medium is the theme default
				t.Controls = ControlDefault
			}
		}
	}
	// The font set is recorded by name; kit cannot load it without importing
	// kit/fonts. Apply it with th.UseFonts(fonts.ByName(th.Fonts)).
	if v, ok, _ := get("fonts"); ok {
		if s, ok := v.(string); ok {
			t.Fonts = s
		}
	}
	if v, ok, _ := get("name"); ok {
		if s, ok := v.(string); ok {
			t.Name = s
		}
	}
	if n, ok := root["$name"].(string); ok && t.Name == base.Name {
		t.Name = n
	}
	t.syncMaterial()
	return t, nil
}

func parseColorToken(v any) (color.NRGBA, error) {
	switch v := v.(type) {
	case string:
		return ParseHex(v)
	case map[string]any:
		if h, ok := v["hex"].(string); ok {
			c, err := ParseHex(h)
			if a, ok := v["alpha"].(float64); ok {
				c.A = uint8(a*255 + 0.5)
			}
			return c, err
		}
		comps, ok := v["components"].([]any)
		if !ok || len(comps) != 3 {
			return color.NRGBA{}, fmt.Errorf("color object needs hex or 3 components")
		}
		if cs, _ := v["colorSpace"].(string); cs != "" && cs != "srgb" {
			return color.NRGBA{}, fmt.Errorf("color space %q unsupported (use srgb)", cs)
		}
		var ch [3]uint8
		for i, x := range comps {
			f, ok := x.(float64)
			if !ok {
				return color.NRGBA{}, fmt.Errorf("bad component %v", x)
			}
			ch[i] = uint8(max(0, min(1, f))*255 + 0.5)
		}
		c := color.NRGBA{R: ch[0], G: ch[1], B: ch[2], A: 255}
		if a, ok := v["alpha"].(float64); ok {
			c.A = uint8(a*255 + 0.5)
		}
		return c, nil
	}
	return color.NRGBA{}, fmt.Errorf("unsupported color value %v", v)
}

func parseDimension(v any) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		for _, u := range []string{"px", "dp", "sp", "pt"} {
			s = strings.TrimSuffix(s, u)
		}
		if strings.HasSuffix(s, "rem") { // 1rem = 16px
			f, err := strconv.ParseFloat(strings.TrimSuffix(s, "rem"), 64)
			return f * 16, err
		}
		return strconv.ParseFloat(s, 64)
	case map[string]any:
		f, ok := v["value"].(float64)
		if !ok {
			return 0, fmt.Errorf("dimension object needs a numeric value")
		}
		if u, _ := v["unit"].(string); u == "rem" {
			f *= 16
		}
		return f, nil
	}
	return 0, fmt.Errorf("unsupported dimension %v", v)
}

// Tokens exports the theme as DTCG JSON (round-trips through ThemeFromTokens).
func (t *Theme) Tokens() []byte {
	tok := func(typ string, v any) map[string]any { return map[string]any{"$type": typ, "$value": v} }
	colorVal := func(c color.NRGBA) map[string]any {
		m := map[string]any{"colorSpace": "srgb", "hex": Hex(color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}),
			"components": []float64{round3(float64(c.R) / 255), round3(float64(c.G) / 255), round3(float64(c.B) / 255)}}
		if c.A != 255 {
			m["alpha"] = round3(float64(c.A) / 255)
		}
		return m
	}
	dim := func(v float32) map[string]any { return map[string]any{"value": v, "unit": "px"} }
	colors, sizes := map[string]any{}, map[string]any{}
	for _, tk := range t.Palette.tokens() {
		colors[tk.name] = tok("color", colorVal(*tk.c))
	}
	ts := t.Type
	for _, tk := range ts.tokens() {
		sizes[tk.name] = tok("dimension", dim(float32(*tk.v)))
	}
	out := map[string]any{
		"$name": t.Name, "name": tok("string", t.Name), "dark": tok("boolean", t.Dark),
		"color": colors, "font-size": sizes,
		"radius":         tok("dimension", dim(float32(t.Radius))),
		"stroke":         tok("dimension", dim(float32(t.Stroke))),
		"ring":           tok("dimension", dim(float32(t.Ring))),
		"elevation":      tok("number", round3(float64(t.Elevation))),
		"heading-weight": tok("fontWeight", int(t.HeadingWeight)+400), // DTCG: 100..900
		"controls":       tok("string", [...]string{"medium", "small", "medium", "large"}[t.Controls]),
		"fonts":          tok("string", t.Fonts),
		"density":        tok("number", t.Density), "motion": tok("number", t.Motion),
		"disabled-opacity": tok("number", round3(float64(t.DisabledAlpha))),
	}
	b, _ := json.MarshalIndent(out, "", "  ") // map keys are sorted: stable, diff-friendly files
	return b
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }
