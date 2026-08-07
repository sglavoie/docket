package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
// Pass an optional accent color to override the title foreground color.
func renderFullscreenTitle(title, subtitle string, accent ...color.Color) string {
	return renderFullscreenTitleContent(fullscreenTitleStyle(accent...).Render(title), subtitle)
}

// renderFullscreenTitleWithPulse renders the shared one-row full-screen title
// with the workspace-wide pulse right-aligned. Transient presentations use
// renderFullscreenTitle instead and therefore never duplicate the host pulse.
func renderFullscreenTitleWithPulse(title, subtitle string, pulse WorkspacePulse, width int, accent ...color.Color) string {
	return renderFullscreenTitleContentWithPulse(
		fullscreenTitleStyle(accent...).Render(title),
		subtitle,
		pulse,
		width,
	)
}

func fullscreenTitleStyle(accent ...color.Color) lipgloss.Style {
	if len(accent) > 0 && accent[0] != nil {
		return lipgloss.NewStyle().Bold(true).Foreground(accent[0])
	}
	return TitleStyle
}

func renderFullscreenTitleContent(titleContent, subtitle string) string {
	t := titleContent
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

func renderFullscreenTitleContentWithPulse(titleContent, subtitle string, pulse WorkspacePulse, width int) string {
	if width <= 0 {
		return renderFullscreenTitleContent(titleContent, subtitle)
	}
	context := ""
	if subtitle != "" {
		context = MutedStyle.Render(subtitle)
	}
	return renderOperationalTitle(operationalTitleSpec{
		Surface: titleContent,
		Context: []string{context},
		Pulse:   &pulse,
	}, width)
}

func renderTaskScopedFullscreenTitleWithPulse(prefix string, task *models.Task, subtitle string, pulse WorkspacePulse, width int, accent ...color.Color) string {
	titleStyle := fullscreenTitleStyle(accent...)
	if task == nil {
		return renderFullscreenTitleContentWithPulse(titleStyle.Render(prefix), subtitle, pulse, width)
	}
	titleContent := titleStyle.Render(prefix) + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)
	return renderFullscreenTitleContentWithPulse(titleContent, subtitle, pulse, width)
}

// renderPriorityBadge renders a priority indicator with color styling. An unset
// priority has no glyph of its own, so it renders nothing rather than borrowing
// the normal-priority icon.
func renderPriorityBadge(priority int) string {
	if models.ValidatePriority(priority, false) != nil {
		return ""
	}
	return GetPriorityStyle(priority).Render(PriorityIcon(priority))
}
