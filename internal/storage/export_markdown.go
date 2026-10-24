package storage

import (
	"fmt"
	"io"
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
//
// related are the other workspace projects an outgoing task reference may point
// into, supplied so a cross-project dependency can be named rather than left as
// a bare locator. They are loaded on a best effort: a project that cannot be
// read is skipped rather than failing the export, because its only contribution
// is a display label and the locator fallback already carries the whole value.
func ExportProjectMarkdown(p *models.Project, destPath string, related ...*models.Project) (string, error) {
	if destPath == "" {
		return "", fmt.Errorf("export destination path is required")
	}
	data, err := renderProjectExport(p, related)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(destPath, []byte(data), 0600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return destPath, nil
}

// ExportProjectMarkdownTo writes the same clean markdown export
// ExportProjectMarkdown produces to w instead of a file.
func ExportProjectMarkdownTo(w io.Writer, p *models.Project, related ...*models.Project) error {
	data, err := renderProjectExport(p, related)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, data); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	return nil
}

// renderProjectExport loads p and the related label population and renders
// the whole export document, notes included.
func renderProjectExport(p *models.Project, related []*models.Project) (string, error) {
	if err := LoadProject(p); err != nil {
		return "", fmt.Errorf("load project: %w", err)
	}

	loaded := make([]*models.Project, 0, len(related))
	for _, project := range related {
		if project == nil || project.ID == p.ID {
			continue
		}
		if err := LoadProject(project); err != nil {
			continue
		}
		loaded = append(loaded, project)
	}

	data := WriteProjectMarkdownClean(p, loaded...)

	notes, err := ListNotes(p)
	if err != nil {
		return "", fmt.Errorf("list notes: %w", err)
	}
	return data + writeNotesSectionClean(notes), nil
}

// WriteProjectMarkdownClean generates a human-readable markdown export of a
// project. Unlike canonical task storage, it omits HTML metadata comments while
// retaining the user-facing schema-6 task fields. Each task heading
// includes an inline badge and is prefixed with its short ID for traceability.
//
// related supplies the display labels for outgoing references that leave p. Only
// outgoing references are exported: dependents are computed rather than stored,
// so writing them into a document would turn a derived view into metadata that
// goes stale the moment another project changes.
func WriteProjectMarkdownClean(p *models.Project, related ...*models.Project) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "# Project: %s\n", p.Name)

	labels := newExportReferenceLabeller(p, related)
	hierarchy := indexTaskHierarchy(p.Tasks)
	visited := make(map[string]bool, len(p.Tasks))
	for _, task := range hierarchy.roots {
		builder.WriteString("\n")
		writeTaskClean(&builder, task, hierarchy.children, 2, visited, labels)
	}

	appendOrphanedTasksClean(&builder, p.Tasks, hierarchy.children, visited, labels)

	return builder.String()
}

// exportReferenceLabeller renders one outgoing reference for a clean export. It
// resolves against the exported project plus whichever other projects the caller
// supplied, and falls back to the durable locator for anything else, so no
// reference is ever dropped from the document for want of a label.
type exportReferenceLabeller struct {
	projects []*models.Project
}

func newExportReferenceLabeller(p *models.Project, related []*models.Project) exportReferenceLabeller {
	projects := make([]*models.Project, 0, len(related)+1)
	if p != nil {
		projects = append(projects, p)
	}
	for _, project := range related {
		if project != nil && (p == nil || project.ID != p.ID) {
			projects = append(projects, project)
		}
	}
	return exportReferenceLabeller{projects: projects}
}

