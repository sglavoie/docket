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
// contains only richer content plus tags: description, tags, checklist items,
// dependency names, and recurrence detail.
func inlinePeekLines(opts inlinePeekOptions) []string {
	if opts.Task == nil || opts.Width <= 0 {
		return nil
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	lines := make([]string, 0, inlinePeekMaxLines)
	lines = append(lines, inlinePeekDescriptionLines(opts.Task.Description, opts.Indent, available)...)

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

	if opts.Task.Recurrence != nil {
		if recurrence := strings.TrimSpace(opts.Task.Recurrence.String()); recurrence != "" {
			lines = append(lines, opts.Indent+MutedStyle.Render(
				truncateIntentional("Repeats: "+recurrence, available, "…"),
			))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, opts.Indent+MutedStyle.Render("No additional details"))
	}
	if len(lines) > inlinePeekMaxLines {
		lines = lines[:inlinePeekMaxLines]
	}
	return lines
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
	if task == nil || owner == nil {
		return nil, nil
	}

	allProjects := uniqueRelationshipProjects(owner, projects)
	seenNeeds := make(map[TaskIdentity]bool)
	for _, stored := range task.DependsOn {
		dependency, project := resolveRelationshipReference(stored, owner, allProjects)
		if dependency == nil || project == nil {
			continue
		}
		identity := TaskIdentity{ProjectID: project.ID, TaskID: dependency.ID}
		if seenNeeds[identity] {
			continue
		}
		seenNeeds[identity] = true
		needs = append(needs, taskRelationship{Task: dependency, Project: project})
	}

	target := TaskIdentity{ProjectID: owner.ID, TaskID: task.ID}
	seenBlocks := make(map[TaskIdentity]bool)
	for _, project := range allProjects {
		if project == nil {
			continue
		}
		for _, candidate := range project.Tasks {
			if candidate == nil {
				continue
			}
			for _, stored := range candidate.DependsOn {
				dependency, dependencyProject := resolveRelationshipReference(stored, project, allProjects)
				if dependency == nil || dependencyProject == nil ||
					(TaskIdentity{ProjectID: dependencyProject.ID, TaskID: dependency.ID}) != target {
					continue
				}
				identity := TaskIdentity{ProjectID: project.ID, TaskID: candidate.ID}
				if seenBlocks[identity] {
					break
				}
				seenBlocks[identity] = true
				blocks = append(blocks, taskRelationship{Task: candidate, Project: project})
				break
			}
		}
	}
	return needs, blocks
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

func resolveRelationshipReference(
	stored string,
	owner *models.Project,
	projects []*models.Project,
) (*models.Task, *models.Project) {
	ref := models.ParseTaskRef(stored)
	if ref.TaskID == "" {
		return nil, nil
	}

	project := owner
	if ref.ProjectID != "" {
		project = nil
		for _, candidate := range projects {
			if candidate == nil {
				continue
			}
			if candidate.ID == ref.ProjectID || candidate.Slug == ref.ProjectID {
				project = candidate
				break
			}
		}
	}
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(ref.TaskID), project
}
