// Contrast checks for the theme validator (issue #97, ADR 0002 §7): jsdom
// can't compute colours, so the core checks the WCAG 2 ratio of the token
// values a theme ships. A pair that misses its threshold costs the foreground
// token its value, exactly like any other invalid token.
package themes

import (
	"math"
	"path"
	"strconv"
	"strings"
	"sync"
)

// The thresholds, from WCAG 2.2 AA: 4.5:1 for text (1.4.3) and 3:1 for large
// text and non-text indicators such as the focus ring (1.4.11).
const (
	textRatio = 4.5
	uiRatio   = 3.0
)

// contrastPair is one foreground/background pair the validator checks and the
// ratio it must reach.
type contrastPair struct {
	fg, bg string
	min    float64
}

// contrastPairs is the check. Body text sits on both surfaces; muted and faint
// text are read at the UI threshold; the accent's own text sits on both
// accents; the focus ring sits on the page.
var contrastPairs = []contrastPair{
	{"text", "ground", textRatio},
	{"text", "raised", textRatio},
	{"text-body", "ground", textRatio},
	{"text-body", "raised", textRatio},
	{"text-muted", "ground", uiRatio},
	{"text-muted", "raised", uiRatio},
	{"text-faint", "ground", uiRatio},
	{"text-faint", "raised", uiRatio},
	{"on-accent", "accent", textRatio},
	{"on-accent", "accent-strong", textRatio},
	{"focus", "ground", uiRatio},
}

// builtinDefaults is the default theme's token set: the values a failing token
// falls back to. It is read without the contrast check, which would ask for it
// again.
var (
	defaultsOnce sync.Once
	defaults     map[string]string
)

func builtinDefaults() map[string]string {
	defaultsOnce.Do(func() {
		src, err := builtin.ReadFile(path.Join("builtin", DefaultID, "theme.toml"))
		if err != nil {
			return
		}
		defaults = parseTheme(src, false).Tokens
	})
	return defaults
}

// checkContrast drops every token that misses its pair's ratio, so Resolve
// falls back to the built-in value. It checks the theme's tokens over the
// built-in ones, so a theme that only moves the ground still has its text
// checked. The pair is what has to pass: an inherited foreground alone can't
// fix a pair the theme broke, so the override it did set falls back too, and
// the built-in pair is what reaches the UI. A background that falls back
// changes pairs already checked against it, so the pairs run again until
// nothing falls back (each pass only reverts overrides, so this ends). Each
// declared scheme is checked the same way over its own resolved tokens
// (issue #134): a failing scheme token falls back to the dark value.
func (t *Theme) checkContrast() {
	def := builtinDefaults()
	if def == nil {
		return
	}
	tokens := make(map[string]string, len(def)+len(t.Tokens))
	for name, v := range def {
		tokens[name] = v
	}
	for name, v := range t.Tokens {
		tokens[name] = v
	}
	for changed := true; changed; {
		changed = false
		for _, p := range contrastPairs {
			got, missing := missingContrast(tokens, p)
			if !missing {
				continue
			}
			before := tokens[p.fg] + "\x00" + tokens[p.bg]
			t.warn("token %s on %s: %s on %s is %.2f:1, needs %.1f:1, using the built-in value",
				p.fg, p.bg, tokens[p.fg], tokens[p.bg], got, p.min)
			// Drop the foreground, then the background when the pair still
			// misses: a token the theme didn't set has nothing to drop, and
			// returning before the background would accept a failing pair.
			delete(t.Tokens, p.fg)
			tokens[p.fg] = def[p.fg]
			if _, still := missingContrast(tokens, p); still {
				delete(t.Tokens, p.bg)
				tokens[p.bg] = def[p.bg]
			}
			if tokens[p.fg]+"\x00"+tokens[p.bg] != before {
				changed = true
			}
		}
	}
	for _, scheme := range schemeNames {
		over, ok := t.Schemes[scheme]
		if len(over) == 0 || !ok {
			continue
		}
		resolved := make(map[string]string, len(tokens))
		for name, v := range tokens {
			resolved[name] = v
		}
		for name, v := range over {
			resolved[name] = v
		}
		for changed := true; changed; {
			changed = false
			for _, p := range contrastPairs {
				got, missing := missingContrast(resolved, p)
				if !missing {
					continue
				}
				before := resolved[p.fg] + "\x00" + resolved[p.bg]
				t.warn("token %s in scheme %s on %s: %s on %s is %.2f:1, needs %.1f:1, using the built-in value",
					p.fg, scheme, p.bg, resolved[p.fg], resolved[p.bg], got, p.min)
				delete(t.Schemes[scheme], p.fg)
				resolved[p.fg] = tokens[p.fg]
				if _, still := missingContrast(resolved, p); still {
					delete(t.Schemes[scheme], p.bg)
					resolved[p.bg] = tokens[p.bg]
				}
				if resolved[p.fg]+"\x00"+resolved[p.bg] != before {
					changed = true
				}
			}
		}
		if len(t.Schemes[scheme]) == 0 {
			delete(t.Schemes, scheme)
		}
	}
}

