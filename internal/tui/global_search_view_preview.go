package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPreview renders the preview pane for the selected task.
func (m *GlobalSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected.")
	}

	result := m.filtered[m.cursor].result
	if result.IsNote() {
		return m.renderNotePreview(result)
	}

	return m.renderTaskPreview(m.filtered[m.cursor])
}

func (m *GlobalSearchModel) renderNotePreview(result models.SearchResult) string {
	var b strings.Builder
	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(result.Project.Name))
	b.WriteString("\n\n")
	noteHeader := "Note: " + truncateToVisualWidth(result.Note.Title, max(m.previewWidth-8, 1))
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(noteHeader))
	b.WriteString("\n\n")
	if len(result.Note.Tags) > 0 {
		fmt.Fprintf(&b, "%s %s\n", MutedStyle.Render("Tags:"), strings.Join(result.Note.Tags, ", "))
	}
	if len(result.Note.LinkedTasks) > 0 {
		fmt.Fprintf(&b, "%s %d\n", MutedStyle.Render("Linked tasks:"), len(result.Note.LinkedTasks))
	}
	b.WriteString("\n")
	body := result.Note.Body
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	b.WriteString(body)
	return b.String()
}

func (m *GlobalSearchModel) renderTaskPreview(filteredResult filteredSearchResult) string {
	task := filteredResult.result.Task
	project := filteredResult.result.Project
	var b strings.Builder

	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(project.Name))
	b.WriteString("\n\n")

	typeStyle := GetTypeStyle(string(task.Type))
	titlePrefix := fmt.Sprintf("%s #%d: ", task.Type.HeadingPrefix(), task.ShortID)
	maxTitleLen := max(m.previewWidth-lipgloss.Width(titlePrefix)-4, 10)
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		GetTitlePriorityStyle(task.Priority).Render(truncateToVisualWidth(task.Title, maxTitleLen)))
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(header))
	b.WriteString("\n\n")

	m.renderTaskPreviewMetadata(&b, task, typeStyle)
	m.renderTaskPreviewDescription(&b, task)
	m.renderMatchedComment(&b, task, filteredResult)
	m.renderChildrenSummary(&b, task)

	return b.String()
}

func (m *GlobalSearchModel) renderTaskPreviewMetadata(b *strings.Builder, task *models.Task, typeStyle lipgloss.Style) {
	statusStyle := GetStatusStyle(string(task.Status))
	fmt.Fprintf(b, "%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status)))

	priorityStyle := NormalStyle
	if task.Priority <= 2 {
		priorityStyle = WarningIconStyle
	}
	fmt.Fprintf(b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority)))

	fmt.Fprintf(b, "%s %s\n",
		MutedStyle.Render("Type:"),
		typeStyle.Render(task.Type.HeadingPrefix()))

	if len(task.Tags) > 0 {
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = TagInlineStyle.Render(tag)
		}
		fmt.Fprintf(b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", "))
	}

	if task.DueDate != nil {
		dateStr := task.DueDate.Format(util.DateFormat)
		dateStr = AppendTimeIfHasTime(task, dateStr, "15:04")
		fmt.Fprintf(b, "%s %s\n",
			MutedStyle.Render("Due:"),
			dateStr)
	}

	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		fmt.Fprintf(b, "%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(truncateToVisualWidth(parentRef, m.previewWidth-10)))
	}
}

func (m *GlobalSearchModel) renderTaskPreviewDescription(b *strings.Builder, task *models.Task) {
	if task.Description == "" {
		return
	}

	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Description"))
	b.WriteString("\n")

	rendered := renderMarkdown(task.Description, m.previewWidth-2)
	maxLines := max(m.Height()-20, 3)
	lines := strings.Split(rendered, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "...")
	}
	b.WriteString(strings.Join(lines, "\n"))
}

func (m *GlobalSearchModel) renderMatchedComment(b *strings.Builder, task *models.Task, filteredResult filteredSearchResult) {
	if filteredResult.matchSource != "comment" || filteredResult.matchedCommentIdx < 0 || filteredResult.matchedCommentIdx >= len(task.Comments) {
		return
	}

	comment := task.Comments[filteredResult.matchedCommentIdx]
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Matched Comment"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(comment.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")
	text := comment.Text
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	b.WriteString(renderMarkdown(text, m.previewWidth-2))
	b.WriteString("\n")
}

func (m *GlobalSearchModel) renderChildrenSummary(b *strings.Builder, task *models.Task) {
	if len(task.Children) == 0 {
		return
	}

	b.WriteString("\n\n")
	completed := 0
	for _, child := range task.Children {
		if child.IsComplete() {
			completed++
		}
	}
	fmt.Fprintf(b, "%s %d/%d completed\n",
		MutedStyle.Render("Subtasks:"),
		completed, len(task.Children))
}