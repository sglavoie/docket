package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestRenderOverlayBoxCompleteFrameAndHalo(t *testing.T) {
	originalHaloStyle := theme.OverlayHaloStyle
	theme.OverlayHaloStyle = lipgloss.NewStyle().Background(lipgloss.Color("#112233"))
	t.Cleanup(func() { theme.OverlayHaloStyle = originalHaloStyle })

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	title := borderStyle.Bold(true).Render("Choose")
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Enter · Esc")

	const frameWidth = 24
	rendered := RenderOverlayBox("first\nsecond", frameWidth, title, footer, borderStyle)
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

	haloCell := theme.OverlayHaloStyle.Render(" ")
	if !strings.HasPrefix(lines[1], haloCell) || !strings.HasSuffix(lines[1], haloCell) {
		t.Fatalf("side halo cells are missing their dim background style: %q", lines[1])
	}
	expectedFrame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderStyle.GetForeground()).Padding(0, 1).Width(frameWidth).Render(title + "\nfirst\nsecond\n" + footer)
	expectedTopBorder := strings.Split(expectedFrame, "\n")[0]
	if !strings.Contains(lines[1], expectedTopBorder) {
		t.Fatal("semantic border style was not applied to the frame")
	}
}
