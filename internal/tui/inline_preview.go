package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const (
	inlinePeekMaxLines        = 5
	inlinePeekDescriptionRows = 2
)

type inlinePeekOptions struct {
	Task     *models.Task
	Project  *models.Project
	Projects []*models.Project
	Indent   string
	Width    int
}

type taskRelationship struct {
	Task    *models.Task
	Project *models.Project
}

func (m *OutlinerModel) inlinePeekLinesForRow(row outlinerRow, indent string, width int) []string {
	if m == nil || !row.isTaskRow() {
		return nil
	}
	project := m.resolveProjectForRow(row)
	return inlinePeekLines(inlinePeekOptions{
		Task:     row.task,
		Project:  project,
		Projects: m.relationshipProjects(project),
		Indent:   indent,
		Width:    width,
	})
}

func (m *OutlinerModel) inlinePeekLineCount(row outlinerRow, width int) int {
	return len(m.inlinePeekLinesForRow(row, "", max(width, 1)))
}

func (m *OutlinerModel) relationshipProjects(owner *models.Project) []*models.Project {
	if m == nil {
		return nil
	}
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return uniqueRelationshipProjects(owner, m.loadedProjects)
}

// inlinePeekLines renders the additive, cursor-following task preview. The
// selected row already owns task identity and operational metadata, so Peek
// contains only richer content plus tags: description, attachment names,
// decision counts, checklist items, and dependency names.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	lines := make([]string, 0, inlinePeekMaxLines)
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

	if attachments := inlinePeekAttachmentsLine(opts.Task, available); attachments != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(attachments))
	}

	if decisions := inlinePeekDecisionCountLine(opts.Task, available); decisions != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(decisions))
	}

	if tags := inlinePeekTagsLine(opts.Task, available); tags != "" {
		lines = append(lines, opts.Indent+tags)
	}

	if checklist := inlinePeekChecklistLine(opts.Task, available); checklist != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(checklist))
	}

	needs, blocks := taskRelationships(opts.Task, opts.Project, opts.Projects)
	if dependencies := inlinePeekDependencyLine(needs, blocks, opts.Project, available); dependencies != "" {
		lines = append(lines, opts.Indent+MutedStyle.Render(dependencies))
	}

	if len(lines) == 0 {
		lines = append(lines, opts.Indent+MutedStyle.Render("No additional details"))
	}
	if len(lines) > inlinePeekMaxLines {
		lines = lines[:inlinePeekMaxLines]
	}
	return lines
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

func inlinePeekDecisionCountLine(task *models.Task, width int) string {
	if task == nil || len(task.Decisions) == 0 || width <= 0 {
		return ""
	}
	return truncateIntentional(fmt.Sprintf("Decisions: %d", len(task.Decisions)), width, "…")
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

func inlinePeekChecklistLine(task *models.Task, width int) string {
	if task == nil || len(task.Checklist) == 0 || width <= 0 {
		return ""
	}

	const maxItems = 2
	parts := make([]string, 0, maxItems+1)
	for i, item := range task.Checklist {
		if i >= maxItems {
			parts = append(parts, fmt.Sprintf("+%d more", len(task.Checklist)-maxItems))
			break
		}
		marker := "[ ]"
		if item.Done {
			marker = "[x]"
		}
		parts = append(parts, marker+" "+strings.TrimSpace(item.Text))
	}
	return truncateIntentional("Checklist: "+strings.Join(parts, " · "), width, "…")
}

func inlinePeekDependencyLine(
	needs, blocks []taskRelationship,
	owner *models.Project,
	width int,
) string {
	if width <= 0 || (len(needs) == 0 && len(blocks) == 0) {
		return ""
	}

	parts := make([]string, 0, 2)
	if len(needs) > 0 {
		parts = append(parts, "Needs: "+relationshipLabels(needs, owner))
	}
	if len(blocks) > 0 {
		parts = append(parts, "Blocks: "+relationshipLabels(blocks, owner))
	}
	return truncateIntentional(strings.Join(parts, " · "), width, "…")
}

func relationshipLabels(relationships []taskRelationship, owner *models.Project) string {
	labels := make([]string, 0, len(relationships))
	for _, relationship := range relationships {
		if relationship.Task == nil {
			continue
		}
		label := relationship.Task.Title
		if relationship.Project != nil && owner != nil && relationship.Project.ID != owner.ID {
			projectLabel := strings.TrimSpace(relationship.Project.Slug)
			if projectLabel == "" {
				projectLabel = strings.TrimSpace(relationship.Project.Name)
			}
			if projectLabel != "" {
				label = "[" + projectLabel + "] " + label
			}
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ", ")
}

func taskRelationships(
	task *models.Task,
	owner *models.Project,
	projects []*models.Project,
) (needs, blocks []taskRelationship) {
	return nil, nil
}

func uniqueRelationshipProjects(owner *models.Project, projects []*models.Project) []*models.Project {
	all := make([]*models.Project, 0, len(projects)+1)
	seen := make(map[string]bool, len(projects)+1)
	for _, project := range append([]*models.Project{owner}, projects...) {
		if project == nil || project.ID == "" || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		all = append(all, project)
	}
	return all
}
