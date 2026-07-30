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

func TestRenderTitledBorder_OverflowSafety(t *testing.T) {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	longTitle := lipgloss.NewStyle().Bold(true).Render(strings.Repeat("X", 50))

	// Width too small for the title — safety clamp should keep it bounded
	result := renderTitledTopBorder(longTitle, 20, borderStyle)
	got := lipgloss.Width(result)
	if got > 20 {
		t.Errorf("top border overflowed: width = %d, want <= 20", got)
	}
}

func TestRenderOverlayBoxCompleteFrameAndHalo(t *testing.T) {
	originalHaloStyle := OverlayHaloStyle
	OverlayHaloStyle = lipgloss.NewStyle().Background(lipgloss.Color("#112233"))
	t.Cleanup(func() { OverlayHaloStyle = originalHaloStyle })

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	title := borderStyle.Bold(true).Render("Choose")
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Enter · Esc")

	const frameWidth = 24
	rendered := renderOverlayBox("first\nsecond", frameWidth, title, footer, borderStyle)
	lines := strings.Split(rendered, "\n")
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = ansi.Strip(line)
		if got := lipgloss.Width(line); got != frameWidth+2 {
			t.Fatalf("line %d width = %d, want %d:\n%s", i, got, frameWidth+2, ansi.Strip(rendered))
		}
	}

	if got, want := len(lines), 8; got != want {
		t.Fatalf("overlay height = %d, want %d:\n%s", got, want, ansi.Strip(rendered))
	}
	if strings.TrimSpace(plain[0]) != "" || strings.TrimSpace(plain[len(plain)-1]) != "" {
		t.Fatalf("top and bottom halo rows must surround the frame:\n%s", ansi.Strip(rendered))
	}
	if !strings.HasPrefix(plain[1], " ╭") || !strings.HasSuffix(plain[1], "╮ ") {
		t.Fatalf("top border is not surrounded by side halo cells: %q", plain[1])
	}
	if !strings.HasPrefix(plain[len(plain)-2], " ╰") || !strings.HasSuffix(plain[len(plain)-2], "╯ ") {
		t.Fatalf("bottom border is not surrounded by side halo cells: %q", plain[len(plain)-2])
	}
	if strings.Contains(plain[1], "Choose") || !strings.Contains(plain[2], "Choose") {
		t.Fatalf("title must render inside an uninterrupted top border:\n%s", ansi.Strip(rendered))
	}
	if strings.Contains(plain[len(plain)-2], "Enter") || !strings.Contains(plain[len(plain)-3], "Enter · Esc") {
		t.Fatalf("help must render inside an uninterrupted bottom border:\n%s", ansi.Strip(rendered))
	}

	haloCell := OverlayHaloStyle.Render(" ")
	if !strings.HasPrefix(lines[1], haloCell) || !strings.HasSuffix(lines[1], haloCell) {
		t.Fatalf("side halo cells are missing their dim background style: %q", lines[1])
	}
	expectedFrame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderStyle.GetForeground()).
		Padding(0, 1).
		Width(frameWidth).
		Render(title + "\nfirst\nsecond\n" + footer)
	expectedTopBorder := strings.Split(expectedFrame, "\n")[0]
	if !strings.Contains(lines[1], expectedTopBorder) {
		t.Fatal("semantic border style was not applied to the frame")
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
