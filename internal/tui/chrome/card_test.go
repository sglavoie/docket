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

// TestFitOverlayTitleKeepsOneRowAndTheSuffixWhole sweeps the widths an overlay
// title is asked for. The result never exceeds the width, so it never wraps; the
// suffix is whole wherever the title and suffix alone fit; and the subject is
// the part that gives way, untouched once there is room for all three.
func TestFitOverlayTitleKeepsOneRowAndTheSuffixWhole(t *testing.T) {
	const (
		title   = "Set Due, Defer, or Check-In"
		subject = "国際化 🚀 café résumé release check"
		suffix  = " · due Fri Aug 14"
	)
	whole := title + ": " + subject + suffix
	for width := 1; width <= lipgloss.Width(whole)+4; width++ {
		got := FitOverlayTitle(title, subject, suffix, width)
		if lipgloss.Width(got) > width {
			t.Fatalf("width %d: %q is %d columns wide", width, got, lipgloss.Width(got))
		}
		if strings.Contains(got, "\n") {
			t.Fatalf("width %d: title wrapped: %q", width, got)
		}
		if width >= lipgloss.Width(title+suffix) && !strings.HasSuffix(got, suffix) {
			t.Fatalf("width %d: %q lost the suffix", width, got)
		}
		if width >= lipgloss.Width(whole) && got != whole {
			t.Fatalf("width %d: %q, want the whole title %q", width, got, whole)
		}
	}
}

// TestFitOverlayTitleSpendsEveryColumnOnTheSubject holds the budget itself: a
// subject that exactly fills what the title leaves is not shortened, and one a
// column wider loses only its tail.
func TestFitOverlayTitleSpendsEveryColumnOnTheSubject(t *testing.T) {
	const title, subject = "Task Actions", "Build login form"
	width := lipgloss.Width(title + ": " + subject)
	if got := FitOverlayTitle(title, subject, "", width); got != title+": "+subject {
		t.Fatalf("exact fit = %q, want the subject whole", got)
	}
	if got := FitOverlayTitle(title, subject, "", width-1); got != title+": Build login fo…" {
		t.Fatalf("one column short = %q", got)
	}
	if got := FitOverlayTitle(title, "", "", width); got != title {
		t.Fatalf("no subject = %q, want the bare title", got)
	}
}
