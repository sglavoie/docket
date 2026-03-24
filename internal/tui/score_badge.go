package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// HasManualScores returns true if the task has any manually-set score dimension
// that matches a dimension in the given profile.
func HasManualScores(task *models.Task, profile *scoring.ScoringProfile) bool {
	if len(task.ScoreDimensions) == 0 {
		return false
	}
	for _, dim := range profile.Dimensions {
		if _, ok := task.ScoreDimensions[dim.ID]; ok {
			return true
		}
	}
	return false
}

// scoreColor returns a color based on the score percentage using a 5-bucket gradient.
// red(196) < orange(208) < yellow(226) < light-green(118) < green(42)
func scoreColor(pct float64) color.Color {
	switch {
	case pct < 60:
		return ColorError
	case pct < 120:
		return ColorOrange
	case pct < 180:
		return ColorAccent
	case pct < 240:
		return lipgloss.Color("118")
	default:
		return ColorSuccess
	}
}

// RenderScoreBadge computes the score for a task and returns a colored [S:N] badge.
// Unscored tasks (no manual dimensions set) show [S:~N] with the tilde indicating
// the score is purely derived/defaults.
func RenderScoreBadge(task *models.Task, profile *scoring.ScoringProfile) string {
	if profile == nil {
		return ""
	}
	score := scoring.ComputeScore(task, profile)
	pct := scoring.ScorePercent(score, profile)
	color := scoreColor(pct)
	style := lipgloss.NewStyle().Foreground(color)

	if HasManualScores(task, profile) {
		return style.Render(fmt.Sprintf("[S:%d]", int(pct)))
	}
	return style.Render(fmt.Sprintf("[S:~%d]", int(pct)))
}

// RenderScoreBar returns a block character sized by percentage, colored by gradient.
// Uses ▁▂▃▄▅▆▇█ (8 levels).
func RenderScoreBar(task *models.Task, profile *scoring.ScoringProfile) string {
	if profile == nil {
		return ""
	}
	score := scoring.ComputeScore(task, profile)
	pct := scoring.ScorePercent(score, profile)
	color := scoreColor(pct)
	style := lipgloss.NewStyle().Foreground(color)

	bars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	idx := int(pct / 37.5)
	if idx >= len(bars) {
		idx = len(bars) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return style.Render(string(bars[idx]))
}

// renderScoreCardContent renders the score breakdown body (header + per-dimension lines)
// shared by the sidebar and task detail score cards. The innerWidth should be
// the card's content width (total width minus card chrome, typically width-6).
func renderScoreCardContent(task *models.Task, profile *scoring.ScoringProfile, innerWidth int) string {
	results := scoring.ScoreBreakdown(task, profile)

	var lines []string

	bar := RenderScoreBar(task, profile)
	badge := RenderScoreBadge(task, profile)
	formulaStr := MutedStyle.Render(fmt.Sprintf("(%s)", profile.Formula))
	lines = append(lines, fmt.Sprintf("%s %s  %s", bar, badge, formulaStr))
	lines = append(lines, "")

	manualStyle := InfoStatusStyle
	derivedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("44"))

	maxNameWidth := 0
	for _, r := range results {
		if width := lipgloss.Width(r.DimensionName); width > maxNameWidth {
			maxNameWidth = width
		}
	}
	if maxNameWidth > innerWidth-16 {
		maxNameWidth = innerWidth - 16
	}

	for _, r := range results {
		name := truncateToVisualWidth(r.DimensionName, maxNameWidth)

		valStr := fmt.Sprintf("%d", r.Value)

		var sourceLabel string
		switch r.Source {
		case "manual":
			sourceLabel = manualStyle.Render("manual")
		case "derived":
			sourceLabel = derivedStyle.Render("derived")
		default:
			sourceLabel = MutedStyle.Render("default")
		}

		roleStr := ""
		switch r.Role {
		case scoring.RoleNumerator:
			roleStr = MutedStyle.Render(" [n]")
		case scoring.RoleDenominator:
			roleStr = MutedStyle.Render(" [d]")
		default:
		}

		padding := max(maxNameWidth-lipgloss.Width(name), 0)
		line := fmt.Sprintf("%s%s %s %s%s",
			MutedStyle.Render(name), strings.Repeat(" ", padding),
			valStr, sourceLabel, roleStr)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
