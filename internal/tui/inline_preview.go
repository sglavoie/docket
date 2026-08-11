package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	inlinePeekMaxLines        = 5
	inlinePeekDescriptionRows = 3
)

type inlinePeekOptions struct {
	Task   *models.Task
	Indent string
	Width  int
}

func (m *OutlinerModel) inlinePeekLinesForRow(row outlinerRow, indent string, width int) []string {
	if m == nil || !row.isTaskRow() {
		return nil
	}
	return inlinePeekLines(inlinePeekOptions{
		Task:   row.task,
		Indent: indent,
		Width:  width,
	})
}

func (m *OutlinerModel) inlinePeekLineCount(row outlinerRow, width int) int {
	return len(m.inlinePeekLinesForRow(row, "", max(width, 1)))
}

// inlinePeekLines renders the additive, cursor-following task preview. The
// selected row already owns task identity and operational metadata, so Peek
// contains only richer content plus tags: description, dates, attachment names,
// comments, and tags.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	lines := make([]string, 0, inlinePeekMaxLines)
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

	if dates := inlinePeekDatesLine(opts.Task, time.Now(), available); dates != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(dates))
	}

	if attachments := inlinePeekAttachmentsLine(opts.Task, available); attachments != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(attachments))
	}

	if comments := inlinePeekCommentsLine(opts.Task, available); comments != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(comments))
	}

	if tags := inlinePeekTagsLine(opts.Task, available); tags != "" {
		lines = append(lines, opts.Indent+tags)
	}

	if len(lines) == 0 {
		lines = append(lines, opts.Indent+MutedStyle.Render("No additional details"))
	}
	if len(lines) > inlinePeekMaxLines {
		lines = lines[:inlinePeekMaxLines]
	}
	return lines
}

// inlinePeekDatesLine names the task clock the row has no width for: how long
// ago it was created and last touched, when it is due, and whether it recently
// came back from a snooze.
func inlinePeekDatesLine(task *models.Task, now time.Time, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	parts := make([]string, 0, 4)
	if age := formatRecencyAge(task.CreatedAt, now); age != "" {
		parts = append(parts, "created "+age)
	}
	if age := formatRecencyAge(task.UpdatedAt, now); age != "" {
		parts = append(parts, "updated "+age)
	}
	if task.DueDate != nil {
		parts = append(parts, "due "+task.DueDate.Format("2006-01-02"))
	}
	if taskWokeRecently(task, now) {
		parts = append(parts, "woke "+formatRecencyAge(*task.SnoozedUntil, now))
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateIntentional(strings.Join(parts, " · "), width, "…")
}

func inlinePeekAttachmentsLine(task *models.Task, width int) string {
	if task == nil || len(task.Attachments) == 0 || width <= 0 {
		return ""
	}

	const maxAttachments = 2
	parts := make([]string, 0, min(len(task.Attachments), maxAttachments+1))
	for i, attachment := range task.Attachments {
		if i >= maxAttachments {
			parts = append(parts, fmt.Sprintf("+%d more", len(task.Attachments)-maxAttachments))
			break
		}
		if name := strings.TrimSpace(attachmentDisplayName(attachment)); name != "" {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateIntentional("Attachments: "+strings.Join(parts, " · "), width, "…")
}

func inlinePeekCommentsLine(task *models.Task, width int) string {
	if task == nil || len(task.Comments) == 0 || width <= 0 {
		return ""
	}
	return truncateIntentional(fmt.Sprintf("Comments: %d", len(task.Comments)), width, "…")
}

func inlinePeekTagsLine(task *models.Task, width int) string {
	if task == nil || len(task.Tags) == 0 || width <= 0 {
		return ""
	}

	tags := make([]string, 0, len(task.Tags))
	for _, tag := range task.Tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, "#"+tag)
		}
	}
	if len(tags) == 0 {
		return ""
	}
	line := MutedStyle.Render("Tags: ") +
		TagInlineStyle.Bold(true).Render(strings.Join(tags, " "))
	return truncateIntentional(line, width, "…")
}

func inlinePeekDescriptionLines(description, indent string, width int) []string {
	description = strings.TrimSpace(strings.ReplaceAll(description, "\r\n", "\n"))
	if description == "" || width <= 0 {
		return nil
	}

	label := "Description: "
	bodyWidth := max(width-lipgloss.Width(label), 1)
	wrapped := wrapText(description, bodyWidth)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekDescriptionRows {
		wrapped = wrapped[:inlinePeekDescriptionRows]
		wrapped[len(wrapped)-1] = truncateIntentional(wrapped[len(wrapped)-1]+"…", bodyWidth, "…")
	}

	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		prefix := strings.Repeat(" ", lipgloss.Width(label))
		if i == 0 {
			prefix = label
		}
		lines = append(lines, indent+MutedStyle.Render(
			truncateIntentional(prefix+line, width, "…"),
		))
	}
	return lines
}
