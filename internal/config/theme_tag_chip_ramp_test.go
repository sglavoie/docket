package config

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// caller may reason about.
const tagChipRampSlots = 6

func TestDefaultTagChipRamp_DispatchesToVariant(t *testing.T) {
	dark := DefaultTagChipRamp("dark")
	light := DefaultTagChipRamp("light")

	if slices.Equal(dark, light) {
		t.Error("DefaultTagChipRamp(dark) and DefaultTagChipRamp(light) must not be equal")
	}
	if got := DefaultTagChipRamp("unknown"); !slices.Equal(got, dark) {
		t.Errorf("DefaultTagChipRamp(unknown) = %v, want the dark ramp %v", got, dark)
	}

	for variant, ramp := range map[string][]string{"dark": dark, "light": light} {
		if len(ramp) != tagChipRampSlots {
			t.Errorf("DefaultTagChipRamp(%s) has %d slots, want %d", variant, len(ramp), tagChipRampSlots)
		}
		for slot, hex := range ramp {
			if len(hex) != 7 || !strings.HasPrefix(hex, "#") {
				t.Errorf("DefaultTagChipRamp(%s) slot %d = %q, want a #rrggbb value", variant, slot, hex)
			}
		}
	}

	// Every caller gets its own slice, so a consumer that sorts or rewrites the
	// ramp cannot repaint the ramp for everyone else.
	DefaultTagChipRamp("dark")[0] = "#000000"
	if got := DefaultTagChipRamp("dark"); got[0] != dark[0] {
		t.Errorf("mutating a returned ramp changed slot 0 to %q, want %q", got[0], dark[0])
	}
}

// TestDefaultTagChipRampSlotsStayPerceptuallyDistinct guards what the ramp is
// for: two tags on the same row must not read as one color. Chips genuinely sit
// adjacent, so the floor is higher than the one against the title colors, for
// the same reason the semantic titles carry their own within-tier floor —
// contrast cannot see this, because two colors can each be readable against the
// background and still be the same color to a reader.
func TestDefaultTagChipRampSlotsStayPerceptuallyDistinct(t *testing.T) {
	const minimumDeltaE = 25.0

	for _, variant := range []string{"dark", "light"} {
		ramp := DefaultTagChipRamp(variant)
		for i := 0; i < len(ramp); i++ {
			for j := i + 1; j < len(ramp); j++ {
				if got := deltaE76(t, ramp[i], ramp[j]); got < minimumDeltaE {
					t.Errorf("%s: chip slots %d (%s) and %d (%s) differ by only %.1f, want at least %.1f",
						variant, i, ramp[i], j, ramp[j], got, minimumDeltaE)
				}
			}
		}
	}
}

// TestDefaultTagChipRampStaysClearOfTitleColors keeps a chip from being mistaken
// for the title colors it shares a row with. The floor is lower than the
// within-ramp one on purpose: a chip and a title are never adjacent, so they
// need to be tellable apart, not instantly separable.
func TestDefaultTagChipRampStaysClearOfTitleColors(t *testing.T) {
	const minimumDeltaE = 18.0

	for _, tc := range []struct {
		variant string
		palette PaletteColors
	}{
		{"dark", DefaultDarkTheme().Palette},
		{"light", DefaultLightTheme().Palette},
	} {
		neighbors := map[string]string{
			"SemanticBugTitle":      tc.palette.SemanticBugTitle,
			"SemanticQuestionTitle": tc.palette.SemanticQuestionTitle,
			"SemanticEpicTitle":     tc.palette.SemanticEpicTitle,
			"SemanticStoryTitle":    tc.palette.SemanticStoryTitle,
			"SemanticFeatureTitle":  tc.palette.SemanticFeatureTitle,
			"SemanticIdeaTitle":     tc.palette.SemanticIdeaTitle,
			"Priority1Title":        tc.palette.Priority1Title,
			"Priority2Title":        tc.palette.Priority2Title,
			"Priority3Title":        tc.palette.Priority3Title,
			"TaskContext":           tc.palette.TaskContext,
		}
		names := slices.Sorted(maps.Keys(neighbors))
		for slot, chip := range DefaultTagChipRamp(tc.variant) {
			for _, name := range names {
				if got := deltaE76(t, chip, neighbors[name]); got < minimumDeltaE {
					t.Errorf("%s: chip slot %d (%s) and %s (%s) differ by only %.1f, want at least %.1f",
						tc.variant, slot, chip, name, neighbors[name], got, minimumDeltaE)
				}
			}
		}
	}
}

// TestDefaultTagChipRampContrastsWithRowBackgrounds is the check easiest to
// forget and the most visible when it fails: a chip is drawn on the alert row
// backgrounds too, not only on the theme background. The 3.0:1 floor is
// deliberately below the 4.5:1 TaskContext carries in
// TestDefaultTaskContextColorsContrastWithRowAlerts, because a chip is a short
// secondary segment rather than required reading. The row backgrounds are the
// same hardcoded literals that test uses; they are not palette fields.
func TestDefaultTagChipRampContrastsWithRowBackgrounds(t *testing.T) {
	const minimumContrast = 3.0

	for _, tc := range []struct {
		variant     string
		backgrounds map[string]string
	}{
		{"dark", map[string]string{
			"theme background": DefaultDarkTheme().BackgroundColor,
			"waiting row":      "#2e1515",
			"overdue row":      "#2d1f0e",
		}},
		{"light", map[string]string{
			"theme background": DefaultLightTheme().BackgroundColor,
			"waiting row":      "#fde8e8",
			"overdue row":      "#fef3e2",
		}},
	} {
		names := slices.Sorted(maps.Keys(tc.backgrounds))
		for slot, chip := range DefaultTagChipRamp(tc.variant) {
			for _, name := range names {
				got := wcagContrastRatio(t, chip, tc.backgrounds[name])
				if got < minimumContrast {
					t.Errorf("%s: chip slot %d (%s) on %s (%s) contrast = %.2f:1, want at least %.1f:1",
						tc.variant, slot, chip, name, tc.backgrounds[name], got, minimumContrast)
				}
			}
		}
	}
}

