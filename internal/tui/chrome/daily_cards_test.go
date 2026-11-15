package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestNowCardLeadsWithLabel(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	for _, name := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: name})
		for _, tc := range []struct {
			name, title, meta, want string
			width                   int
			label, metadata         bool
		}{
			{"roomy", "Build login form", "site · 12m in Doing", "Build login form", 80, true, true},
			{"label exactly fits", "Build login form", "site · 12m in Doing", "Build login form", 44, true, true},
			{"shed label first", "Build login form", "site · 12m in Doing", "Build login form", 43, false, true},
			{"no metadata exact fit", "Build login form", "", "Build login form", 23, true, false},
			{"no metadata shed label", "Build login form", "", "Build login form", 22, false, false},
			{"title before metadata", "A fairly long task title", "website · 3h in Doing", "A fairly long task title", 30, false, false},
			{"truncate only after label", strings.Repeat("x", 70), "site", strings.Repeat("x", 51) + "…", 60, false, true},
			{"wide glyph title", "界界界界界界界界", "", "界界界界界界界界", 23, true, false},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				line := RenderNowCard(NowCardSpec{Title: tc.title, Meta: tc.meta}, tc.width)[0]
				plain := ansi.Strip(line)
				if strings.Contains(plain, "Now:") != tc.label || !strings.Contains(plain, tc.want) {
					t.Fatalf("unexpected label/title: %q", plain)
				}
				if tc.meta != "" && strings.Contains(plain, tc.meta) != tc.metadata {
					t.Fatalf("metadata policy changed: %q", plain)
				}
				if tc.label && !strings.Contains(line, theme.MutedStyle.Render("Now: ")) {
					t.Fatalf("label is not muted: %q", line)
				}
				if lipgloss.Width(line) != tc.width {
					t.Fatalf("width = %d, want %d: %q", lipgloss.Width(line), tc.width, plain)
				}
			})
		}
	}
}
