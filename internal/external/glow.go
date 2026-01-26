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

	b.WriteString(fmt.Sprintf("| Created | %s |\n", task.CreatedAt.Format(util.DateTimeFormat)))

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

		// Progress line (exclude notes)
		if task.HasProgressChildren() {
			total := 0
			completed := 0
			for _, child := range task.Children {
				if child.IsNote() {
					continue
				}
				total++
				if child.IsComplete() {
					completed++
				}
			}
			b.WriteString(fmt.Sprintf("Progress: %d/%d completed (%.0f%%)\n\n",
				completed, total, task.GetProgress()))
		}

		// List all children (including notes)
		for _, child := range task.Children {
			checkbox := "[ ]"
			if child.IsComplete() {
				checkbox = "[x]"
			} else if child.IsNote() {
				checkbox = "[·]"
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
