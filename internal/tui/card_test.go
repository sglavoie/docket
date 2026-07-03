package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderTitledTopBorder_ExactWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	titleStyle := lipgloss.NewStyle().Bold(true)

	tests := []struct {
		name  string
		title string
		width int
	}{
		{"short title wide box", titleStyle.Render("Hi"), 40},
		{"medium title", titleStyle.Render("Confirm"), 30},
		{"long title tight fit", titleStyle.Render("A Longer Title Here"), 30},
		{"unstyled title", "Plain", 25},
		{"minimum dashes", titleStyle.Render("Confirm"), 13}, // 5 + 7 + 1 = 13
		{"wide box", titleStyle.Render("X"), 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTitledTopBorder(tt.title, tt.width, borderStyle)
			got := lipgloss.Width(result)
			if got != tt.width {
				t.Errorf("width = %d, want %d", got, tt.width)
			}
			stripped := ansi.Strip(result)
			if !strings.HasSuffix(stripped, "╮") {
				t.Errorf("expected border to end with corner char, got %q", stripped)
			}
		})
	}
}

func TestRenderTitledBottomBorder_ExactWidth(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))

	tests := []struct {
		name   string
		footer string
		width  int
	}{
		{"short footer", lipgloss.NewStyle().Render("Esc"), 30},
		{"long footer tight fit", lipgloss.NewStyle().Render("y confirm · n/Esc cancel"), 30},
		{"wide box", lipgloss.NewStyle().Render("footer"), 80},
		{"minimum dashes", lipgloss.NewStyle().Render("ok"), 8}, // 5 + 2 + 1 = 8
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTitledBottomBorder(tt.footer, tt.width, borderStyle)
			got := lipgloss.Width(result)
			if got != tt.width {
				t.Errorf("width = %d, want %d", got, tt.width)
			}
			stripped := ansi.Strip(result)
			if !strings.HasSuffix(stripped, "╯") {
				t.Errorf("expected border to end with corner char, got %q", stripped)
			}
		})
	}
}

func TestRenderTitledBorder_OverflowSafety(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	longTitle := lipgloss.NewStyle().Bold(true).Render(strings.Repeat("X", 50))

	// Width too small for the title — safety clamp should keep it bounded
	result := renderTitledTopBorder(longTitle, 20, borderStyle)
	got := lipgloss.Width(result)
	if got > 20 {
		t.Errorf("top border overflowed: width = %d, want <= 20", got)
	}

	longFooter := lipgloss.NewStyle().Render(strings.Repeat("Y", 50))
	result = renderTitledBottomBorder(longFooter, 20, borderStyle)
	got = lipgloss.Width(result)
	if got > 20 {
		t.Errorf("bottom border overflowed: width = %d, want <= 20", got)
	}
}

func TestRenderCardFixedHeightPadsShortContent(t *testing.T) {
	config := DefaultCardConfig()
	config.Height = 6

	rendered := ansi.Strip(renderCard("Fixed", "one", 24, config))
	lines := strings.Split(rendered, "\n")
	if got, want := len(lines), config.Height; got != want {
		t.Fatalf("fixed-height card rendered %d lines, want %d:\n%s", got, want, rendered)
	}
	if !strings.Contains(lines[1], "one") {
		t.Fatalf("expected original content to remain in first body row:\n%s", rendered)
	}
	for i := 2; i < len(lines)-1; i++ {
		if !strings.HasPrefix(lines[i], "│") || !strings.HasSuffix(lines[i], "│") {
			t.Fatalf("expected padded body line %d to preserve side borders, got %q\n%s", i, lines[i], rendered)
		}
	}
	if !strings.HasSuffix(lines[len(lines)-1], "╯") {
		t.Fatalf("expected fixed-height card to preserve bottom border:\n%s", rendered)
	}
}

func TestRenderCardFixedHeightTruncatesAndPreservesBottomBorder(t *testing.T) {
	config := DefaultCardConfig()
	config.Height = 4
	content := strings.Join([]string{"one", "two", "three", "four"}, "\n")

	rendered := ansi.Strip(renderCard("Fixed", content, 24, config))
	lines := strings.Split(rendered, "\n")
	if got, want := len(lines), config.Height; got != want {
		t.Fatalf("fixed-height card rendered %d lines, want %d:\n%s", got, want, rendered)
	}
	if strings.Contains(rendered, "four") {
		t.Fatalf("expected overflowing content to be truncated:\n%s", rendered)
	}
	if !strings.HasSuffix(lines[len(lines)-1], "╯") {
		t.Fatalf("expected truncated fixed-height card to preserve bottom border:\n%s", rendered)
	}
}

func TestRenderCardWrapsLongSeparatorRowsWithinOuterWidth(t *testing.T) {
	content := strings.Join([]string{
		"| Raw ingredients                                                     | Spoken output |",
		"| ------------------------------------------------------------------- | ------------------------------------------------------------------------------ |",
		"| `INTERESTRATE:IRSWAP:FIXEDFLOAT` + `EUR EURIBOR` + `5Y`             | \"five-year Euribor fixed-float swap\" |",
	}, "\n")

	const width = 64
	rendered := ansi.Strip(renderCard("Description", content, width, DefaultCardConfig()))
	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected card frame and body lines, got:\n%s", rendered)
	}

	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("card line %d width = %d, want <= %d: %q\n\n%s", i, got, width, line, rendered)
		}
	}
	if !strings.HasSuffix(lines[0], "╮") {
		t.Fatalf("expected top border to close within width, got %q\n%s", lines[0], rendered)
	}
	if !strings.HasSuffix(lines[len(lines)-1], "╯") {
		t.Fatalf("expected bottom border to close within width, got %q\n%s", lines[len(lines)-1], rendered)
	}
	for i, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Fatalf("expected body line %d to keep both side borders, got %q\n%s", i+1, line, rendered)
		}
	}
	if !strings.Contains(rendered, "fixed-float") {
		t.Fatalf("expected wrapped card content to be preserved, got:\n%s", rendered)
	}
}
