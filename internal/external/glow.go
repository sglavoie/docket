package external

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const defaultGlowTimeoutSeconds = 30

// glowTimeoutSeconds is the timeout for non-interactive glow invocations
// (RenderMarkdown, RenderFile). Tests may override this; production code
// should treat it as constant. Interactive flows (PageMarkdown) deliberately
// ignore this — see PageMarkdown for rationale.
var glowTimeoutSeconds = defaultGlowTimeoutSeconds

func glowTimeout() time.Duration {
	return time.Duration(glowTimeoutSeconds) * time.Second
}

// GlowAvailable checks if glow is installed and available.
func GlowAvailable() bool {
	_, err := CachedGlowPath()
	return err == nil
}

// RenderMarkdown renders markdown content using glow.
// Falls back to plain text output if glow fails or is unavailable.
func RenderMarkdown(content string) error {
	glowPath, err := CachedGlowPath()
	if err != nil {
		logging.Warn("glow not available — showing plain text")
		fmt.Println("(glow not available — showing plain text)\n\n" + content)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), glowTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, glowPath, "-")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logging.Warn("glow timed out after %ds — falling back to plain text", glowTimeoutSeconds)
		} else {
			logging.Warn("glow render failed, falling back to plain text: %v", err)
		}
		fmt.Println(content)
	}
	return nil
}

// RenderFile renders a markdown file using glow.
// Falls back to printing the file content if glow fails or is unavailable.
func RenderFile(path string) error {
	glowPath, err := CachedGlowPath()
	if err != nil {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		logging.Warn("glow not available — showing plain text")
		fmt.Println("(glow not available — showing plain text)\n\n" + string(content))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), glowTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, glowPath, "--", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logging.Warn("glow timed out after %ds for %s — falling back to plain text", glowTimeoutSeconds, path)
		} else {
			logging.Warn("glow render failed for %s, falling back to plain text: %v", path, err)
		}
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

func markdownTableCell(value any) string {
	s := fmt.Sprint(value)
	s = strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
	return strings.ReplaceAll(s, "|", `\|`)
}

// FormatTaskAsMarkdown formats a task as markdown.
func FormatTaskAsMarkdown(task *models.Task) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "# Task #%d: %s\n\n", task.ShortID, task.Title)

	// Metadata table
	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	state := task.State()
	fmt.Fprintf(&b, "| State | %s %s |\n", state.Icon(), state)
	fmt.Fprintf(&b, "| Priority | %d |\n", task.Priority)
	if len(task.Tags) > 0 {
		fmt.Fprintf(&b, "| Tags | %s |\n", markdownTableCell(strings.Join(task.Tags, ", ")))
	}

	if !task.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "| Created | %s |\n", task.CreatedAt.Format(util.DateTimeFormat))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		fmt.Fprintf(&b, "| Updated | %s |\n", task.UpdatedAt.Format(util.DateTimeFormat))
	}

	if task.CompletedAt != nil {
		fmt.Fprintf(&b, "| Completed | %s |\n", task.CompletedAt.Format(util.DateTimeFormat))
	}

	if task.DueDate != nil {
		fmt.Fprintf(&b, "| Due | %s |\n", task.DueDate.Format(util.DateFormat))
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
				fmt.Fprintf(&b, "%d. [URL] %s\n", i+1, att.Filename)
			} else {
				fmt.Fprintf(&b, "%d. [File] %s\n", i+1, att.Filename)
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
			fmt.Fprintf(&b, "Progress: %d/%d completed (%.0f%%)\n\n",
				completed, total, task.GetProgress())
		}

		// List all children
		for _, child := range task.Children {
			checkbox := "[ ]"
			if child.IsComplete() {
				checkbox = "[x]"
			}
			fmt.Fprintf(&b, "- %s #%d: %s\n",
				checkbox, child.ShortID, child.Title)
		}
	}

	return b.String()
}

// PageMarkdown renders markdown content using glow with pager.
// Falls back to plain text output if glow fails or is unavailable.
// Unlike RenderMarkdown, this is interactive — the pager waits for user input
// so we deliberately use no deadline; the user controls how long to read.
func PageMarkdown(content string) error {
	glowPath, err := CachedGlowPath()
	if err != nil {
		logging.Warn("glow not available — showing plain text")
		fmt.Println("(glow not available — showing plain text)\n\n" + content)
		return nil
	}

	// Write content to a temp file rather than piping via stdin. With `-p`
	// glow spawns a pager (typically `less`) that needs a real TTY on stdin
	// to read keypresses; piping content through stdin would feed that pipe
	// to the pager, leaving it with EOF and no way to scroll/quit.
	tmpPath, tmpErr := CreateTempFileWithContent(content, "tt-page-", ".md")
	if tmpErr != nil {
		logging.Warn("failed to stage glow pager input, falling back to plain text: %v", tmpErr)
		fmt.Println(content)
		return nil
	}
	defer func() { _ = os.Remove(tmpPath) }()

	cmd := exec.Command(glowPath, "-p", "--", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logging.Warn("glow pager failed, falling back to plain text: %v", err)
		fmt.Println(content)
	}
	return nil
}
