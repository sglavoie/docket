package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// renderHeader renders the header with progress information.
func (m *InboxProcessorModel) renderHeader() string {
	processed, skipped, remaining, total := m.getProgressStats()
	subtitle := fmt.Sprintf("%d/%d tasks (%d processed, %d skipped, %d remaining)",
		m.currentIndex+1, total, processed, skipped, remaining)
	title := renderFullscreenTitle("Inbox Processing", subtitle)

	// Progress bar based on current position
	var percent float64
	if total > 0 {
		percent = float64(m.currentIndex+1) / float64(total) * 100
	}
	barWidth := 30
	bar := RenderProgressBar(percent, barWidth)
	progressBar := fmt.Sprintf("%s %.0f%%", bar, percent)

	return fmt.Sprintf("%s\n%s", title, progressBar)
}

// renderTaskDisplay renders the current task details.
func (m *InboxProcessorModel) renderTaskDisplay(task *models.Task) string {
	return renderInboxTaskDisplay(task, m.width, m.willLeaveInbox(task), "", m.config)
}

// renderCurrentSettings renders the current task settings as a vertical label-value layout.
func (m *InboxProcessorModel) renderCurrentSettings(task *models.Task) string {
	return renderInboxCurrentSettings(task, m.config)
}

// renderOperationsPanel renders the available operations.
func (m *InboxProcessorModel) renderOperationsPanel() string {
	return renderInboxOperationsPanel()
}

// renderFooter renders the footer with navigation controls.
func (m *InboxProcessorModel) renderFooter() string {
	return renderInboxFooter(m.width)
}

// renderEmptyState renders the state when current task is nil.
func (m *InboxProcessorModel) renderEmptyState() string {
	content := SuccessStyle.Render("✓ Inbox is empty! All tasks have been triaged.")
	return BoxStyle.Render(content)
}

// renderEmptyInbox renders the view when no inbox tasks exist.
func (m *InboxProcessorModel) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Inbox Processing", ""))
	b.WriteString("\n\n")
	content := SuccessStyle.Render("✓ Inbox is empty! All tasks have been triaged.")
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("q") + MutedStyle.Render(": return to project tree"))
	return b.String()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *InboxProcessorModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",": StatusPrefixHints,
			"g": NavigationPrefixHints,
		},
	}
}
