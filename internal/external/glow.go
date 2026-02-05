package external

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// GlowAvailable checks if glow is installed and available.
func GlowAvailable() bool {
	_, err := exec.LookPath("glow")
	return err == nil
}

// RenderMarkdown renders markdown content using glow.
// Falls back to plain text output if glow fails or is unavailable.
func RenderMarkdown(content string) error {
	if !GlowAvailable() {
		fmt.Println(content)
		return nil
	}

	cmd := exec.Command("glow", "-")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Glow failed at runtime - fall back to plain text
		fmt.Println(content)
	}
	return nil
}

// RenderFile renders a markdown file using glow.
// Falls back to printing the file content if glow fails or is unavailable.
func RenderFile(path string) error {
	if !GlowAvailable() {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Println(string(content))
		return nil
	}

	cmd := exec.Command("glow", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Glow failed at runtime - fall back to plain text
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fmt.Println(string(content))
	}
	return nil
}

// RenderTaskMarkdown formats a task as markdown and renders it with glow.
func RenderTaskMarkdown(task *models.Task) error {
	content := FormatTaskAsMarkdown(task)
	return RenderMarkdown(content)
}

// FormatTaskAsMarkdown formats a task as markdown.
func FormatTaskAsMarkdown(task *models.Task) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# %s #%d: %s\n\n",
		task.Type.HeadingPrefix(), task.ShortID, task.Title))

	// Metadata table
	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString(fmt.Sprintf("| Status | %s %s |\n", task.Status.Icon(), task.Status))
	b.WriteString(fmt.Sprintf("| Priority | %d |\n", task.Priority))
	b.WriteString(fmt.Sprintf("| Type | %s |\n", task.Type.HeadingPrefix()))

	if len(task.Tags) > 0 {
		b.WriteString(fmt.Sprintf("| Tags | %s |\n", strings.Join(task.Tags, ", ")))
	}

	if !task.CreatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("| Created | %s |\n", task.CreatedAt.Format(util.DateTimeFormat)))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		b.WriteString(fmt.Sprintf("| Updated | %s |\n", task.UpdatedAt.Format(util.DateTimeFormat)))
	}

	if task.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("| Completed | %s |\n", task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		b.WriteString(fmt.Sprintf("| Due | %s |\n", task.DueDate.Format(util.DateFormat)))
	}

	b.WriteString("\n")

	// Type-specific attributes
	if attrTable := formatTaskAttributesMarkdown(task); attrTable != "" {
		b.WriteString("## Attributes\n\n")
		b.WriteString(attrTable)
		b.WriteString("\n")
	}

	// Description
	if task.Description != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(task.Description)
		b.WriteString("\n\n")
	}

	// Attachments
	if len(task.Attachments) > 0 {
		b.WriteString("## Attachments\n\n")
		for i, att := range task.Attachments {
			if att.IsURL {
				b.WriteString(fmt.Sprintf("%d. [URL] %s\n", i+1, att.Filename))
			} else {
				b.WriteString(fmt.Sprintf("%d. [File] %s\n", i+1, att.Filename))
			}
		}
		b.WriteString("\n")
	}

	// Children summary
	if len(task.Children) > 0 {
		b.WriteString("## Subtasks\n\n")

		// Progress line
		if task.HasProgressChildren() {
			total := 0
			completed := 0
			for _, child := range task.Children {
				total++
				if child.IsComplete() {
					completed++
				}
			}
			b.WriteString(fmt.Sprintf("Progress: %d/%d completed (%.0f%%)\n\n",
				completed, total, task.GetProgress()))
		}

		// List all children
		for _, child := range task.Children {
			checkbox := "[ ]"
			if child.IsComplete() {
				checkbox = "[x]"
			}
			b.WriteString(fmt.Sprintf("- %s %s #%d: %s\n",
				checkbox, child.Type.Icon(), child.ShortID, child.Title))
		}
	}

	return b.String()
}

// PageMarkdown renders markdown content using glow with pager.
// Falls back to plain text output if glow fails or is unavailable.
func PageMarkdown(content string) error {
	if !GlowAvailable() {
		fmt.Println(content)
		return nil
	}

	cmd := exec.Command("glow", "-p", "-")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Glow failed at runtime - fall back to plain text
		fmt.Println(content)
	}
	return nil
}

// formatTaskAttributesMarkdown formats type-specific attributes as a markdown table.
// Returns empty string if no attributes are set.
func formatTaskAttributesMarkdown(task *models.Task) string {
	var rows []string

	switch task.Type {
	case models.TaskTypeBug:
		if task.BugAttrs != nil {
			if task.BugAttrs.Severity != "" {
				rows = append(rows, fmt.Sprintf("| Severity | %s |", task.BugAttrs.Severity))
			}
			if task.BugAttrs.Reproducible != "" {
				rows = append(rows, fmt.Sprintf("| Reproducible | %s |", task.BugAttrs.Reproducible))
			}
			if task.BugAttrs.Workaround != "" {
				workaround := strings.ReplaceAll(task.BugAttrs.Workaround, "\n", " ")
				rows = append(rows, fmt.Sprintf("| Workaround | %s |", workaround))
			}
		}
	case models.TaskTypeIdea:
		if task.IdeaAttrs != nil {
			if task.IdeaAttrs.Category != "" {
				rows = append(rows, fmt.Sprintf("| Category | %s |", task.IdeaAttrs.Category))
			}
			if task.IdeaAttrs.Effort != "" {
				rows = append(rows, fmt.Sprintf("| Effort | %s |", task.IdeaAttrs.Effort))
			}
			if task.IdeaAttrs.Value != "" {
				rows = append(rows, fmt.Sprintf("| Value | %s |", task.IdeaAttrs.Value))
			}
			if task.IdeaAttrs.Validated {
				rows = append(rows, "| Validated | Yes |")
			}
		}
	case models.TaskTypeTask:
		if task.TaskAttrs != nil {
			if task.TaskAttrs.Effort != "" {
				rows = append(rows, fmt.Sprintf("| Effort | %s |", task.TaskAttrs.Effort))
			}
			if task.TaskAttrs.WorkType != "" {
				rows = append(rows, fmt.Sprintf("| Work Type | %s |", task.TaskAttrs.WorkType))
			}
		}
	case models.TaskTypeStory:
		if task.StoryAttrs != nil {
			if task.StoryAttrs.StoryPoints > 0 {
				rows = append(rows, fmt.Sprintf("| Story Points | %d |", task.StoryAttrs.StoryPoints))
			}
			if task.StoryAttrs.Persona != "" {
				persona := strings.ReplaceAll(task.StoryAttrs.Persona, "\n", " ")
				rows = append(rows, fmt.Sprintf("| Persona | %s |", persona))
			}
		}
	case models.TaskTypeEpic:
		if task.EpicAttrs != nil {
			if task.EpicAttrs.Scope != "" {
				rows = append(rows, fmt.Sprintf("| Scope | %s |", task.EpicAttrs.Scope))
			}
			if task.EpicAttrs.Milestone != "" {
				rows = append(rows, fmt.Sprintf("| Milestone | %s |", task.EpicAttrs.Milestone))
			}
			if task.EpicAttrs.SuccessMetrics != "" {
				metrics := strings.ReplaceAll(task.EpicAttrs.SuccessMetrics, "\n", " ")
				rows = append(rows, fmt.Sprintf("| Success Metrics | %s |", metrics))
			}
		}
	}

	if len(rows) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("| Attribute | Value |\n")
	b.WriteString("|-----------|-------|\n")
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}
