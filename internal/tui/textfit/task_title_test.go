package textfit

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestTaskTitleCodeSpans(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	for _, name := range []string{"", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: name})
		for _, tc := range []struct {
			text string
			code []string
		}{
			{"Run `go test` today", []string{"`go test`"}},
			{"Use `α` and `界`", []string{"`α`", "`界`"}},
			{"Use ``a ` b`` today", []string{"``a ` b``"}},
			{"Unfinished `command", nil},
			{"Escaped \\`command\\`", nil},
			{"Ordinary task", nil},
		} {
			for _, base := range []lipgloss.Style{theme.NormalStyle, theme.SelectedTaskTitleStyle, theme.CanceledStyle} {
				got := RenderTaskTitle(tc.text, nil, base)
				if ansi.Strip(got) != tc.text || lipgloss.Width(got) != lipgloss.Width(tc.text) {
					t.Fatalf("title text or geometry changed: %q", got)
				}
				if len(tc.code) == 0 && got != base.Render(tc.text) {
					t.Fatalf("literal backticks changed styling: %q", got)
				}
				for _, code := range tc.code {
					want := base.Foreground(theme.ColorAccent).Render(code)
					if want == base.Render(code) || !strings.Contains(got, want) {
						t.Fatalf("theme %q: code lacks distinct color or inherited style: %q", name, got)
					}
				}
			}
		}
	}
}

func TestTaskTitleCodeSearchAndClipping(t *testing.T) {
	base := theme.NormalStyle
	text := "Run `界 command` today"
	got := RenderTaskTitle(text, []int{5, 6, 7}, base)
	if !strings.Contains(got, theme.HighlightStyle.Inherit(base).Render("界 c")) {
		t.Fatalf("search match inside code lost its highlight: %q", got)
	}
	clipped := TruncateToVisualWidth(RenderTaskTitle(text, nil, base), 9)
	if ansi.Strip(clipped) != "Run `界 …" || lipgloss.Width(clipped) != 9 {
		t.Fatalf("clipped code changed geometry: %q", clipped)
	}
	if !strings.Contains(clipped, base.Foreground(theme.ColorAccent).Render("`界 ")) {
		t.Fatalf("closing backtick outside viewport lost code color: %q", clipped)
	}
}
