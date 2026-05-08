package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// ExportProjectMarkdown writes a clean markdown export of p to destPath.
// If destPath is "", it defaults to ~/.tt/exports/<slug>.md.
// LoadProject is called first to ensure tasks are populated.
// Returns the final absolute path written.
func ExportProjectMarkdown(p *models.Project, destPath string) (string, error) {
	if err := LoadProject(p); err != nil {
		return "", fmt.Errorf("load project: %w", err)
	}

	if destPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		dir := filepath.Join(homeDir, ".tt", "exports")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("create exports dir: %w", err)
		}
		destPath = filepath.Join(dir, p.Slug+".md")
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

// WriteProjectMarkdownClean generates a human-readable markdown export of a project.
// Unlike WriteTasksMarkdown, it omits HTML metadata comments, activity logs,
// and comments. Each task heading includes an inline badge (status, priority,
// due date, snooze date) and is prefixed with its short ID for traceability.
func WriteProjectMarkdownClean(p *models.Project) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", p.Name)

	rootTasks := collectRootTasks(p.Tasks)
	visited := make(map[string]bool, len(p.Tasks))
	for _, task := range rootTasks {
		builder.WriteString("\n")
		writeTaskClean(&builder, task, p.Tasks, 2, visited)
	}

	appendOrphanedTasksClean(&builder, p.Tasks, visited)

	return builder.String()
}

// writeNotesSectionClean renders the trailing Notes section for a clean
// project export. Returns "" when notes is empty; otherwise emits a
// "\n---\n\n# Notes\n" separator followed by each note as "## Title",
// an optional tags line, and the body verbatim. Notes flagged with a
// ParseError get a " [parse error]" suffix on the heading.
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

		if len(note.Tags) > 0 {
			fmt.Fprintf(&b, "tags: %s\n", strings.Join(note.Tags, ", "))
		}

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

func writeTaskClean(builder *strings.Builder, task *models.Task, allTasks []*models.Task, level int, visited map[string]bool) {
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

	fmt.Fprintf(builder, "%s %s: %s\n", hashes, task.Type.HeadingPrefix(), title)

	if len(task.Tags) > 0 {
		fmt.Fprintf(builder, "tags: %s\n", strings.Join(task.Tags, ", "))
	}

	if task.Description != "" {
		builder.WriteString("\n")
		builder.WriteString(task.Description)
		builder.WriteString("\n")
	}

	if len(task.Decisions) > 0 {
		builder.WriteString("\n**Decisions:**\n")
		for _, d := range task.Decisions {
			fmt.Fprintf(builder, "- **[%s]** %s\n", string(d.Category), d.Text)
			if d.Rationale != "" {
				fmt.Fprintf(builder, "  *Rationale:* %s\n", d.Rationale)
			}
			if d.Resolved {
				builder.WriteString("  *(resolved)*\n")
			}
		}
	}

	for _, child := range getChildrenInOrder(task.ID, allTasks) {
		builder.WriteString("\n")
		writeTaskClean(builder, child, allTasks, level+1, visited)
	}
}

// buildExportBadge builds the inline badge string for a task export.
// Returns an empty string if no badge parts apply.
func buildExportBadge(task *models.Task) string {
	var parts []string

	if task.Status != models.StatusTodo {
		parts = append(parts, string(task.Status))
	}
	if task.Priority != 0 {
		parts = append(parts, fmt.Sprintf("!%d", task.Priority))
	}
	if task.DueDate != nil {
		if task.HasTime {
			parts = append(parts, "due "+task.DueDate.Format("2006-01-02 15:04"))
		} else {
			parts = append(parts, "due "+task.DueDate.Format("2006-01-02"))
		}
	}
	if task.SnoozedUntil != nil {
		parts = append(parts, "snoozed until "+task.SnoozedUntil.Format("2006-01-02"))
	}

	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// appendOrphanedTasksClean appends tasks whose parent was not visited (orphans).
// Unlike appendOrphanedTasks, this does not log warnings since the output is
// user-facing markdown.
func appendOrphanedTasksClean(builder *strings.Builder, tasks []*models.Task, visited map[string]bool) {
	for _, task := range tasks {
		if visited[task.ID] {
			continue
		}
		builder.WriteString("\n")
		writeTaskClean(builder, task, tasks, 2, visited)
	}
}
