package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// WriteTasksMarkdown generates markdown content from tasks.
// Tasks are written in their slice order, preserving user-defined ordering.
func WriteTasksMarkdown(projectName string, tasks []*models.Task) string {
	return WriteTasksMarkdownWithSections(projectName, tasks, nil, nil)
}

// WriteTasksMarkdownWithSections generates markdown content from tasks and sections.
// Tasks are written in their slice order, preserving user-defined ordering.
func WriteTasksMarkdownWithSections(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", projectName)
	writeSectionsMetadata(&builder, sections, sectionColors)
	builder.WriteString("\n")

	rootTasks := collectRootTasks(tasks)
	visited := make(map[string]bool, len(tasks))
	for i, task := range rootTasks {
		writeTaskPreservingOrder(&builder, task, tasks, 2, visited)
		if i < len(rootTasks)-1 {
			builder.WriteString("\n")
		}
	}

	appendOrphanedTasks(&builder, tasks, visited)

	return builder.String()
}

func writeSectionsMetadata(builder *strings.Builder, sections []string, sectionColors map[string]string) {
	if len(sections) == 0 {
		return
	}

	var sectionParts []string
	for _, name := range sections {
		if color, ok := sectionColors[name]; ok && color != "" {
			sectionParts = append(sectionParts, name+":"+color)
			continue
		}
		sectionParts = append(sectionParts, name)
	}

	fmt.Fprintf(builder, "<!-- sections: %s -->\n", strings.Join(sectionParts, ";"))
}

func collectRootTasks(tasks []*models.Task) []*models.Task {
	var rootTasks []*models.Task
	for _, task := range tasks {
		if task.ParentID == nil {
			rootTasks = append(rootTasks, task)
		}
	}
	return rootTasks
}

func appendOrphanedTasks(builder *strings.Builder, tasks []*models.Task, visited map[string]bool) {
	for _, task := range tasks {
		if visited[task.ID] {
			continue
		}

		logging.Warn("task #%d %q has missing parent reference, saving as root-level entry", task.ShortID, task.Title)
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, task, tasks, 2, visited)
	}
}

// getChildrenInOrder returns tasks that are children of the given parent ID,
// preserving their original order in the tasks slice.
func getChildrenInOrder(parentID string, tasks []*models.Task) []*models.Task {
	var children []*models.Task
	for _, task := range tasks {
		if task.ParentID != nil && *task.ParentID == parentID {
			children = append(children, task)
		}
	}
	return children
}

// writeTaskPreservingOrder writes a single task and its children to the builder,
// preserving the original slice order for children.
// The visited set prevents infinite loops on circular parent references.
// maxHeadingLevel is the maximum markdown heading depth (######).
// Tasks nested deeper are clamped to this level to prevent writing headings
// that the parser's regex (#{2,6}) cannot read back, which would cause data loss.
// The parent: metadata field preserves the true parent relationship even when
// the heading level is capped.
const maxHeadingLevel = 6

func writeTaskPreservingOrder(builder *strings.Builder, task *models.Task, allTasks []*models.Task, level int, visited map[string]bool) {
	if visited[task.ID] {
		return
	}
	visited[task.ID] = true

	writeLevel := min(level, maxHeadingLevel)
	hashes := strings.Repeat("#", writeLevel)
	fmt.Fprintf(builder, "%s %s: %s\n", hashes, task.Type.HeadingPrefix(), task.Title)

	builder.WriteString(format.WriteMetadata(task))
	builder.WriteString("\n")

	if task.Description != "" {
		builder.WriteString("<!-- DESCRIPTION -->\n")
		builder.WriteString(escapeDescription(task.Description))
		builder.WriteString("\n<!-- /DESCRIPTION -->\n")
	}

	if len(task.Activity) > 0 {
		builder.WriteString("\n")
		builder.WriteString(format.WriteActivity(task))
	}

	if len(task.Comments) > 0 {
		builder.WriteString("\n")
		builder.WriteString(format.WriteComments(task))
	}

	for _, child := range getChildrenInOrder(task.ID, allTasks) {
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, child, allTasks, level+1, visited)
	}
}
