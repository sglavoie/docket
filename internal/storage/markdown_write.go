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
	return writeTasksMarkdownWithProjectMetadata(projectName, tasks, nil, nil)
}

// WriteTasksMarkdownWithProjectMetadata generates canonical schema-v2 markdown
// with all project-level task-file metadata.
func WriteTasksMarkdownWithProjectMetadata(projectName string, tasks []*models.Task, tags []string, pinnedTags []string) string {
	return writeTasksMarkdownWithProjectMetadata(projectName, tasks, tags, pinnedTags)
}

func writeTasksMarkdownWithProjectMetadata(projectName string, tasks []*models.Task, tags []string, pinnedTags []string) string {
	header := func(b *strings.Builder) {
		fmt.Fprintf(b, "<!-- tt-schema: %d -->\n", SchemaVersionV2)
	}
	return writeTasksMarkdownDocument(projectName, tasks, tags, pinnedTags, header, format.WriteMetadataV2)
}

// writeTasksMarkdownDocument is the canonical serializer used by ordinary
// project writes and ProjectDocument. The header hook writes the schema marker
// and any preserved opaque blocks before structured project metadata; writeMeta
// supplies the canonical per-task metadata encoder.
func writeTasksMarkdownDocument(projectName string, tasks []*models.Task, tags []string, pinnedTags []string, header func(*strings.Builder), writeMeta func(*models.Task) string) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", projectName)
	if header != nil {
		header(&builder)
	}
	writeTagsMetadata(&builder, tags)
	writePinnedTagsMetadata(&builder, pinnedTags)
	builder.WriteString("\n")

	hierarchy := indexTaskHierarchy(tasks)
	visited := make(map[string]bool, len(tasks))
	for i, task := range hierarchy.roots {
		writeTaskPreservingOrder(&builder, task, hierarchy.children, 2, visited, writeMeta)
		if i < len(hierarchy.roots)-1 {
			builder.WriteString("\n")
		}
	}

	appendOrphanedTasks(&builder, tasks, hierarchy.children, visited, writeMeta)

	return builder.String()
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

// taskHierarchyIndex records the roots and ordered child adjacency lists for a
// flat task slice. Building it once keeps serialization linear: the previous
// recursive writer rescanned every task to discover each node's children,
// making a wide or deeply nested project O(n²).
type taskHierarchyIndex struct {
	roots    []*models.Task
	children map[string][]*models.Task
}

func indexTaskHierarchy(tasks []*models.Task) taskHierarchyIndex {
	index := taskHierarchyIndex{children: make(map[string][]*models.Task)}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.ParentID == nil {
			index.roots = append(index.roots, task)
			continue
		}
		index.children[*task.ParentID] = append(index.children[*task.ParentID], task)
	}
	return index
}

func appendOrphanedTasks(builder *strings.Builder, tasks []*models.Task, children map[string][]*models.Task, visited map[string]bool, writeMeta func(*models.Task) string) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if visited[task.ID] {
			continue
		}

		logging.Warn("task #%d %q has missing parent reference, saving as root-level entry", task.ShortID, task.Title)
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, task, children, 2, visited, writeMeta)
	}
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

func writeTaskPreservingOrder(builder *strings.Builder, task *models.Task, children map[string][]*models.Task, level int, visited map[string]bool, writeMeta func(*models.Task) string) {
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

	for _, child := range children[task.ID] {
		builder.WriteString("\n")
		writeTaskPreservingOrder(builder, child, children, level+1, visited, writeMeta)
	}
}
