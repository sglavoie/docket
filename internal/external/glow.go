package external

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// GlowAvailable checks if glow is installed and available.
func GlowAvailable() bool {
	_, err := exec.LookPath("glow")
	return err == nil
}

// RenderMarkdown renders markdown content using glow.
func RenderMarkdown(content string) error {
	cmd := exec.Command("glow", "-")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RenderFile renders a markdown file using glow.
func RenderFile(path string) error {
	cmd := exec.Command("glow", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

	b.WriteString(fmt.Sprintf("| Created | %s |\n", task.CreatedAt.Format("2006-01-02 15:04")))

	if task.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("| Completed | %s |\n", task.CompletedAt.Format("2006-01-02 15:04")))
	}

	if task.DueDate != nil {
		b.WriteString(fmt.Sprintf("| Due | %s |\n", task.DueDate.Format("2006-01-02")))
	}

	b.WriteString("\n")

	// Description
	if task.Description != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(task.Description)
		b.WriteString("\n\n")
	}

	// Children summary
	if len(task.Children) > 0 {
		b.WriteString("## Subtasks\n\n")
		completed := 0
		for _, child := range task.Children {
			if child.IsComplete() {
				completed++
			}
		}
		b.WriteString(fmt.Sprintf("Progress: %d/%d completed (%.0f%%)\n\n",
			completed, len(task.Children), task.GetProgress()))

		// List children
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
func PageMarkdown(content string) error {
	cmd := exec.Command("glow", "-p", "-")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
