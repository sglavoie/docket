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
	fmt.Fprintf(&b, "# %s #%d: %s\n\n",
		task.Type.HeadingPrefix(), task.ShortID, task.Title)

	// Metadata table
	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	status := models.LegacyStatusForState(task.State())
	fmt.Fprintf(&b, "| Status | %s %s |\n", status.Icon(), status)
	fmt.Fprintf(&b, "| Priority | %d |\n", task.Priority)
	fmt.Fprintf(&b, "| Type | %s |\n", task.Type.HeadingPrefix())

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
		dueFmt := util.DateFormat
		if task.HasTime {
			dueFmt = util.DateTimeFormat
		}
		fmt.Fprintf(&b, "| Due | %s |\n", task.DueDate.Format(dueFmt))
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
			fmt.Fprintf(&b, "- %s %s #%d: %s\n",
				checkbox, child.Type.Icon(), child.ShortID, child.Title)
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

// formatTaskAttributesMarkdown formats type-specific attributes as a markdown table.
// Returns empty string if no attributes are set.
func formatTaskAttributesMarkdown(task *models.Task) string {
	var rows []string

	switch task.Type {
	case models.TaskTypeBug:
		if task.BugAttrs != nil {
			if task.BugAttrs.Severity != "" {
				rows = append(rows, fmt.Sprintf("| Severity | %s |", markdownTableCell(task.BugAttrs.Severity)))
			}
			if task.BugAttrs.Reproducible != "" {
				rows = append(rows, fmt.Sprintf("| Reproducible | %s |", markdownTableCell(task.BugAttrs.Reproducible)))
			}
			if task.BugAttrs.Workaround != "" {
				rows = append(rows, fmt.Sprintf("| Workaround | %s |", markdownTableCell(task.BugAttrs.Workaround)))
			}
		}
	case models.TaskTypeIdea:
		if task.IdeaAttrs != nil {
			if task.IdeaAttrs.Category != "" {
				rows = append(rows, fmt.Sprintf("| Category | %s |", markdownTableCell(task.IdeaAttrs.Category)))
			}
			if task.IdeaAttrs.Effort != "" {
				rows = append(rows, fmt.Sprintf("| Effort | %s |", markdownTableCell(task.IdeaAttrs.Effort)))
			}
			if task.IdeaAttrs.Value != "" {
				rows = append(rows, fmt.Sprintf("| Value | %s |", markdownTableCell(task.IdeaAttrs.Value)))
			}
			if task.IdeaAttrs.Validated {
				rows = append(rows, "| Validated | Yes |")
			}
		}
	case models.TaskTypeTask:
		if task.TaskAttrs != nil {
			if task.TaskAttrs.Effort != "" {
				rows = append(rows, fmt.Sprintf("| Effort | %s |", markdownTableCell(task.TaskAttrs.Effort)))
			}
			if task.TaskAttrs.WorkType != "" {
				rows = append(rows, fmt.Sprintf("| Work Type | %s |", markdownTableCell(task.TaskAttrs.WorkType)))
			}
		}
	case models.TaskTypeStory:
		if task.StoryAttrs != nil {
			if task.StoryAttrs.StoryPoints > 0 {
				rows = append(rows, fmt.Sprintf("| Story Points | %d |", task.StoryAttrs.StoryPoints))
			}
			if task.StoryAttrs.Persona != "" {
				rows = append(rows, fmt.Sprintf("| Persona | %s |", markdownTableCell(task.StoryAttrs.Persona)))
			}
		}
	case models.TaskTypeEpic:
		if task.EpicAttrs != nil {
			if task.EpicAttrs.Scope != "" {
				rows = append(rows, fmt.Sprintf("| Scope | %s |", markdownTableCell(task.EpicAttrs.Scope)))
			}
			if task.EpicAttrs.Milestone != "" {
				rows = append(rows, fmt.Sprintf("| Milestone | %s |", markdownTableCell(task.EpicAttrs.Milestone)))
			}
			if task.EpicAttrs.SuccessMetrics != "" {
				rows = append(rows, fmt.Sprintf("| Success Metrics | %s |", markdownTableCell(task.EpicAttrs.SuccessMetrics)))
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
