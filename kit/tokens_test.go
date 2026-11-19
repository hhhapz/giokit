package kit

import (
	"encoding/json"
	"errors"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/hhhapz/giokit/gio/unit"
)

// Every malformed value giokit reads is an error naming the token's path,
// not a silently ignored or half-applied value.
func TestTokensRejectMalformed(t *testing.T) {
	for _, c := range []struct {
		src, path, cause string
	}{
		{`{"dark":{"$value":"{missing}"}}`, "dark", "unknown alias"},
		{`{"controls":{"$value":"{missing}"}}`, "controls", "unknown alias"},
		{`{"fonts":{"$value":"{missing}"}}`, "fonts", "unknown alias"},
		{`{"name":{"$value":"{missing}"}}`, "name", "unknown alias"},
		{`{"radius":{"$value":{"value":6,"unit":"bananas"}}}`, "radius", "unit"},
		{`{"radius":{"$value":"6bananas"}}`, "radius", "unit"},
		{`{"radius":{"$value":"NaN"}}`, "radius", "finite"},
		{`{"radius":{"$value":"Inf"}}`, "radius", "finite"},
		{`{"radius":{"$value":-1}}`, "radius", "negative"},
		{`{"radius":{"$value":true}}`, "radius", "dimension"},
		{`{"font-size":{"body":{"$value":"NaNpx"}}}`, "font-size.body", "finite"},
		{`{"size":{"icon":{"$value":{"value":"6","unit":"px"}}}}`, "size.icon", "number"},
		{`{"density":{"$value":"1px"}}`, "density", "number"},
		{`{"motion":{"$value":"NaN"}}`, "motion", "finite"},
		{`{"disabled-opacity":{"$value":1.5}}`, "disabled-opacity", "0 to 1"},
		{`{"heading-weight":{"$value":1200}}`, "heading-weight", "1 to 1000"},
		{`{"dark":{"$value":"yes"}}`, "dark", "boolean"},
		{`{"controls":{"$value":"huge"}}`, "controls", "control size"},
		{`{"controls":{"$value":3}}`, "controls", "string"},
		{`{"fonts":{"$value":true}}`, "fonts", "string"},
		{`{"name":{"$value":5}}`, "name", "string"},
		{`{"$name":5}`, "$name", "string"},
		{`{"color":{"bg":{"$value":"#12"}}}`, "color.bg", "hex"},
		{`{"color":{"bg":{"$value":"#12345g"}}}`, "color.bg", "hex"},
		{`{"color":{"bg":{"$value":{"hex":"#zzzzzz","alpha":0.5}}}}`, "color.bg", "hex"},
		{`{"color":{"bg":{"$value":{"hex":"#112233","alpha":2}}}}`, "color.bg", "alpha"},
		{`{"color":{"bg":{"$value":{"hex":"#112233","alpha":"half"}}}}`, "color.bg", "alpha"},
		{`{"color":{"bg":{"$value":{"colorSpace":"display-p3","hex":"#112233"}}}}`, "color.bg", "color space"},
		{`{"color":{"bg":{"$value":{"components":[0.1,1.5,0.1]}}}}`, "color.bg", "component"},
		{`{"color":{"bg":{"$value":{"components":[0.1,"none",0.1]}}}}`, "color.bg", "component"},
		{`{"color":{"bg":{"$value":{"components":[0.1,0.1]}}}}`, "color.bg", "components"},
		{`{"color":{"bg":{"$value":42}}}`, "color.bg", "color"},
	} {
		_, err := ThemeFromTokens([]byte(c.src), nil)
		var te *TokenError
		if !errors.As(err, &te) {
			t.Errorf("%s: err = %v, want a *TokenError", c.src, err)
			continue
		}
		if te.Path != c.path || !strings.Contains(err.Error(), c.cause) {
			t.Errorf("%s: err = %v (path %q), want path %q and %q", c.src, err, te.Path, c.path, c.cause)
		}
	}
}

// Paths giokit does not read are ignored, so a design-system file whose
// other tokens use features giokit doesn't support still loads.
func TestTokensIgnoreUnreadPaths(t *testing.T) {
	src := `{"ref":{"broken":{"$value":"{missing}"},"odd":{"$value":{"unit":"bananas"}}},
	         "color":{"extra":{"$value":"not a color"}}}`
	if _, err := ThemeFromTokens([]byte(src), nil); err != nil {
		t.Fatal(err)
	}
}

