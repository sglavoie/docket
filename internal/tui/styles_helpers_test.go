package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGetTitlePriorityStyle(t *testing.T) {
	// Ensure styles are built from defaults.
	ApplyTheme(ThemeConfig{})

	tests := []struct {
		priority    int
		wantBold    bool
		wantItalic  bool
		wantFaint   bool
		wantColorFg color.Color
	}{
		{1, true, true, false, lipgloss.Color(defaultColorPriority1Title)},
		{2, true, false, false, lipgloss.Color(defaultColorPriority2Title)},
		{3, false, false, false, lipgloss.Color(defaultColorPriority3Title)},
		{4, false, false, false, lipgloss.Color(defaultColorPriority4Title)},
		{5, false, false, false, lipgloss.Color(defaultColorPriority5Title)},
	}

	for _, tc := range tests {
		s := GetTitlePriorityStyle(tc.priority)
		if s.GetBold() != tc.wantBold {
			t.Errorf("priority %d: Bold = %v, want %v", tc.priority, s.GetBold(), tc.wantBold)
		}
		if s.GetItalic() != tc.wantItalic {
			t.Errorf("priority %d: Italic = %v, want %v", tc.priority, s.GetItalic(), tc.wantItalic)
		}
		if s.GetFaint() != tc.wantFaint {
			t.Errorf("priority %d: Faint = %v, want %v", tc.priority, s.GetFaint(), tc.wantFaint)
		}
		if s.GetForeground() != tc.wantColorFg {
			t.Errorf("priority %d: Foreground = %v, want %v", tc.priority, s.GetForeground(), tc.wantColorFg)
		}
	}
}

func TestGetTitlePriorityStyleOutOfRange(t *testing.T) {
	ApplyTheme(ThemeConfig{})

	p3 := GetTitlePriorityStyle(3)
	for _, priority := range []int{0, 6, -1, 99} {
		s := GetTitlePriorityStyle(priority)
		if s.GetBold() != p3.GetBold() || s.GetItalic() != p3.GetItalic() ||
			s.GetFaint() != p3.GetFaint() || s.GetForeground() != p3.GetForeground() {
			t.Errorf("priority %d: expected P3 style, got different style", priority)
		}
	}
}
