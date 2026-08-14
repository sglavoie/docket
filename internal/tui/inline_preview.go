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
	inlinePeekTitleRows       = 3
)

type inlinePeekOptions struct {
	Task          *models.Task
	Indent        string
	Width         int
	TitleOverflow string
}

func (m *OutlinerModel) inlinePeekLinesForRow(row outlinerRow, indent string, rowWidth int, colLayout *ColumnLayout) []string {
	if m == nil || !row.isTaskRow() {
		return nil
	}
	return inlinePeekLines(inlinePeekOptions{
		Task:          row.task,
		Indent:        indent,
		Width:         rowWidth,
		TitleOverflow: m.taskRowTitleOverflow(row, rowWidth, colLayout),
	})
}

// inlinePeekLineCount measures the Peek height computeOutlinerViewport must
// reserve. It reconstructs the row's real indent and substitutes the worst-case
// column layout, because the title continuation now sits outside the additive
// cap and a measurement that assumed neither would diverge without bound on a
// deep row.
func (m *OutlinerModel) inlinePeekLineCount(row outlinerRow, width int) int {
	width = max(width, 1)
	indent := strings.Repeat(" ", lipgloss.Width(row.prefix)+4)
	return len(m.inlinePeekLinesForRow(row, indent, width, worstCaseColumnLayout(width)))
}

// inlinePeekLines renders the additive, cursor-following task preview. The
// selected row already owns task identity and operational metadata, so Peek
// contains only richer content plus tags: description, dates, attachment names,
// comments, and tags. A title the row had to clip is completed above them, and
// those continuation rows raise the cap rather than displacing anything.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	titleLines := inlinePeekTitleLines(opts.Task, opts.TitleOverflow, opts.Indent, available)
	lines := make([]string, 0, inlinePeekMaxLines+len(titleLines))
	lines = append(lines, titleLines...)
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

	if dates := inlinePeekDatesLine(opts.Task, time.Now(), available); dates != "" {
		lines = append(lines, opts.Indent+dates)
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
	if maxLines := inlinePeekMaxLines + len(titleLines); len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// inlinePeekTitleLines completes a title the row clipped rather than restating
// it: the row keeps its ellipsis, which now reads as "continues below", and Peek
// carries only the remainder. No label and the task's own title styling, so it
// reads as the same title instead of as another Label: value line.
func inlinePeekTitleLines(task *models.Task, overflow, indent string, width int) []string {
	if task == nil || overflow == "" || width <= 0 {
		return nil
	}

	wrapped := wrapText(overflow, width)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekTitleRows {
		wrapped = wrapped[:inlinePeekTitleRows]
		wrapped[len(wrapped)-1] = truncateIntentional(wrapped[len(wrapped)-1]+"…", width, "…")
	}

	style := TaskTitleStyleFor(task, TaskTitleStyleOptions{})
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, indent+style.Render(truncateIntentional(line, width, "…")))
	}
	return lines
}

// inlinePeekDatesLine names the task clock the row has no width for: how long
// ago it was created and last touched, when it is due, and whether it recently
// came back from a deferral. Labels stay muted and values take the footer's key
// color, so the durations read as data rather than as prose.
func inlinePeekDatesLine(task *models.Task, now time.Time, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	parts := make([]string, 0, 4)
	field := func(label, value string) {
		parts = append(parts, HelpStyle.Render(label+" ")+HelpKeyStyle.Render(value))
	}
	if age := formatRecencyAge(task.CreatedAt, now); age != "" {
		field("created", age)
	}
	if age := formatRecencyAge(task.UpdatedAt, now); age != "" {
		field("updated", age)
	}
	if task.DueDate != nil {
		field("due", task.DueDate.Format("2006-01-02"))
	}
	if taskWokeRecently(task, now) {
		field("woke", formatRecencyAge(*task.DeferredUntil, now))
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateIntentional(strings.Join(parts, HelpStyle.Render(" · ")), width, "…")
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
