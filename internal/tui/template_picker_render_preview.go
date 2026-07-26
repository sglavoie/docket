package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/template"
)

// renderPreviewPanel renders the template preview for the right pane.
func (m *TemplatePickerModel) renderPreviewPanel(t *template.TaskTemplate, width, height int) string {
	if t == nil {
		// No template selected (Skip is highlighted)
		msg := "No template selected"
		return MutedStyle.Render(msg)
	}

	var lines []string

	// Template name as header
	lines = append(lines, BoldStyle.Render(t.Name))
	lines = append(lines, "")

	// Type badge
	typeBadge := fmt.Sprintf("[%s]", t.Type)
	lines = append(lines, typeBadge)

	// Priority
	if t.Priority > 0 {
		lines = append(lines, fmt.Sprintf("Priority: P%d", t.Priority))
	}

	// Tags
	if len(t.Tags) > 0 {
		tagStr := strings.Join(t.Tags, ", ")
		tagStr = truncateToVisualWidth(tagStr, max(width-8, 1))
		lines = append(lines, fmt.Sprintf("Tags: %s", tagStr))
	}

	// State
	lines = append(lines, fmt.Sprintf("State: %s", t.EffectiveState()))

	// Description
	if t.Description != "" {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Description:"))
		// Word-wrap description (width-4 accounts for indent and margins)
		wrapWidth := max(width-4, 20)
		wrapped := wrapTemplateText(t.Description, wrapWidth)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
	}

	// Checklist items
	if len(t.Checklist) > 0 {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Checklist:"))
		for i, item := range t.Checklist {
			itemText := item
			maxLen := width - 6
			if maxLen > 0 && len(itemText) > maxLen {
				itemText = itemText[:maxLen-1] + "\u2026"
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, itemText))
		}
	}

	// Subtasks (Children)
	if len(t.Children) > 0 {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Subtasks:"))
		for i, child := range t.Children {
			itemText := child.Title
			maxLen := width - 14
			if maxLen > 0 && len(itemText) > maxLen {
				itemText = itemText[:maxLen-1] + "\u2026"
			}
			childType := child.GetChildType()
			optional := ""
			if child.Optional {
				optional = " (optional)"
			}
			lines = append(lines, fmt.Sprintf("  %d. %s [%s]%s", i+1, itemText, childType, optional))
		}
	}

	// Variables
	if template.HasVariables(t) {
		vars := template.ExtractVariables(t)
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render("Variables:"))
		for _, v := range vars {
			entry := fmt.Sprintf("  {{%s}}", v.Name)
			if v.Default != "" {
				entry += MutedStyle.Render(fmt.Sprintf(" (default: %s)", v.Default))
			}
			lines = append(lines, entry)
		}
	}

	// Apply scroll offset
	if m.previewScroll > 0 && m.previewScroll < len(lines) {
		lines = lines[m.previewScroll:]
	} else if m.previewScroll >= len(lines) {
		// Clamp scroll to valid range
		m.previewScroll = max(0, len(lines)-1)
		if m.previewScroll > 0 {
			lines = lines[m.previewScroll:]
		}
	}

	// Limit to height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// wrapTemplateText wraps text to the specified width for template preview.
func wrapTemplateText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
