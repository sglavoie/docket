package theme

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var projectColorHexPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func TestProjectColorPaletteLiteralsAndUniqueness(t *testing.T) {
	if got := len(projectColorPalette); got != config.ProjectColorSlotCount {
		t.Fatalf("palette length = %d, want %d", got, config.ProjectColorSlotCount)
	}

	darkSeen := make(map[string]int, len(projectColorPalette))
	lightSeen := make(map[string]int, len(projectColorPalette))
	for i, pair := range projectColorPalette {
		slot := i + 1
		for theme, literal := range map[string]string{"dark": pair.Dark, "light": pair.Light} {
			if !projectColorHexPattern.MatchString(literal) {
				t.Errorf("slot %d %s literal = %q, want #rrggbb", slot, theme, literal)
			}
		}
		if previous, exists := darkSeen[pair.Dark]; exists {
			t.Errorf("dark color %q is duplicated in slots %d and %d", pair.Dark, previous, slot)
		}
		if previous, exists := lightSeen[pair.Light]; exists {
			t.Errorf("light color %q is duplicated in slots %d and %d", pair.Light, previous, slot)
		}
		darkSeen[pair.Dark] = slot
		lightSeen[pair.Light] = slot
	}
}

func TestProjectColorPaletteContrast(t *testing.T) {
	themes := []struct {
		name       string
		foreground func(ProjectColorPair) string
		background []string
	}{
		{
			name:       "dark",
			foreground: func(pair ProjectColorPair) string { return pair.Dark },
			background: projectColorTestBackgrounds(config.DefaultDarkTheme()),
		},
		{
			name:       "light",
			foreground: func(pair ProjectColorPair) string { return pair.Light },
			background: projectColorTestBackgrounds(config.DefaultLightTheme()),
		},
	}

	for _, variant := range themes {
		t.Run(variant.name, func(t *testing.T) {
			for i, pair := range projectColorPalette {
				foreground := variant.foreground(pair)
				for _, background := range variant.background {
					if ratio := projectColorContrast(foreground, background); ratio < 4.5 {
						t.Errorf("slot %d %s contrast %s on %s = %.3f, want >= 4.5", i+1, variant.name, foreground, background, ratio)
					}
				}
			}
		})
	}
}

func TestProjectColorPaletteCIE76DistanceAndHuePairing(t *testing.T) {
	const minimumDistance = 6.5
	minimumObserved := math.MaxFloat64
	for i, first := range projectColorPalette {
		if difference := projectColorHueDifference(first.Dark, first.Light); difference > 2 {
			t.Errorf("slot %d dark/light hue difference = %.3f degrees, want <= 2", i+1, difference)
		}
		for j := i + 1; j < len(projectColorPalette); j++ {
			second := projectColorPalette[j]
			weakerDistance := math.Min(
				projectColorCIE76(first.Dark, second.Dark),
				projectColorCIE76(first.Light, second.Light),
			)
			minimumObserved = math.Min(minimumObserved, weakerDistance)
			if weakerDistance < minimumDistance {
				t.Errorf("slots %d and %d weaker CIE76 distance = %.3f, want >= %.1f", i+1, j+1, weakerDistance, minimumDistance)
			}
		}
	}
	t.Logf("minimum weaker-theme CIE76 distance: %.3f", minimumObserved)
}

