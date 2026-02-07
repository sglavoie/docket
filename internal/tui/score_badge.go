package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

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
func scoreColor(pct float64) lipgloss.Color {
	switch {
	case pct < 20:
		return lipgloss.Color("196")
	case pct < 40:
		return lipgloss.Color("208")
	case pct < 60:
		return lipgloss.Color("226")
	case pct < 80:
		return lipgloss.Color("118")
	default:
		return lipgloss.Color("42")
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
		return style.Render(fmt.Sprintf("[S:%d]", int(score)))
	}
	return style.Render(fmt.Sprintf("[S:~%d]", int(score)))
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
	idx := int(pct / 12.5)
	if idx >= len(bars) {
		idx = len(bars) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return style.Render(string(bars[idx]))
}
