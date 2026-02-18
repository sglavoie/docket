package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// inboxBoxStyle returns a compact box style with no vertical padding.
func inboxBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2)
}

// inboxSelectedBoxStyle returns a compact selected box style with no vertical padding.
func inboxSelectedBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 2)
}

// inboxInnerWidth returns the available content width inside a box (border + padding).
func inboxInnerWidth(width int) int {
	// 2 border chars + 4 padding chars (2 each side)
	w := width - 6
	if w < 20 {
		w = 20
	}
	return w
}

// renderSettingLine renders a single label: value line with right-aligned label.
func renderSettingLine(label, value string) string {
	padded := fmt.Sprintf("%*s", FormLabelWidth, label)
	return MutedStyle.Render(padded+":") + " " + value
}

// inboxSnapshot stores task state for undo in inbox processors.
type inboxSnapshot struct {
	section   string
	priority  int
	tags      []string
	dueDate   *time.Time
	dependsOn []string
}

// captureInboxSnapshot creates a snapshot of task state for undo.
func captureInboxSnapshot(task *models.Task) *inboxSnapshot {
	var dueDate *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueDate = &d
	}

	return &inboxSnapshot{
		section:   task.Section,
		priority:  task.Priority,
		tags:      append([]string{}, task.Tags...),
		dueDate:   dueDate,
		dependsOn: append([]string{}, task.DependsOn...),
	}
}

// restoreInboxSnapshot restores a task from a snapshot.
func restoreInboxSnapshot(task *models.Task, snap *inboxSnapshot) {
	task.Section = snap.section
	task.Priority = snap.priority
	task.Tags = append([]string{}, snap.tags...)
	task.DependsOn = append([]string{}, snap.dependsOn...)
	if snap.dueDate != nil {
		d := *snap.dueDate
		task.DueDate = &d
	} else {
		task.DueDate = nil
	}
	task.UpdatedAt = time.Now()
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func willLeaveInbox(task *models.Task) bool {
	return task.Section != models.SectionInbox || task.IsComplete()
}