func TestProjectColorResolutionAndFallback(t *testing.T) {
	previousTheme := ActiveThemeName
	defer func() { ActiveThemeName = previousTheme }()

	project := &models.Project{ColorSlot: 1}
	ActiveThemeName = ""
	if got, want := projectColor(project), lipgloss.Color(projectColorPalette[0].Dark); got != want {
		t.Errorf("empty theme projectColor = %v, want dark %v", got, want)
	}
	ActiveThemeName = "dark"
	if got, want := projectColor(project), lipgloss.Color(projectColorPalette[0].Dark); got != want {
		t.Errorf("dark projectColor = %v, want %v", got, want)
	}
	ActiveThemeName = "light"
	if got, want := projectColor(project), lipgloss.Color(projectColorPalette[0].Light); got != want {
		t.Errorf("light projectColor = %v, want %v", got, want)
	}

	for _, invalid := range []*models.Project{nil, {}, {ColorSlot: -1}, {ColorSlot: config.ProjectColorSlotCount + 1}} {
		if got := projectColor(invalid); got != nil {
			t.Errorf("projectColor(%+v) = %v, want nil", invalid, got)
		}
		if got, want := fmt.Sprint(ProjectColorStyle(invalid).GetForeground()), fmt.Sprint(MutedStyle.GetForeground()); got != want {
			t.Errorf("ProjectColorStyle(%+v) foreground = %q, want muted %q", invalid, got, want)
		}
	}
}

func projectColorTestBackgrounds(defaults config.Theme) []string {
	return []string{
		defaults.BackgroundColor,
		defaults.Palette.DimmedBg,
		darkenHex(defaults.BackgroundColor, zebraBackgroundDarkenFactor),
	}
}

func projectColorContrast(foreground, background string) float64 {
	foregroundLuminance := projectColorLuminance(projectColorRGB(foreground))
	backgroundLuminance := projectColorLuminance(projectColorRGB(background))
	if foregroundLuminance < backgroundLuminance {
		foregroundLuminance, backgroundLuminance = backgroundLuminance, foregroundLuminance
	}
	return (foregroundLuminance + 0.05) / (backgroundLuminance + 0.05)
}

func projectColorLuminance(rgb [3]float64) float64 {
	linearize := func(channel float64) float64 {
		channel /= 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linearize(rgb[0]) + 0.7152*linearize(rgb[1]) + 0.0722*linearize(rgb[2])
}

func projectColorCIE76(first, second string) float64 {
	firstLab := projectColorLab(first)
	secondLab := projectColorLab(second)
	deltaL := firstLab[0] - secondLab[0]
	deltaA := firstLab[1] - secondLab[1]
	deltaB := firstLab[2] - secondLab[2]
	return math.Sqrt(deltaL*deltaL + deltaA*deltaA + deltaB*deltaB)
}

func projectColorLab(hex string) [3]float64 {
	rgb := projectColorRGB(hex)
	linearize := func(channel float64) float64 {
		channel /= 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	r, g, b := linearize(rgb[0]), linearize(rgb[1]), linearize(rgb[2])
	x := (0.4124564*r + 0.3575761*g + 0.1804375*b) / 0.95047
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := (0.0193339*r + 0.1191920*g + 0.9503041*b) / 1.08883
	transform := func(value float64) float64 {
		if value > 0.008856 {
			return math.Cbrt(value)
		}
		return 7.787*value + 16.0/116.0
	}
	x, y, z = transform(x), transform(y), transform(z)
	return [3]float64{116*y - 16, 500 * (x - y), 200 * (y - z)}
}

func projectColorHueDifference(first, second string) float64 {
	firstHue := projectColorHue(projectColorRGB(first))
	secondHue := projectColorHue(projectColorRGB(second))
	difference := math.Abs(firstHue - secondHue)
	return math.Min(difference, 360-difference)
}

func projectColorHue(rgb [3]float64) float64 {
	r, g, b := rgb[0]/255, rgb[1]/255, rgb[2]/255
	maximum := math.Max(r, math.Max(g, b))
	minimum := math.Min(r, math.Min(g, b))
	delta := maximum - minimum
	if delta == 0 {
		return 0
	}
	var hue float64
	switch maximum {
	case r:
		hue = math.Mod((g-b)/delta, 6)
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue
}

func projectColorRGB(hex string) [3]float64 {
	var rgb [3]float64
	for i := range rgb {
		value, err := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		if err != nil {
			panic(fmt.Sprintf("invalid test color %q: %v", hex, err))
		}
		rgb[i] = float64(value)
	}
	return rgb
}
