package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
)

// normalizedNoteLocation completes a half-named location. A message that
// carried only a project ID names that project's notes; one that named neither
// is global.
func normalizedNoteLocation(location app.NoteLocation, projectID string) app.NoteLocation {
	if location.Scope != "" {
		return location
	}
	if projectID != "" {
		return app.ProjectNoteLocation(projectID)
	}
	return app.GlobalNoteLocation()
}

// noteViewMatchesLocation reports whether the open manager owns the location a
// result came back for, which is what decides between a view status line and
// the shell's own error path.
func noteViewMatchesLocation(view *NotesViewModel, location app.NoteLocation) bool {
	if view == nil {
		return false
	}
	if location.Scope == app.NoteScopeGlobal {
		_, ok := view.owners[app.GlobalNoteLocation()]
		return ok
	}
	_, ok := view.owners[location]
	return ok
}

func startNoteEditor(editor string, location app.NoteLocation, note *models.Note, create bool) tea.Cmd {
	original := cloneNoteForCommand(note)
	content, err := storage.RenderNoteContent(original)
	if err != nil {
		return status.ErrorCmd("failed to render note: %w", err)
	}
	return startNoteEditorWithContent(editor, location, original, content, create)
}

func startNoteEditorWithContent(editor string, location app.NoteLocation, note *models.Note, content string, create bool) tea.Cmd {
	original := cloneNoteForCommand(note)
	tmpPath, err := external.CreateTempFileWithContent(content, "tt-note-edit-", ".md")
	if err != nil {
		return status.ErrorCmd("failed to create temporary note: %w", err)
	}
	command := external.EditorCommand(editor, tmpPath)
	return tea.ExecProcess(command, func(execErr error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()
		if execErr != nil {
			return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Create: create, Err: fmt.Errorf("editor failed: %w", execErr)}
		}
		modified, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Create: create, Err: fmt.Errorf("failed to read temporary note: %w", readErr)}
		}
		return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Modified: string(modified), Create: create}
	})
}

func parseProjectNoteEditorResult(original *models.Note, modified string, create bool) (*models.Note, error) {
	if original == nil {
		return nil, fmt.Errorf("edited note has no source")
	}
	updated, err := storage.ParseNoteContent(original.Filename, modified)
	if err != nil {
		return nil, fmt.Errorf("failed to parse edited note: %w", err)
	}
	if updated.ParseError != "" {
		return nil, fmt.Errorf("edited note has invalid frontmatter: %s (changes not saved)", updated.ParseError)
	}
	if strings.TrimSpace(updated.Title) == "" {
		return nil, fmt.Errorf("edited note has empty title (changes not saved)")
	}
	substituted := strings.TrimSuffix(original.Filename, ".md")
	if original.Title != "" && original.Title != substituted && updated.Title == substituted {
		return nil, fmt.Errorf("edited note has empty title (changes not saved)")
	}
	if create {
		updated.Filename = ""
		updated.RawRevision = ""
	} else {
		updated.CreatedAt = original.CreatedAt
		updated.ModifiedAt = time.Now()
		updated.Filename = original.Filename
		updated.RawRevision = original.RawRevision
	}
	return updated, nil
}
