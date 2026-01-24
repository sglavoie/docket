package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderHeader renders the header with progress information.
func (m *InboxProcessorModel) renderHeader() string {
	processed, skipped, remaining, total := m.getProgressStats()
	title := TitleStyle.Render("Inbox Processing")
	progress := MutedStyle.Render(fmt.Sprintf(
		"Processing %d/%d tasks (%d processed, %d skipped, %d remaining)",
		m.currentIndex+1, total, processed, skipped, remaining,
	))
	return fmt.Sprintf("%s\n%s", title, progress)
}

// renderTaskDisplay renders the current task details.
func (m *InboxProcessorModel) renderTaskDisplay(task *models.Task) string {
	var b strings.Builder

	// Task title and metadata
	titleLine := fmt.Sprintf("#%d: %s", task.ShortID, task.Title)
	b.WriteString(BoldStyle.Render(titleLine))
	b.WriteString("\n")

	// Type, status, created date, priority
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	metadata := fmt.Sprintf("%s %s · Created %s · Priority %s",
		typeIcon, statusIcon,
		MutedStyle.Render(task.CreatedAt.Format("Jan 2")),
		MutedStyle.Render(renderPriorityStars(task.Priority)),
	)
	b.WriteString(metadata)

	// Show triaged indicator if task will leave inbox after save
	if m.willLeaveInbox(task) {
		b.WriteString(" · ")
		b.WriteString(TriagedBadgeStyle.Render("↗ Triaged"))
	}
	b.WriteString("\n\n")

	// Description if present
	if task.Description != "" {
		desc := util.Truncate(task.Description, 300)
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	// Current settings
	b.WriteString(m.renderCurrentSettings(task))

	return b.String()
}

// renderCurrentSettings renders the current task settings.
func (m *InboxProcessorModel) renderCurrentSettings(task *models.Task) string {
	var parts []string

	section := task.Section
	if section == "" {
		section = "(no section)"
	}
	parts = append(parts, fmt.Sprintf("Section: %s", section))
	parts = append(parts, fmt.Sprintf("Priority: %d", task.Priority))

	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("Tags: %s", strings.Join(task.Tags, ", ")))
	} else {
		parts = append(parts, "Tags: none")
	}

	if task.DueDate != nil {
		parts = append(parts, fmt.Sprintf("Due: %s", task.DueDate.Format("Jan 2")))
	}

	if len(task.DependsOn) > 0 {
		parts = append(parts, fmt.Sprintf("Dependencies: %d", len(task.DependsOn)))
	}

	return MutedStyle.Render(strings.Join(parts, " · "))
}

// renderOperationsPanel renders the available operations.
func (m *InboxProcessorModel) renderOperationsPanel() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Operations"))
	b.WriteString("\n")

	// Group operations into 3 lines for better readability
	line1 := []string{"s: Section", "1-5: Priority", "t: Manage tags"}
	line2 := []string{"c: Create subtask", "e: Edit", "D: Due date"}
	line3 := []string{"w: Waiting", "b: Dependencies", "x: Delete", "u: Undo"}

	b.WriteString(MutedStyle.Render(strings.Join(line1, " · ")))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(strings.Join(line2, " · ")))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(strings.Join(line3, " · ")))

	return b.String()
}

// renderFooter renders the footer with navigation controls.
func (m *InboxProcessorModel) renderFooter() string {
	controls := []string{
		"n: Skip",
		"j/k: Navigate",
		"gg/G: First/last",
		",s/d/b/x/o: Status",
		"q: Quit",
	}
	return MutedStyle.Render(strings.Join(controls, " · "))
}

// renderEmptyState renders the state when current task is nil.
func (m *InboxProcessorModel) renderEmptyState() string {
	return SuccessStyle.Render("Inbox is empty! All tasks have been triaged.")
}

// renderEmptyInbox renders the view when no inbox tasks exist.
func (m *InboxProcessorModel) renderEmptyInbox() string {
	var b strings.Builder
	b.WriteString(TitleStyle.Render("Inbox Processing"))
	b.WriteString("\n\n")
	b.WriteString(SuccessStyle.Render("Inbox is empty! All tasks have been triaged."))
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Press 'q' to return to project tree"))
	return b.String()
}

// renderPriorityStars renders priority as stars.
func renderPriorityStars(priority int) string {
	if priority <= 0 {
		return ""
	}
	return strings.Repeat("★", priority)
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
// Returns empty string if no prefix is pending.
func (m *InboxProcessorModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"g": NavigationPrefixHints,
	}, m.width)
}
