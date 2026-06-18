package external

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ReferenceSnapshotWarning is included in every reference snapshot so users do
// not mistake the opened copy for an editable source document.
const ReferenceSnapshotWarning = "Reference copy: edits made in this file are not imported back into tt."

const defaultReferenceSnapshotMaxAge = 24 * time.Hour

var (
	referenceSnapshotCacheDir = defaultReferenceSnapshotCacheDir
	referenceSnapshotMaxAge   = defaultReferenceSnapshotMaxAge
	referenceSnapshotNow      = time.Now
)

// FormatTaskReferenceMarkdownWithNotes formats a task as a read-only reference
// snapshot and includes notes known to link to the task.
func FormatTaskReferenceMarkdownWithNotes(task *models.Task, linkedNotes []*models.Note) string {
	if task == nil {
		var b strings.Builder
		b.WriteString("# Task Reference\n\n")
		writeReferenceWarning(&b)
		b.WriteString("Task unavailable.\n")
		return b.String()
	}

	var b strings.Builder
	b.WriteString(insertReferenceWarning(FormatTaskAsMarkdown(task)))
	writeReferenceTaskDetails(&b, task, linkedNotes)
	return b.String()
}

// FormatNoteReferenceMarkdownWithTasks formats a note as a read-only reference
// snapshot and summarizes linked tasks when task records are available.
func FormatNoteReferenceMarkdownWithTasks(note *models.Note, linkedTasks []*models.Task) string {
	var b strings.Builder
	if note == nil {
		b.WriteString("# Note Reference\n\n")
		writeReferenceWarning(&b)
		b.WriteString("Note unavailable.\n")
		return b.String()
	}

	title := note.Title
	if title == "" {
		title = models.TitleFromFilename(note.Filename)
	}
	fmt.Fprintf(&b, "# Note: %s\n\n", title)
	writeReferenceWarning(&b)

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	if note.Filename != "" {
		fmt.Fprintf(&b, "| Filename | %s |\n", markdownTableCell(note.Filename))
	}
	if len(note.Tags) > 0 {
		fmt.Fprintf(&b, "| Tags | %s |\n", markdownTableCell(strings.Join(note.Tags, ", ")))
	}
	if !note.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "| Created | %s |\n", note.CreatedAt.Format(util.DateTimeFormat))
	}
	if !note.ModifiedAt.IsZero() {
		fmt.Fprintf(&b, "| Modified | %s |\n", note.ModifiedAt.Format(util.DateTimeFormat))
	}
	if note.Pinned {
		b.WriteString("| Pinned | Yes |\n")
	}
	b.WriteString("\n")

	writeLinkedTaskSummary(&b, note.LinkedTasks, linkedTasks)

	if note.ParseError != "" {
		b.WriteString("## Parse Error\n\n")
		b.WriteString(note.ParseError)
		b.WriteString("\n\n")
	}

	if strings.TrimSpace(note.Body) != "" {
		b.WriteString("## Body\n\n")
		b.WriteString(note.Body)
		if !strings.HasSuffix(note.Body, "\n") {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// CreateReferenceSnapshotFile stages markdown in tt's reference snapshot cache.
// The file intentionally survives after launch so async GUI editors can read it.
// Stale cached snapshots are removed opportunistically before each new snapshot.
func CreateReferenceSnapshotFile(content string, prefix string, suffix string) (string, error) {
	if prefix == "" {
		prefix = "tt-reference-"
	}
	if suffix == "" {
		suffix = ".md"
	}

	dir, err := referenceSnapshotCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create reference snapshot cache: %w", err)
	}
	_ = cleanupStaleReferenceSnapshots(dir)

	return createTempFileWithContentInDir(dir, content, prefix, suffix)
}

func defaultReferenceSnapshotCacheDir() (string, error) {
	cacheDir, _ := os.UserCacheDir()
	if cacheDir == "" {
		return filepath.Join(os.TempDir(), "tt", "reference-snapshots"), nil
	}
	return filepath.Join(cacheDir, "tt", "reference-snapshots"), nil
}

func cleanupStaleReferenceSnapshots(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	cutoff := referenceSnapshotNow().Add(-referenceSnapshotMaxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
	return nil
}

func insertReferenceWarning(markdown string) string {
	markdown = strings.TrimRight(markdown, "\n")
	idx := strings.Index(markdown, "\n\n")
	if idx < 0 {
		var b strings.Builder
		b.WriteString(markdown)
		b.WriteString("\n\n")
		writeReferenceWarning(&b)
		return b.String()
	}

	var b strings.Builder
	b.WriteString(markdown[:idx+2])
	writeReferenceWarning(&b)
	b.WriteString(markdown[idx+2:])
	b.WriteString("\n\n")
	return b.String()
}

func writeReferenceWarning(b *strings.Builder) {
	fmt.Fprintf(b, "> **%s**\n\n", ReferenceSnapshotWarning)
}

func writeReferenceTaskDetails(b *strings.Builder, task *models.Task, linkedNotes []*models.Note) {
	if len(task.Checklist) > 0 {
		b.WriteString("## Checklist\n\n")
		for _, item := range task.Checklist {
			checkbox := "[ ]"
			if item.Done {
				checkbox = "[x]"
			}
			fmt.Fprintf(b, "- %s %s\n", checkbox, item.Text)
		}
		b.WriteString("\n")
	}

	if len(task.Comments) > 0 {
		b.WriteString("## Comments\n\n")
		for _, comment := range task.Comments {
			if comment == nil {
				continue
			}
			status := ""
			if comment.Resolved {
				status = " (resolved)"
			}
			if comment.CreatedAt.IsZero() {
				fmt.Fprintf(b, "- %s%s\n", formatMarkdownListText(comment.Text), status)
			} else {
				fmt.Fprintf(b, "- %s: %s%s\n", comment.CreatedAt.Format(util.DateTimeFormat), formatMarkdownListText(comment.Text), status)
			}
		}
		b.WriteString("\n")
	}

	if len(task.Decisions) > 0 {
		b.WriteString("## Decisions\n\n")
		for _, decision := range task.Decisions {
			if decision == nil {
				continue
			}
			var parts []string
			if !decision.CreatedAt.IsZero() {
				parts = append(parts, decision.CreatedAt.Format(util.DateTimeFormat))
			}
			if decision.Category != "" {
				parts = append(parts, string(decision.Category))
			}
			if decision.Resolved {
				parts = append(parts, "resolved")
			}
			prefix := ""
			if len(parts) > 0 {
				prefix = strings.Join(parts, " | ") + ": "
			}
			fmt.Fprintf(b, "- %s%s\n", prefix, formatMarkdownListText(decision.Text))
			if decision.Rationale != "" {
				fmt.Fprintf(b, "  - Rationale: %s\n", formatMarkdownListText(decision.Rationale))
			}
		}
		b.WriteString("\n")
	}

	if len(task.RelatedTo) > 0 {
		b.WriteString("## Related Tasks\n\n")
		for _, id := range task.RelatedTo {
			if strings.TrimSpace(id) == "" {
				continue
			}
			fmt.Fprintf(b, "- %s\n", id)
		}
		b.WriteString("\n")
	}

	if len(linkedNotes) > 0 {
		b.WriteString("## Linked Notes\n\n")
		for _, note := range linkedNotes {
			if note == nil {
				continue
			}
			fmt.Fprintf(b, "- %s\n", formatNoteSummary(note))
		}
		b.WriteString("\n")
	}
}

func writeLinkedTaskSummary(b *strings.Builder, linkedTaskIDs []string, linkedTasks []*models.Task) {
	if len(linkedTaskIDs) == 0 && len(linkedTasks) == 0 {
		return
	}

	taskByID := make(map[string]*models.Task, len(linkedTasks))
	for _, task := range linkedTasks {
		if task != nil && task.ID != "" {
			taskByID[task.ID] = task
		}
	}

	b.WriteString("## Linked Tasks\n\n")
	seen := map[string]bool{}
	for _, id := range linkedTaskIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if task := taskByID[id]; task != nil {
			fmt.Fprintf(b, "- %s\n", formatTaskSummary(task))
			continue
		}
		fmt.Fprintf(b, "- %s\n", id)
	}
	for _, task := range linkedTasks {
		if task == nil || task.ID == "" || seen[task.ID] {
			continue
		}
		seen[task.ID] = true
		fmt.Fprintf(b, "- %s\n", formatTaskSummary(task))
	}
	b.WriteString("\n")
}

func formatTaskSummary(task *models.Task) string {
	if task.ShortID > 0 {
		return fmt.Sprintf("#%d %s: %s (%s) [%s]", task.ShortID, task.Type.HeadingPrefix(), task.Title, task.Status, task.ID)
	}
	return fmt.Sprintf("%s (%s)", task.Title, task.ID)
}

func formatNoteSummary(note *models.Note) string {
	title := note.Title
	if title == "" {
		title = models.TitleFromFilename(note.Filename)
	}
	if note.Filename == "" {
		return title
	}
	return fmt.Sprintf("%s (%s)", title, note.Filename)
}

func formatMarkdownListText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	return strings.ReplaceAll(s, "\n", "\n  ")
}