func TestTokensDimensionForms(t *testing.T) {
	for src, want := range map[string]float32{
		`"6"`: 6, `"6px"`: 6, `"6dp"`: 6, `" 6sp "`: 6, `"1.5rem"`: 24, `6`: 6,
		`{"value":6,"unit":"px"}`: 6, `{"value":0.5,"unit":"rem"}`: 8,
	} {
		th, err := ThemeFromTokens([]byte(`{"radius":{"$value":`+src+`}}`), nil)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if float32(th.Radius) != want {
			t.Errorf("%s: radius %v, want %v", src, th.Radius, want)
		}
	}
}

// Tokens reports a theme it cannot represent instead of returning empty or
// partial output, and never panics on an out-of-range enum.
func TestTokensEncodingContract(t *testing.T) {
	for name, f := range map[string]func(*Theme){
		"radius NaN":     func(t *Theme) { t.Radius = unit.Dp(float32NaN()) },
		"density Inf":    func(t *Theme) { t.Density = float32(math.Inf(1)) },
		"icon NaN":       func(t *Theme) { t.IconSize = unit.Dp(float32NaN()) },
		"body NaN":       func(t *Theme) { t.Type.Body = unit.Sp(float32NaN()) },
		"controls 9":     func(t *Theme) { t.Controls = 9 },
		"controls -1":    func(t *Theme) { t.Controls = -1 },
		"weight 900":     func(t *Theme) { t.HeadingWeight = 900 },
		"opacity NaN":    func(t *Theme) { t.DisabledAlpha = float32NaN() },
		"elevation -Inf": func(t *Theme) { t.Elevation = float32(math.Inf(-1)) },
	} {
		b, err := Dark().Derive(f).Tokens()
		if err == nil || b != nil {
			t.Errorf("%s: Tokens() = %d bytes, %v; want nil, error", name, len(b), err)
		}
	}
	b, err := Dark().Tokens()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("Tokens output is not JSON: %v", err)
	}
}

func float32NaN() float32 { return float32(math.NaN()) }

// The controls token has one value for medium, so ControlMedium reads back
// as ControlDefault, which sizes controls the same.
func TestTokensControlMedium(t *testing.T) {
	th := Dark().Derive(func(t *Theme) { t.Controls = ControlMedium })
	b, err := th.Tokens()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ThemeFromTokens(b, Light().Derive(func(t *Theme) { t.Controls = ControlLarge }))
	if err != nil {
		t.Fatal(err)
	}
	if back.Controls != ControlDefault {
		t.Errorf("Controls = %d, want ControlDefault", back.Controls)
	}
	if got, want := back.ControlPad(ControlDefault, fieldPadY), th.ControlPad(ControlDefault, fieldPadY); got != want {
		t.Errorf("control padding %v, want %v", got, want)
	}
}

// Every alpha survives export, a fully transparent color included.
func TestTokensRoundTripAlpha(t *testing.T) {
	for _, a := range []uint8{0, 1, 0x8c, 254, 255} {
		th := Dark().Derive(func(t *Theme) { t.Scrim = WithAlpha(t.Scrim, a) })
		b, err := th.Tokens()
		if err != nil {
			t.Fatal(err)
		}
		back, err := ThemeFromTokens(b, Light())
		if err != nil {
			t.Fatal(err)
		}
		if back.Scrim != th.Scrim {
			t.Errorf("alpha %d: scrim %v, want %v", a, back.Scrim, th.Scrim)
		}
	}
}

func TestParseHex(t *testing.T) {
	for s, want := range map[string]color.NRGBA{
		"#abc":      {R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff},
		"ABC":       {R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff},
		"#3366ff":   {R: 0x33, G: 0x66, B: 0xff, A: 0xff},
		"#3366ff80": {R: 0x33, G: 0x66, B: 0xff, A: 0x80},
	} {
		if got, err := ParseHex(s); err != nil || got != want {
			t.Errorf("ParseHex(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	for _, s := range []string{"", "#", "#12", "#1234", "#12345", "#12345g", "#ggg", "#1234567", "##123456", " #123456"} {
		if got, err := ParseHex(s); err == nil || got != (color.NRGBA{}) || !strings.Contains(err.Error(), strconv.Quote(s)) {
			t.Errorf("ParseHex(%q) = %v, %v; want the zero color and an error quoting the input", s, got, err)
		}
	}
}
