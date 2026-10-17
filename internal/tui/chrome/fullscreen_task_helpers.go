package chrome

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// RenderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
// Pass an optional accent color to override the title foreground color.
func RenderFullscreenTitle(title, subtitle string, accent ...color.Color) string {
	return renderFullscreenTitleContent(FullscreenTitleStyle(accent...).Render(title), subtitle)
}

// RenderFullscreenTitleWithPulse renders the shared one-row full-screen title
// with the workspace-wide pulse right-aligned. Transient presentations use
// RenderFullscreenTitle instead and therefore never duplicate the host pulse.
func RenderFullscreenTitleWithPulse(title, subtitle string, workspacePulse pulse.WorkspacePulse, width int, accent ...color.Color) string {
	return renderFullscreenTitleContentWithPulse(
		FullscreenTitleStyle(accent...).Render(title),
		subtitle,
		workspacePulse,
		width,
	)
}

func FullscreenTitleStyle(accent ...color.Color) lipgloss.Style {
	if len(accent) > 0 && accent[0] != nil {
		return lipgloss.NewStyle().Bold(true).Foreground(accent[0])
	}
	return theme.TitleStyle
}

func renderFullscreenTitleContent(titleContent, subtitle string) string {
	t := titleContent
	if subtitle != "" {
		t += "  " + theme.MutedStyle.Render(subtitle)
	}
	return t
}

func renderFullscreenTitleContentWithPulse(titleContent, subtitle string, workspacePulse pulse.WorkspacePulse, width int) string {
	if width <= 0 {
		return renderFullscreenTitleContent(titleContent, subtitle)
	}
	context := ""
	if subtitle != "" {
		context = theme.MutedStyle.Render(subtitle)
	}
	return pulse.RenderOperationalTitle(pulse.OperationalTitleSpec{
		Surface: titleContent,
		Context: []string{context},
		Pulse:   &workspacePulse,
	}, width)
}

func RenderTaskScopedFullscreenTitleWithPulse(prefix string, task *models.Task, subtitle string, workspacePulse pulse.WorkspacePulse, width int, accent ...color.Color) string {
	titleStyle := FullscreenTitleStyle(accent...)
	if task == nil {
		return renderFullscreenTitleContentWithPulse(titleStyle.Render(prefix), subtitle, workspacePulse, width)
	}
	titleContent := titleStyle.Render(prefix) + textfit.RenderTaskTitle(task.Title, nil, theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{}))
	return renderFullscreenTitleContentWithPulse(titleContent, subtitle, workspacePulse, width)
}

// RenderPriorityBadge renders a priority indicator with color styling. An unset
// priority has no glyph of its own, so it renders nothing rather than borrowing
// the normal-priority icon.
func RenderPriorityBadge(priority int) string {
	if models.ValidatePriority(priority, false) != nil {
		return ""
	}
	return theme.GetPriorityStyle(priority).Render(theme.PriorityIcon(priority))
}
