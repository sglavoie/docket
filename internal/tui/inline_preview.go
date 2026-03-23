package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// inlinePreviewLines returns the styled preview lines for a task (description + checklist).
// indentStr is prepended to each line. maxWidth is the max content width per line.
// Returns nil if the task has no description or checklist.
func inlinePreviewLines(task *models.Task, indentStr string, maxWidth int) []string {
	if task == nil {
		return nil
	}

	var lines []string
	availWidth := max(maxWidth-lipgloss.Width(indentStr), 10)

	// Description lines (up to 2)
	if task.Description != "" {
		descLines := strings.SplitN(strings.TrimSpace(task.Description), "\n", 3)
		for i, line := range descLines {
			if i >= 2 {
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			lines = append(lines, indentStr+MutedStyle.Render(truncateToVisualWidth(trimmed, availWidth)))
		}
	}

	// Checklist items (up to 2, then "+N more")
	if len(task.Checklist) > 0 {
		shown := 0
		for _, item := range task.Checklist {
			if shown >= 2 {
				remaining := len(task.Checklist) - shown
				lines = append(lines, indentStr+MutedStyle.Render(fmt.Sprintf("+%d more", remaining)))
				break
			}
			check := "[ ]"
			if item.Done {
				check = "[x]"
			}
			text := truncateToVisualWidth(item.Text, availWidth-4) // 4 = "[x] "
			lines = append(lines, indentStr+MutedStyle.Render(check+" "+text))
			shown++
		}
	}

	if len(lines) == 0 {
		lines = append(lines, indentStr+MutedStyle.Render("(no description)"))
	}

	return lines
}

// inlinePreviewLineCount returns how many preview lines a task would produce.
func inlinePreviewLineCount(task *models.Task) int {
	if task == nil {
		return 0
	}

	count := 0

	// Description lines (up to 2)
	if task.Description != "" {
		descLines := strings.SplitN(strings.TrimSpace(task.Description), "\n", 3)
		for i, line := range descLines {
			if i >= 2 {
				break
			}
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
	}

	// Checklist items (up to 2, plus "+N more" if needed)
	if len(task.Checklist) > 0 {
		if len(task.Checklist) <= 2 {
			count += len(task.Checklist)
		} else {
			count += 3 // 2 items + "+N more"
		}
	}

	if count == 0 {
		count = 1 // "(no description)"
	}

	return count
}