// missingContrast reports the ratio of one pair and whether it misses its
// threshold. A colour it can't parse is skipped, as the grammar, not the
// contrast check, decides what is a colour.
func missingContrast(tokens map[string]string, p contrastPair) (float64, bool) {
	bg, ok := backdrop(tokens, p.bg)
	if !ok {
		return 0, false
	}
	fg, ok := parseColor(tokens[p.fg])
	if !ok {
		return 0, false
	}
	got := contrast(fg.over(bg), bg)
	return got, got < p.min
}

// backdrop returns the opaque colour a token is painted over: the ground over
// black (the window behind every theme), and every other surface over the
// ground.
func backdrop(tokens map[string]string, name string) (rgba, bool) {
	ground, ok := parseColor(tokens["ground"])
	if !ok {
		return rgba{}, false
	}
	ground = ground.over(black)
	if name == "ground" {
		return ground, true
	}
	c, ok := parseColor(tokens[name])
	if !ok {
		return rgba{}, false
	}
	return c.over(ground), true
}

// rgba is a colour in sRGB with straight alpha: channels and alpha run 0..1.
type rgba struct {
	r, g, b, a float64
}

// black is the backdrop of a theme, which may itself be translucent.
var black = rgba{a: 1}

// over paints c on top of dst, which must be opaque.
func (c rgba) over(dst rgba) rgba {
	if c.a >= 1 {
		return rgba{r: c.r, g: c.g, b: c.b, a: 1}
	}
	return rgba{
		r: c.r*c.a + dst.r*(1-c.a),
		g: c.g*c.a + dst.g*(1-c.a),
		b: c.b*c.a + dst.b*(1-c.a),
		a: 1,
	}
}

// luminance is the WCAG 2 relative luminance of an opaque colour.
func (c rgba) luminance() float64 {
	return 0.2126*srgbToLinear(c.r) + 0.7152*srgbToLinear(c.g) + 0.0722*srgbToLinear(c.b)
}

func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func linearToSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1.0/2.4) - 0.055
}

// contrast is the WCAG 2 contrast ratio of two opaque colours, 1..21.
func contrast(a, b rgba) float64 {
	la, lb := a.luminance(), b.luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// parseColor turns one of the colour values checkToken accepts into sRGB. It
// reports false for a value it can't read and the check then skips the pair:
// the grammar, not the contrast check, decides what is a colour.
func parseColor(v string) (rgba, bool) {
	s := strings.ToLower(strings.TrimSpace(v))
	switch {
	case s == "transparent":
		return rgba{a: 0}, true
	case strings.HasPrefix(s, "#"):
		return hexColor(s[1:])
	}
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return rgba{}, false
	}
	args := colourArgs(s[open+1 : len(s)-1])
	switch s[:open] {
	case "rgb", "rgba":
		return rgbColor(args)
	case "hsl", "hsla":
		return hslColor(args)
	case "oklch":
		return oklchColor(args)
	}
	return rgba{}, false
}

// colourArgs splits a colour function's arguments on commas, slashes and
// spaces, so the comma and the slash syntaxes both read as one list.
func colourArgs(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '/' || r == ' ' || r == '\t' || r == '\n'
	})
}

func hexColor(h string) (rgba, bool) {
	switch len(h) {
	case 3, 4:
		var digits []float64
		for i := 0; i < len(h); i++ {
			n, err := strconv.ParseUint(h[i:i+1], 16, 8)
			if err != nil {
				return rgba{}, false
			}
			digits = append(digits, float64(n)/15)
		}
		return withAlpha(digits), true
	case 6, 8:
		var digits []float64
		for i := 0; i < len(h); i += 2 {
			n, err := strconv.ParseUint(h[i:i+2], 16, 8)
			if err != nil {
				return rgba{}, false
			}
			digits = append(digits, float64(n)/255)
		}
		return withAlpha(digits), true
	}
	return rgba{}, false
}

// withAlpha builds a colour from three or four channels.
func withAlpha(digits []float64) rgba {
	c := rgba{r: digits[0], g: digits[1], b: digits[2], a: 1}
	if len(digits) == 4 {
		c.a = digits[3]
	}
	return c
}

