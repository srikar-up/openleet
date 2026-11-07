package theme

import (
	"math"
	"strings"
)

// ContrastRatio returns the WCAG contrast ratio between two #rrggbb colours.
// Invalid input returns 0 so callers can treat it as an audit failure.
func ContrastRatio(a, b string) float64 {
	la, okA := relativeLuminance(a)
	lb, okB := relativeLuminance(b)
	if !okA || !okB {
		return 0
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// ColorDistance returns the Euclidean RGB distance between two colours. It is
// used to keep status colours visually distinct even when their luminance is
// similar.
func ColorDistance(a, b string) float64 {
	ar, ag, ab, okA := parseHex(a)
	br, bg, bb, okB := parseHex(b)
	if !okA || !okB {
		return 0
	}
	dr := float64(ar - br)
	dg := float64(ag - bg)
	db := float64(ab - bb)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// EnsureContrast returns fg adjusted toward black or white until it reaches
// min contrast against bg. The hue is preserved by linear interpolation.
func EnsureContrast(fg, bg string, min float64) string {
	if min <= 1 {
		return fg
	}
	fr, fgc, fb, ok := parseHex(fg)
	if !ok {
		return fg
	}
	br, bgc, bb, ok := parseHex(bg)
	if !ok {
		return fg
	}
	if ContrastRatio(fg, bg) >= min {
		return fg
	}

	bgLum := relativeLuminanceRGB(br, bgc, bb)
	target := "#ffffff"
	if contrastAgainstLuminance(0, bgLum) > contrastAgainstLuminance(1, bgLum) {
		target = "#000000"
	}
	tr, tg, tb, _ := parseHex(target)

	best := fg
	for i := 1; i <= 100; i++ {
		t := float64(i) / 100
		candidate := rgbHex(
			blendChannel(fr, tr, t),
			blendChannel(fgc, tg, t),
			blendChannel(fb, tb, t),
		)
		if ContrastRatio(candidate, bg) >= min {
			return candidate
		}
		best = candidate
	}
	return best
}

// BestTextOn picks the first candidate with readable contrast on bg, then
// falls back to black or white, whichever is stronger.
func BestTextOn(bg string, candidates ...string) string {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" && ContrastRatio(candidate, bg) >= 4.5 {
			return candidate
		}
	}
	br, bgc, bb, ok := parseHex(bg)
	if !ok {
		return "#ffffff"
	}
	bgLum := relativeLuminanceRGB(br, bgc, bb)
	if contrastAgainstLuminance(1, bgLum) >= contrastAgainstLuminance(0, bgLum) {
		return "#ffffff"
	}
	return "#000000"
}

func relativeLuminance(value string) (float64, bool) {
	r, g, b, ok := parseHex(value)
	if !ok {
		return 0, false
	}
	return relativeLuminanceRGB(r, g, b), true
}

func relativeLuminanceRGB(r, g, b int) float64 {
	return 0.2126*linearChannel(r) + 0.7152*linearChannel(g) + 0.0722*linearChannel(b)
}

func linearChannel(value int) float64 {
	v := float64(value) / 255
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func contrastAgainstLuminance(lum, bgLum float64) float64 {
	if lum < bgLum {
		return (bgLum + 0.05) / (lum + 0.05)
	}
	return (lum + 0.05) / (bgLum + 0.05)
}

func blendChannel(from, to int, t float64) int {
	value := float64(from) + float64(to-from)*t
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return int(value + 0.5)
}

func rgbHex(r, g, b int) string {
	const digits = "0123456789abcdef"
	out := []byte{'#', digits[(r>>4)&0xf], digits[r&0xf], digits[(g>>4)&0xf], digits[g&0xf], digits[(b>>4)&0xf], digits[b&0xf]}
	return string(out)
}
