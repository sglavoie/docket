package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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
	Breadcrumb    string
	// Dependencies and Dependents are the two read-only reference details. They
	// arrive resolved because Peek is a pure renderer and neither direction can
	// be read off the task alone: an outgoing entry needs the target project to
	// name it, and an incoming one exists nowhere but in the other tasks.
	Dependencies []dependencyDetail
	Dependents   []dependencyDetail
	// DependencyCensus is the same reading the row's `~S/N` painted, carried in
	// so Peek's Dependencies label repeats those numbers rather than recounting
	// them against a population of its own.
	DependencyCensus dependencyCensus
}

func (m *OutlinerModel) inlinePeekLinesForRow(row outlinerRow, indent string, rowWidth int, colLayout *ColumnLayout) []string {
	if m == nil || !row.isTaskRow() {
		return nil
	}
	index := m.dependencyIndex()
	return inlinePeekLines(inlinePeekOptions{
		Task:             row.task,
		Indent:           indent,
		Width:            rowWidth,
		TitleOverflow:    m.taskRowTitleOverflow(row, rowWidth, colLayout),
		Breadcrumb:       row.Breadcrumb,
		Dependencies:     outgoingDependencyDetails(row.task, index),
		Dependents:       incomingDependencyDetails(m.rowIdentity(row), index),
		DependencyCensus: dependencyCensusFor(row.task, index),
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
// comments, and tags. A title the row had to clip is completed above them and a
// promoted root's ancestor chain follows it; both raise the cap rather than
// displacing anything.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	titleLines := inlinePeekTitleLines(opts.Task, opts.TitleOverflow, opts.Indent, available)
	parentLine := inlinePeekParentLine(opts.Breadcrumb, available)
	lines := make([]string, 0, inlinePeekMaxLines+len(titleLines)+1)
	lines = append(lines, titleLines...)
	if parentLine != "" {
		lines = append(lines, opts.Indent+parentLine)
	}
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

	if dates := inlinePeekDatesLine(opts.Task, time.Now(), available); dates != "" {
		lines = append(lines, opts.Indent+dates)
	}

	if attachments := inlinePeekAttachmentsLine(opts.Task, available); attachments != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(attachments))
	}

	if comments := inlinePeekCommentsLine(opts.Task, available); comments != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(comments))
	}

	// The two reference lines sit above tags because Peek is the only surface
	// that names a reference at all: the row carries `~S/N`, and there is no
	// panel behind it the way `;` and `&` stand behind comments and attachments.
	// Tags are already on the row, so they are the cheaper line to lose here.
	// Each label carries its own count — the outgoing one the row's very numbers
	// — so a reader sees the whole size of a set the entry cap may have elided.
	// No `?` follows the fraction here: every entry below names its own state in
	// full, so the row's shorthand for "something could not be read" would be
	// repeating what the list already says.
	dependencyLabel := "Dependencies " + opts.DependencyCensus.counts()
	if dependencies := inlinePeekReferenceLine(dependencyLabel, opts.Dependencies, available); dependencies != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(dependencies))
	}

	dependentLabel := fmt.Sprintf("Dependents %d", len(opts.Dependents))
	if dependents := inlinePeekReferenceLine(dependentLabel, opts.Dependents, available); dependents != "" {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render(dependents))
	}

	if tags := inlinePeekTagsLine(opts.Task, available); tags != "" {
		lines = append(lines, opts.Indent+tags)
	}

	if len(lines) == 0 {
		lines = append(lines, opts.Indent+theme.MutedStyle.Render("No additional details"))
	}
	maxLines := inlinePeekMaxLines + len(titleLines)
	if parentLine != "" {
		maxLines++
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// inlinePeekParentLine names the ancestor chain the row reduced to a bare `‹`.
// It raises the cap like the title continuation and for the same reason: the row
// deliberately elided it, and Peek is the surface that shows what the row had no
// width for. Root-first and uncompacted — Peek has the width the row did not.
func inlinePeekParentLine(breadcrumb string, width int) string {
	if width <= 0 {
		return ""
	}
	chain := strings.TrimSuffix(strings.TrimSpace(breadcrumb), " ›")
	if chain == "" {
		return ""
	}
	return textfit.TruncateIntentional(theme.MutedStyle.Render("Parent: ")+chain, width, "…")
}

// inlinePeekTitleLines completes a title the row clipped rather than restating
// it: the row keeps its ellipsis, which now reads as "continues below", and Peek
// carries only the remainder. No label and the task's own title styling, so it
// reads as the same title instead of as another Label: value line.
func inlinePeekTitleLines(task *models.Task, overflow, indent string, width int) []string {
	if task == nil || overflow == "" || width <= 0 {
		return nil
	}

	wrapped := textfit.WrapText(overflow, width)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekTitleRows {
		wrapped = wrapped[:inlinePeekTitleRows]
		wrapped[len(wrapped)-1] = textfit.TruncateIntentional(wrapped[len(wrapped)-1]+"…", width, "…")
	}

	style := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, indent+style.Render(textfit.TruncateIntentional(line, width, "…")))
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

	parts := make([]string, 0, 5)
	field := func(label, value string) {
		parts = append(parts, theme.HelpStyle.Render(label+" ")+theme.HelpKeyStyle.Render(value))
	}
	if age := formatRecencyAge(task.CreatedAt, now); age != "" {
		field("created", age)
	}
	if age := formatRecencyAge(task.UpdatedAt, now); age != "" {
		field("updated", age)
	}
	if task.DueDate != nil {
		field("due", dueGrammarValue(task))
	}
	if taskWokeRecently(task, now) {
		field("woke", formatRecencyAge(*task.DeferredUntil, now))
	}
	if taskHasRepeatedDeferrals(task) {
		field("deferred", fmt.Sprintf("×%d", task.DeferCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return textfit.TruncateIntentional(strings.Join(parts, theme.HelpStyle.Render(" · ")), width, "…")
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
	return textfit.TruncateIntentional("Attachments: "+strings.Join(parts, " · "), width, "…")
}

// inlinePeekReferenceLine renders one read-only reference section. It names the
// first few entries and counts the rest the way the attachments line does, so a
// task with many references costs one line rather than many. The list is
// read-only and non-navigable: there is no cursor in it and no key acts on an
// entry, because a reference is information about other work, not a way to
// reach it.
func inlinePeekReferenceLine(label string, details []dependencyDetail, width int) string {
	if len(details) == 0 || width <= 0 {
		return ""
	}
	const maxDetails = 2
	parts := make([]string, 0, min(len(details), maxDetails+1))
	for i, detail := range details {
		if i >= maxDetails {
			parts = append(parts, fmt.Sprintf("+%d more", len(details)-maxDetails))
			break
		}
		parts = append(parts, detail.String())
	}
	return textfit.TruncateIntentional(label+": "+strings.Join(parts, " · "), width, "…")
}

func inlinePeekCommentsLine(task *models.Task, width int) string {
	if task == nil || len(task.Comments) == 0 || width <= 0 {
		return ""
	}
	return textfit.TruncateIntentional(fmt.Sprintf("Comments: %d", len(task.Comments)), width, "…")
}

func inlinePeekTagsLine(task *models.Task, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	chips := make([]string, 0, len(task.Tags))
	for _, tag := range orderedTaskTags(task.Tags) {
		if tag = strings.TrimSpace(tag); tag != "" {
			chips = append(chips, theme.TagChipStyle(tag).Render("#"+tag))
		}
	}
	showFullContext := lipgloss.Width(task.Context) > 12
	if len(chips) == 0 && !showFullContext {
		return ""
	}
	parts := make([]string, 0, 2)
	if showFullContext {
		parts = append(parts, theme.TaskContextBadgeStyle(task).Render("["+task.Context+"]"))
	}
	if len(chips) > 0 {
		parts = append(parts, strings.Join(chips, " "))
	}
	line := theme.MutedStyle.Render("Tags: ") + strings.Join(parts, " ")
	return textfit.TruncateIntentional(line, width, "…")
}

func inlinePeekDescriptionLines(description, indent string, width int) []string {
	description = strings.TrimSpace(strings.ReplaceAll(description, "\r\n", "\n"))
	if description == "" || width <= 0 {
		return nil
	}

	label := "Description: "
	bodyWidth := max(width-lipgloss.Width(label), 1)
	wrapped := textfit.WrapText(description, bodyWidth)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekDescriptionRows {
		wrapped = wrapped[:inlinePeekDescriptionRows]
		wrapped[len(wrapped)-1] = textfit.TruncateIntentional(wrapped[len(wrapped)-1]+"…", bodyWidth, "…")
	}

	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		prefix := strings.Repeat(" ", lipgloss.Width(label))
		if i == 0 {
			prefix = label
		}
		lines = append(lines, indent+theme.MutedStyle.Render(
			textfit.TruncateIntentional(prefix+line, width, "…"),
		))
	}
	return lines
}