// colourNumber reads one colour argument: its number and the unit it carries,
// "" for a plain number, "%" or "deg" (the grammar allows nothing else).
func colourNumber(s string) (v float64, unit string, ok bool) {
	switch {
	case strings.HasSuffix(s, "%"):
		s, unit = s[:len(s)-1], "%"
	case strings.HasSuffix(s, "deg"):
		s, unit = s[:len(s)-3], "deg"
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, unit, err == nil
}

// alpha reads the optional alpha argument of a colour function.
func alpha(arg string) (float64, bool) {
	v, unit, ok := colourNumber(arg)
	switch {
	case !ok || unit == "deg":
		return 0, false
	case unit == "%":
		return clamp01(v / 100), true
	}
	return clamp01(v), true
}

func rgbColor(args []string) (rgba, bool) {
	if len(args) != 3 && len(args) != 4 {
		return rgba{}, false
	}
	c := rgba{a: 1}
	channels := []*float64{&c.r, &c.g, &c.b}
	for i, arg := range args {
		if i == 3 {
			a, ok := alpha(arg)
			if !ok {
				return rgba{}, false
			}
			c.a = a
			continue
		}
		v, unit, ok := colourNumber(arg)
		switch {
		case !ok || unit == "deg":
			return rgba{}, false
		case unit == "%":
			*channels[i] = clamp01(v / 100)
		default:
			*channels[i] = clamp01(v / 255)
		}
	}
	return c, true
}

func hslColor(args []string) (rgba, bool) {
	if len(args) != 3 && len(args) != 4 {
		return rgba{}, false
	}
	var hsl [3]float64
	for i := 0; i < 3; i++ {
		v, unit, ok := colourNumber(args[i])
		switch {
		case !ok, i == 0 && unit == "%", i > 0 && unit == "deg":
			return rgba{}, false
		}
		// A plain number is a percentage in hsl(), bar the hue (CSS Color 4).
		if i > 0 {
			v /= 100
		}
		hsl[i] = v
	}
	c := hslToRGB(hsl[0], clamp01(hsl[1]), clamp01(hsl[2]))
	if len(args) == 4 {
		a, ok := alpha(args[3])
		if !ok {
			return rgba{}, false
		}
		c.a = a
	}
	return c, true
}

// hslToRGB follows the CSS Color 4 algorithm.
func hslToRGB(h, s, l float64) rgba {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	if s == 0 {
		return rgba{r: l, g: l, b: l, a: 1}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	return rgba{
		r: clamp01(hueToRGB(p, q, h+1.0/3)),
		g: clamp01(hueToRGB(p, q, h)),
		b: clamp01(hueToRGB(p, q, h-1.0/3)),
		a: 1,
	}
}

func hueToRGB(p, q, t float64) float64 {
	t = math.Mod(math.Mod(t, 1)+1, 1)
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

func oklchColor(args []string) (rgba, bool) {
	if len(args) != 3 && len(args) != 4 {
		return rgba{}, false
	}
	l, unit, ok := colourNumber(args[0])
	switch {
	case !ok, unit == "deg":
		return rgba{}, false
	case unit == "%":
		l /= 100
	}
	c, unit, ok := colourNumber(args[1])
	switch {
	case !ok, unit == "deg":
		return rgba{}, false
	case unit == "%":
		c = c / 100 * 0.4 // CSS Color 4: 100% chroma is 0.4.
	}
	h, unit, ok := colourNumber(args[2])
	if !ok || unit == "%" {
		return rgba{}, false
	}
	out := oklchToSRGB(clamp01(l), math.Max(c, 0), h)
	if len(args) == 4 {
		a, ok := alpha(args[3])
		if !ok {
			return rgba{}, false
		}
		out.a = a
	}
	return out, true
}

// oklchToSRGB converts an OKLCH colour to sRGB (Björn Ottosson's OKLab), with
// out-of-gamut channels clamped, as a browser paints them.
func oklchToSRGB(l, c, h float64) rgba {
	rad := h * math.Pi / 180
	a, b := c*math.Cos(rad), c*math.Sin(rad)
	ll := l + 0.3963377774*a + 0.2158037573*b
	mm := l - 0.1055613458*a - 0.0638541728*b
	ss := l - 0.0894841775*a - 1.2914855480*b
	ll, mm, ss = ll*ll*ll, mm*mm*mm, ss*ss*ss
	return rgba{
		r: clamp01(linearToSRGB(4.0767416621*ll - 3.3077115913*mm + 0.2309699292*ss)),
		g: clamp01(linearToSRGB(-1.2684380046*ll + 2.6097574011*mm - 0.3413193965*ss)),
		b: clamp01(linearToSRGB(-0.0041960863*ll - 0.7034186147*mm + 1.7076147010*ss)),
		a: 1,
	}
}

func clamp01(v float64) float64 {
	return math.Min(math.Max(v, 0), 1)
}
