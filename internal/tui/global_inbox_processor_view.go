package tui

import (
	"fmt"
	"strings"
)

// renderHeader renders the header with progress information.
func (m *GlobalInboxProcessorModel) renderHeader() string {
	processed, skipped, remaining, total := m.getProgressStats()
	subtitle := fmt.Sprintf("%d/%d tasks from %d projects (%d processed, %d skipped, %d remaining)",
		m.currentIndex+1, total, len(m.projectCache), processed, skipped, remaining)
	title := renderFullscreenTitle("Global Inbox Processing", subtitle)

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
func (m *GlobalInboxProcessorModel) renderTaskDisplay(git *GlobalInboxTask) string {
	return renderInboxTaskDisplay(git.Task, m.width, m.willLeaveInbox(git.Task), git.Project.Name, m.config)
}

// renderCurrentSettings renders the current task settings as a vertical label-value layout.
func (m *GlobalInboxProcessorModel) renderCurrentSettings(git *GlobalInboxTask) string {
	return renderInboxCurrentSettings(git.Task, m.config)
}

// renderOperationsPanel renders the available operations.
func (m *GlobalInboxProcessorModel) renderOperationsPanel() string {
	return renderInboxOperationsPanel()
}

// renderFooter renders the footer with navigation controls.
func (m *GlobalInboxProcessorModel) renderFooter() string {
	return renderInboxFooter(m.width)
}

// renderEmptyState renders the state when current task is nil.
func (m *GlobalInboxProcessorModel) renderEmptyState() string {
	content := SuccessStyle.Render("✓ All inboxes are empty! All tasks have been triaged.")
	return BoxStyle.Render(content)
}

// renderEmptyInbox renders the view when no inbox tasks exist.
func (m *GlobalInboxProcessorModel) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Global Inbox Processing", ""))
	b.WriteString("\n\n")
	content := SuccessStyle.Render("✓ All inboxes are empty! No tasks to process across any project.")
	b.WriteString(BoxStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(HelpKeyStyle.Render("q") + MutedStyle.Render(": return to dashboard"))
	return b.String()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *GlobalInboxProcessorModel) GetPrefixPopupInfo() *PrefixPopupInfo {
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
