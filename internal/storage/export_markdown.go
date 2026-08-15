package storage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const maxHeadingLevel = 6

// ExportProjectMarkdown writes a clean markdown export of p to destPath.
// destPath must be non-empty; callers resolve default export locations through
// config.Manager.ExportPath. LoadProject is called first to ensure tasks are
// populated. Returns the final absolute path written.
func ExportProjectMarkdown(p *models.Project, destPath string) (string, error) {
	if err := LoadProject(p); err != nil {
		return "", fmt.Errorf("load project: %w", err)
	}

	if destPath == "" {
		return "", fmt.Errorf("export destination path is required")
	}

	data := WriteProjectMarkdownClean(p)

	notes, err := ListNotes(p)
	if err != nil {
		return "", fmt.Errorf("list notes: %w", err)
	}
	data += writeNotesSectionClean(notes)

	if err := os.WriteFile(destPath, []byte(data), 0600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}

	return destPath, nil
}

// WriteProjectMarkdownClean generates a human-readable markdown export of a
// project. Unlike canonical task storage, it omits HTML metadata comments while
// retaining the user-facing schema-6 task fields. Each task heading
// includes an inline badge and is prefixed with its short ID for traceability.
func WriteProjectMarkdownClean(p *models.Project) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", p.Name)

	hierarchy := indexTaskHierarchy(p.Tasks)
	visited := make(map[string]bool, len(p.Tasks))
	for _, task := range hierarchy.roots {
		builder.WriteString("\n")
		writeTaskClean(&builder, task, hierarchy.children, 2, visited)
	}

	appendOrphanedTasksClean(&builder, p.Tasks, hierarchy.children, visited)

	return builder.String()
}

// writeNotesSectionClean renders the trailing Notes section for a clean
// project export. Returns "" when notes is empty; otherwise emits a
// "\n---\n\n# Notes\n" separator followed by each note as "## Title" and the
// body verbatim. Notes flagged with a ParseError get a " [parse error]" suffix
// on the heading.
func writeNotesSectionClean(notes []*models.Note) string {
	if len(notes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n---\n\n# Notes\n")

	for _, note := range notes {
		b.WriteString("\n## ")
		b.WriteString(note.Title)
		if note.ParseError != "" {
			b.WriteString(" [parse error]")
		}
		b.WriteString("\n")

		if note.Body != "" {
			b.WriteString("\n")
			b.WriteString(note.Body)
			if !strings.HasSuffix(note.Body, "\n") {
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func writeTaskClean(builder *strings.Builder, task *models.Task, children map[string][]*models.Task, level int, visited map[string]bool) {
	if visited[task.ID] {
		return
	}
	visited[task.ID] = true

	writeLevel := min(level, maxHeadingLevel)
	hashes := strings.Repeat("#", writeLevel)

	title := fmt.Sprintf("(#%d) %s", task.ShortID, task.Title)
	badge := buildExportBadge(task)
	if badge != "" {
		title = title + " " + badge
	}

	fmt.Fprintf(builder, "%s Task: %s\n", hashes, title)

	if task.Context != "" {
		fmt.Fprintf(builder, "context: %s\n", task.Context)
	}

	if len(task.Tags) > 0 {
		fmt.Fprintf(builder, "tags: %s\n", strings.Join(task.Tags, ", "))
	}

	if task.Description != "" {
		builder.WriteString("\n")
		builder.WriteString(task.Description)
		builder.WriteString("\n")
	}

	if len(task.Checklist) > 0 {
		builder.WriteString("\n**Checklist:**\n")
		for _, item := range task.Checklist {
			marker := " "
			if item.Done {
				marker = "x"
			}
			fmt.Fprintf(builder, "- [%s] %s\n", marker, item.Text)
		}
	}

	if len(task.Comments) > 0 {
		builder.WriteString("\n**Comments:**\n")
		for _, comment := range task.Comments {
			if comment == nil {
				continue
			}
			fmt.Fprintf(builder, "- %s\n", comment.Text)
		}
	}

	if len(task.Attachments) > 0 {
		builder.WriteString("\n**Attachments:**\n")
		for _, attachment := range task.Attachments {
			switch {
			case attachment.IsURL:
				target := attachment.OriginalPath
				if target == "" {
					target = attachment.Filename
				}
				fmt.Fprintf(builder, "- URL: %s\n", target)
			case attachment.IsCodeRef:
				target := attachment.OriginalPath
				if target == "" {
					target = attachment.Filename
				}
				fmt.Fprintf(builder, "- Code: %s:%d\n", target, attachment.LineNumber)
			default:
				fmt.Fprintf(builder, "- File: %s\n", attachment.Filename)
			}
		}
	}

	for _, child := range children[task.ID] {
		builder.WriteString("\n")
		writeTaskClean(builder, child, children, level+1, visited)
	}
}

// buildExportBadge builds the inline badge string for a task export.
// Returns an empty string if no badge parts apply.
func buildExportBadge(task *models.Task) string {
	var parts []string

	// Show the workflow state unless it is the unremarkable default (backlog).
	if st := task.State(); st != models.StateBacklog {
		parts = append(parts, string(st))
	}
	if task.Priority != 0 {
		parts = append(parts, fmt.Sprintf("!%d", task.Priority))
	}
	if task.DueDate != nil {
		parts = append(parts, "due "+task.DueDate.Format("2006-01-02"))
	}
	if task.DeferredUntil != nil {
		// The deferral is a durable instant, but this export stays day-only so the
		// badge keeps one granularity beside the due date. The instant carries the
		// offset of whichever machine wrote it, so the day is resolved locally.
		parts = append(parts, "deferred until "+task.DeferredUntil.In(time.Local).Format("2006-01-02"))
	}

	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// appendOrphanedTasksClean appends tasks whose parent was not visited (orphans).
// Unlike appendOrphanedTasks, this does not log warnings since the output is
// user-facing markdown.
func appendOrphanedTasksClean(builder *strings.Builder, tasks []*models.Task, children map[string][]*models.Task, visited map[string]bool) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if visited[task.ID] {
			continue
		}
		builder.WriteString("\n")
		writeTaskClean(builder, task, children, 2, visited)
	}
}
