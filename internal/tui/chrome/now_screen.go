package chrome

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// NowScreenSpec is the Now screen: one task in Doing, read whole. Progress
// arrives rendered; every other field is plain text.
type NowScreenSpec struct {
	Title      string
	Meta       string
	Elapsed    string
	ResumeNote string
	Steps      []NowScreenStep
	Progress   string
	Beside     string
}

// NowScreenStep is one checklist item as the screen lists it.
type NowScreenStep struct {
	Text string
	Done bool
}

// nowScreenMaxWidth keeps the column a reading measure on a wide terminal, so
// the title sits as a headline rather than a line across the screen.
const nowScreenMaxWidth = 64

// nowScreenMaxSteps bounds the checklist so the screen stays one quiet column;
// the steps around the next one are what it is read for.
const nowScreenMaxSteps = 7

// RenderNowScreen draws the screen at exactly width by height: one centered
// column under an accent rail, everything else blank. Its keys are the
// footer's, so the screen names none of its own.
func RenderNowScreen(spec NowScreenSpec, width, height int) string {
	width, height = max(width, 1), max(height, 1)
	column := min(nowScreenMaxWidth, max(width-8, 20))
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	muted := theme.MutedStyle
	note := lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true)

	var body []string
	body = append(body, accent.Render("NOW")+muted.Render("  ·  "+spec.Elapsed), "")
	for _, line := range textfit.WrapText(strings.TrimSpace(spec.Title), column) {
		body = append(body, theme.BoldPrimaryStyle.Render(line))
	}
	if spec.Meta != "" {
		body = append(body, muted.Render(textfit.TruncateIntentional(spec.Meta, column, "…")))
	}
	if strings.TrimSpace(spec.ResumeNote) != "" {
		body = append(body, "")
		for _, line := range textfit.WrapText("Left off: "+strings.TrimSpace(spec.ResumeNote), column) {
			body = append(body, note.Render(line))
		}
	}
	if len(spec.Steps) > 0 || spec.Progress != "" {
		body = append(body, "")
		if spec.Progress != "" {
			body = append(body, spec.Progress)
		}
		body = append(body, nowScreenSteps(spec.Steps, column)...)
	}
	if spec.Beside != "" {
		body = append(body, "", muted.Render(spec.Beside))
	}

	rail := accent.Render("▎") + " "
	for i, line := range body {
		body[i] = rail + line
	}
	// The column is centered on the frame and stands a little above its middle,
	// where the eye rests on an otherwise empty screen.
	left := max((width-column-2)/2, 0)
	top := max((height-len(body))*2/5, 0)
	lines := make([]string, 0, height)
	for range top {
		lines = append(lines, strings.Repeat(" ", width))
	}
	for _, line := range body {
		if len(lines) == height {
			break
		}
		lines = append(lines, padToWidth(strings.Repeat(" ", left)+line, width))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// nowScreenSteps lists the checklist around its next open step: that step is
// marked and bright, finished ones are muted, and a long list keeps the next
// step in the window rather than its head.
func nowScreenSteps(steps []NowScreenStep, width int) []string {
	next := -1
	for i, step := range steps {
		if !step.Done {
			next = i
			break
		}
	}
	start := 0
	if len(steps) > nowScreenMaxSteps && next > 0 {
		start = min(max(next-1, 0), len(steps)-nowScreenMaxSteps)
	}
	end := min(start+nowScreenMaxSteps, len(steps))
	success := lipgloss.NewStyle().Foreground(theme.ColorSuccess)
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	var lines []string
	if start > 0 {
		lines = append(lines, theme.MutedStyle.Render("  ↑ "+pluralSteps(start)+" done above"))
	}
	for i := start; i < end; i++ {
		step := steps[i]
		text := textfit.TruncateIntentional(strings.TrimSpace(step.Text), max(width-4, 8), "…")
		switch {
		case step.Done:
			lines = append(lines, success.Render("✓ ")+theme.MutedStyle.Render(text))
		case i == next:
			lines = append(lines, accent.Render("→ ")+theme.BoldPrimaryStyle.Render(text))
		default:
			lines = append(lines, theme.MutedStyle.Render("○ ")+text)
		}
	}
	if end < len(steps) {
		lines = append(lines, theme.MutedStyle.Render("  ↓ "+pluralSteps(len(steps)-end)+" more"))
	}
	return lines
}

func pluralSteps(n int) string {
	if n == 1 {
		return "1 step"
	}
	return fmt.Sprintf("%d steps", n)
}
