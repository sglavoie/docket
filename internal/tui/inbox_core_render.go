package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (c *inboxCore) renderHeader() string {
	processed, skipped, _, total := c.getProgressStats()
	subtitle := fmt.Sprintf("%d/%d%s · %d done · %d skipped",
		c.currentIndex+1, total, c.backend.headerSubtitleExtra(), processed, skipped)
	title := renderFullscreenTitle(c.backend.headerTitle(), subtitle, c.AccentColor())

	var percent float64
	if total > 0 {
		percent = float64(c.currentIndex+1) / float64(total) * 100
	}
	barWidth := 30
	bar := RenderProgressBar(percent, barWidth)
	progressBar := fmt.Sprintf("%s %.0f%%", bar, percent)

	return fmt.Sprintf("%s\n%s", title, progressBar)
}

func (c *inboxCore) renderTaskDisplay() string {
	task, project := c.currentTaskRef()
	projectName := ""
	if c.backend.showProjectInHeader() {
		projectName = project.Name
	}
	return renderInboxTaskDisplay(task, c.width, willLeaveInbox(task), projectName, c.config)
}

func (c *inboxCore) renderEmptyState() string {
	content := SuccessStyle.Render(c.backend.emptyStateText())
	return BoxStyle.Render(content)
}

func (c *inboxCore) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle(c.backend.headerTitle(), "", c.AccentColor()))
	b.WriteString("\n\n")
	content := SuccessStyle.Render(c.backend.emptyInboxText())
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("esc") + MutedStyle.Render(": "+c.backend.returnHint()))

	output := b.String()
	if outputHeight := lipgloss.Height(output); outputHeight < c.height {
		output += strings.Repeat("\n", c.height-outputHeight)
	}
	return output
}