// TestDefaultTagChipRampStaysInTheMutedTier makes the DefaultDarkTheme doc
// comment checkable: the ramp is the ≤65% saturation tier, which is what keeps a
// chip subordinate to the full-chroma semantic titles by construction.
func TestDefaultTagChipRampStaysInTheMutedTier(t *testing.T) {
	const maximumSaturation = 0.65

	for _, variant := range []string{"dark", "light"} {
		for slot, chip := range DefaultTagChipRamp(variant) {
			if got := hslSaturation(t, chip); got > maximumSaturation {
				t.Errorf("%s: chip slot %d (%s) saturation = %.2f, want at most %.2f",
					variant, slot, chip, got, maximumSaturation)
			}
		}
	}
}

// TestDefaultTagChipRampKeepsSlotIdentityAcrossVariants holds the promise that
// makes the color a mnemonic: a tag resolves to a slot index, so if slot 3 were
// teal in dark and closer to slot 4's family in light, toggling the theme would
// silently swap two tags' identities. Every light slot must therefore be nearer
// its own dark slot than any other.
func TestDefaultTagChipRampKeepsSlotIdentityAcrossVariants(t *testing.T) {
	dark := DefaultTagChipRamp("dark")
	light := DefaultTagChipRamp("light")

	for slot, chip := range light {
		nearest, nearestDistance := -1, math.Inf(1)
		for candidate, darkChip := range dark {
			if got := deltaE76(t, chip, darkChip); got < nearestDistance {
				nearest, nearestDistance = candidate, got
			}
		}
		if nearest != slot {
			t.Errorf("light chip slot %d (%s) is nearest dark slot %d (%s), want dark slot %d (%s)",
				slot, chip, nearest, dark[nearest], slot, dark[slot])
		}
	}
}

// deltaE76 reports the CIE 1976 perceptual distance between two #rrggbb colors.
func deltaE76(t *testing.T, a, b string) float64 {
	t.Helper()

	l1, a1, b1 := labFromHex(t, a)
	l2, a2, b2 := labFromHex(t, b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// labFromHex converts a #rrggbb color to CIE Lab under the D65 white point.
func labFromHex(t *testing.T, hex string) (float64, float64, float64) {
	t.Helper()

	value := strings.TrimPrefix(hex, "#")
	if len(value) != 6 {
		t.Fatalf("invalid hex color %q: want #rrggbb", hex)
	}
	r := linearizedColorChannel(t, value[0:2])
	g := linearizedColorChannel(t, value[2:4])
	b := linearizedColorChannel(t, value[4:6])

	x := (0.4124*r + 0.3576*g + 0.1805*b) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*b
	z := (0.0193*r + 0.1192*g + 0.9505*b) / 1.08883

	f := func(v float64) float64 {
		if v > 216.0/24389.0 {
			return math.Cbrt(v)
		}
		return v*(841.0/108.0) + 4.0/29.0
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

// hslSaturation reports the HSL saturation of a #rrggbb color, the measure the
// theme doc comments state their ≤65% rule in.
func hslSaturation(t *testing.T, hex string) float64 {
	t.Helper()

	value := strings.TrimPrefix(hex, "#")
	if len(value) != 6 {
		t.Fatalf("invalid hex color %q: want #rrggbb", hex)
	}
	channel := func(pair string) float64 {
		n, err := strconv.ParseUint(pair, 16, 8)
		if err != nil {
			t.Fatalf("invalid hex channel %q: %v", pair, err)
		}
		return float64(n) / 255
	}
	r, g, b := channel(value[0:2]), channel(value[2:4]), channel(value[4:6])

	high, low := max(r, g, b), min(r, g, b)
	if high == low {
		return 0
	}
	if lightness := (high + low) / 2; lightness > 0.5 {
		return (high - low) / (2 - high - low)
	}
	return (high - low) / (high + low)
}

func wcagContrastRatio(t *testing.T, a, b string) float64 {
	t.Helper()

	l1 := relativeLuminance(t, a)
	l2 := relativeLuminance(t, b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(t *testing.T, hex string) float64 {
	t.Helper()

	value := strings.TrimPrefix(hex, "#")
	if len(value) != 6 {
		t.Fatalf("invalid hex color %q: want #rrggbb", hex)
	}

	r := linearizedColorChannel(t, value[0:2])
	g := linearizedColorChannel(t, value[2:4])
	b := linearizedColorChannel(t, value[4:6])
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearizedColorChannel(t *testing.T, hex string) float64 {
	t.Helper()

	n, err := strconv.ParseUint(hex, 16, 8)
	if err != nil {
		t.Fatalf("invalid hex channel %q: %v", hex, err)
	}

	c := float64(n) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