// label names one reference and says what state it is in, which is the same
// `(state)` form Peek uses: a reader of an exported document is as entitled to
// know whether a reference is still open as a reader of the Outliner is, and
// spelling it the same way means the two never have to be reconciled.
//
// The trailing state is the whole of what an export says about a reference.
// There is deliberately no dependents list here and no transitive walk: an
// export is one project's document, so both would assert facts about work the
// document does not contain.
func (l exportReferenceLabeller) label(reference models.TaskReference) string {
	for _, project := range l.projects {
		if project.ID != reference.ProjectID {
			continue
		}
		task := project.GetTaskByID(reference.TaskID)
		if task == nil {
			break
		}
		return fmt.Sprintf("%s (#%d) %s (%s)", project.Name, task.ShortID, task.Title, task.State())
	}
	return reference.Locator()
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

// exportInlineMarkdownSpecials are the characters that carry inline Markdown
// meaning wherever they appear in a paragraph. Line-leading constructs — `#`,
// `-`, `>` at column zero — are absent deliberately: the text this escapes is
// spliced after a bold label, so it never starts a line.
const exportInlineMarkdownSpecials = "\\`*_[]<>&~"

// escapeExportInlineText backslash-escapes text that the clean export splices
// into a labeled paragraph, so a note reading `use *ptr not &ref` exports as
// those words rather than as emphasis and an entity. It is deliberately not
// applied to the description: a description is multi-line prose the author wrote
// as Markdown and the export emits it verbatim, while a resume note is one line
// collected through a prompt that never offered markup, so leaving it raw would
// let punctuation reformat a paragraph the author did not write.
func escapeExportInlineText(text string) string {
	var escaped strings.Builder
	escaped.Grow(len(text))
	for _, r := range text {
		if strings.ContainsRune(exportInlineMarkdownSpecials, r) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

func writeTaskClean(builder *strings.Builder, task *models.Task, children map[string][]*models.Task, level int, visited map[string]bool, labels exportReferenceLabeller) {
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

	if task.ResumeNote != "" {
		fmt.Fprintf(builder, "\n**Resume:** %s\n", escapeExportInlineText(task.ResumeNote))
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

	if len(task.Dependencies) > 0 {
		builder.WriteString("\n**Dependencies:**\n")
		for _, reference := range task.Dependencies {
			fmt.Fprintf(builder, "- %s\n", labels.label(reference))
		}
	}

	for _, child := range children[task.ID] {
		builder.WriteString("\n")
		writeTaskClean(builder, child, children, level+1, visited, labels)
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
		if task.DueHasTime() {
			parts = append(parts, "due "+task.DueDate.Format("2006-01-02 15:04"))
		} else {
			parts = append(parts, "due "+task.DueDate.Format("2006-01-02"))
		}
	}
	if task.DeferredUntil != nil {
		// The deferral is a durable instant, but this export stays day-only so the
		// badge keeps one granularity beside the due date. The instant carries the
		// offset of whichever machine wrote it, so the day is resolved locally.
		parts = append(parts, "deferred until "+task.DeferredUntil.In(time.Local).Format("2006-01-02"))
	}
	// A check-in is stored as a calendar date rather than an instant, so it needs
	// no zone resolution and is formatted as written. The repeat rides the same
	// segment because a cadence without its next occurrence says nothing, and the
	// review instant stays out of the badge entirely: the badge names what a task
	// still owes, and a review is history.
	if schedule := task.CheckIn(); schedule.Next != nil {
		segment := "check-in " + schedule.Next.Format("2006-01-02")
		if schedule.Repeats() {
			segment += fmt.Sprintf(" every %dd", schedule.RepeatDays)
		}
		parts = append(parts, segment)
	}
	// The export writes every stored task, hidden ones included, because it is a
	// record of the project rather than a view of it; the badge says which tasks
	// every view leaves out, so a reader is not surprised by work they never saw.
	if task.Hidden {
		parts = append(parts, "hidden")
	}

	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// appendOrphanedTasksClean appends tasks whose parent was not visited (orphans).
// Unlike appendOrphanedTasks, this does not log warnings since the output is
// user-facing markdown.
func appendOrphanedTasksClean(builder *strings.Builder, tasks []*models.Task, children map[string][]*models.Task, visited map[string]bool, labels exportReferenceLabeller) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if visited[task.ID] {
			continue
		}
		builder.WriteString("\n")
		writeTaskClean(builder, task, children, 2, visited, labels)
	}
}
