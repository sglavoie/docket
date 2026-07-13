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
	return writeTasksMarkdownWithProjectMetadata(projectName, tasks, sections, sectionColors, nil, nil)
}

// WriteTasksMarkdownWithProjectMetadata generates markdown content from tasks,
// sections, and project-level task-file metadata.
func WriteTasksMarkdownWithProjectMetadata(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string, tags []string) string {
	return WriteTasksMarkdownWithFullProjectMetadata(projectName, tasks, sections, sectionColors, tags, nil)
}

// WriteTasksMarkdownWithFullProjectMetadata generates markdown content from
// tasks and all project-level task-file metadata.
func WriteTasksMarkdownWithFullProjectMetadata(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string, tags []string, pinnedTags []string) string {
	return writeTasksMarkdownWithProjectMetadata(projectName, tasks, sections, sectionColors, tags, pinnedTags)
}

func writeTasksMarkdownWithProjectMetadata(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string, tags []string, pinnedTags []string) string {
	return writeTasksMarkdownDocument(projectName, tasks, sections, sectionColors, tags, pinnedTags, nil, format.WriteMetadataV2)
}

// writeTasksMarkdownDocument is the shared serializer for both schema v1 and v2.
// The header hook writes any schema-specific header content (e.g. the v2 schema
// marker and preserved opaque blocks) immediately after the project-level
// metadata, and writeMeta selects the per-task metadata encoder (v1 legacy
// fields vs. v2 canonical `state`).
func writeTasksMarkdownDocument(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string, tags []string, pinnedTags []string, header func(*strings.Builder), writeMeta func(*models.Task) string) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", projectName)
	if header != nil {
		header(&builder)
	}
	writeSectionsMetadata(&builder, sections, sectionColors)
	writeTagsMetadata(&builder, tags)
	writePinnedTagsMetadata(&builder, pinnedTags)
	builder.WriteString("\n")

	rootTasks := collectRootTasks(tasks)
	visited := make(map[string]bool, len(tasks))
	for i, task := range rootTasks {
		writeTaskPreservingOrder(&builder, task, tasks, 2, visited, writeMeta)
		if i < len(rootTasks)-1 {
			builder.WriteString("\n")
		}
	}

	appendOrphanedTasks(&builder, tasks, visited, writeMeta)

	return builder.String()
}

func writeSectionsMetadata(builder *strings.Builder, sections []string, sectionColors map[string]string) {
	sections = models.FilterGTDSections(sections)
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

func writeTagsMetadata(builder *strings.Builder, tags []string) {
	tags = models.DedupeTags(tags)
	if len(tags) == 0 {
		return
	}

	fmt.Fprintf(builder, "<!-- tags: %s -->\n", strings.Join(tags, ";"))
}

func writePinnedTagsMetadata(builder *strings.Builder, pinnedTags []string) {
	pinnedTags = models.DedupeTags(pinnedTags)
	if len(pinnedTags) == 0 {
		return
	}

	fmt.Fprintf(builder, "<!-- pinned_tags: %s -->\n", strings.Join(pinnedTags, ";"))
}

func collectRootTasks(tasks []*models.Task) []*models.Task {
	var rootTasks []*models.Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.ParentID == nil {
			rootTasks = append(rootTasks, task)
		}
	}
	return rootTasks
}

func appendOrphanedTasks(builder *strings.Builder, tasks []*models.Task, visited map[string]bool, writeMeta func(*models.Task) string) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if visited[task.ID] {
			continue
		}

		logging.Warn("task #%d %q has missing parent reference, saving as root-level entry", task.ShortID, task.Title)
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, task, tasks, 2, visited, writeMeta)
	}
}

// getChildrenInOrder returns tasks that are children of the given parent ID,
// preserving their original order in the tasks slice.
func getChildrenInOrder(parentID string, tasks []*models.Task) []*models.Task {
	var children []*models.Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
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

func writeTaskPreservingOrder(builder *strings.Builder, task *models.Task, allTasks []*models.Task, level int, visited map[string]bool, writeMeta func(*models.Task) string) {
	if visited[task.ID] {
		return
	}
	visited[task.ID] = true

	writeLevel := min(level, maxHeadingLevel)
	hashes := strings.Repeat("#", writeLevel)
	// Title is trimmed because the parser strips surrounding whitespace
	// (parser.go parseTaskHeadingLine). Writing the raw title would make
	// the write→parse round-trip non-idempotent for titles with leading
	// or trailing spaces.
	fmt.Fprintf(builder, "%s %s: %s\n", hashes, task.Type.HeadingPrefix(), strings.TrimSpace(task.Title))

	builder.WriteString(writeMeta(task))
	builder.WriteString("\n")

	// Description is trimmed because the parser strips surrounding whitespace
	// in both descBuilder finalization and ExtractDescriptionBlock. Writing
	// the raw value would leave leading/trailing blank lines that disappear
	// on the next load, breaking round-trip idempotence.
	if desc := strings.TrimSpace(task.Description); desc != "" {
		builder.WriteString("<!-- DESCRIPTION -->\n")
		builder.WriteString(escapeDescription(desc))
		builder.WriteString("\n<!-- /DESCRIPTION -->\n")
	}

	if activity := format.WriteActivity(task); activity != "" {
		builder.WriteString("\n")
		builder.WriteString(activity)
	}

	// Comments/Decisions may legitimately return "" even when the slices are
	// non-empty (e.g. every entry has whitespace-only text and would be dropped
	// by the parser). Gate on the returned string so we don't emit a stray
	// leading newline before nothing.
	if comments := format.WriteComments(task); comments != "" {
		builder.WriteString("\n")
		builder.WriteString(comments)
	}

	if decisions := format.WriteDecisions(task); decisions != "" {
		builder.WriteString("\n")
		builder.WriteString(decisions)
	}

	for _, child := range getChildrenInOrder(task.ID, allTasks) {
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, child, allTasks, level+1, visited, writeMeta)
	}
}
