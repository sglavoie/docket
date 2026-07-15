package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// NoteContext holds a path-free workspace note location plus presentation data.
type NoteContext struct {
	Location  app.NoteLocation
	Project   *models.Project // nil for global notes; used only for labels/task lookup
	Editor    string          // machine-local editor selected at command startup
	Workspace *CLIWorkspace   // domain-native commit composition
}

// resolveNoteContext returns the path-free note location and optional project
// presentation context selected by the --global flag.
func resolveNoteContext() (*NoteContext, error) {
	if noteGlobal {
		workspace, err := LoadCLIWorkspaceStrict()
		if err != nil {
			return nil, err
		}
		return &NoteContext{Location: app.GlobalNoteLocation(), Editor: workspace.Runtime.Machine().GetEditor(), Workspace: workspace}, nil
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return nil, err
	}
	return &NoteContext{Location: app.ProjectNoteLocation(project.ID), Project: project, Editor: workspace.Runtime.Machine().GetEditor(), Workspace: workspace}, nil
}

func notesNoteContext(ctx *NoteContext) ([]*models.Note, error) {
	return newWorkspaceAppService(ctx.Workspace).NotesAt(ctx.Location)
}

func createNoteContext(ctx *NoteContext, note *models.Note) (app.NoteResult, error) {
	result, err := newWorkspaceAppService(ctx.Workspace).CreateNote(app.NoteCreateRequest{Location: ctx.Location, Note: note})
	return result, translateAppError(err)
}

func updateNoteContext(ctx *NoteContext, note *models.Note) (app.NoteResult, error) {
	result, err := newWorkspaceAppService(ctx.Workspace).UpdateNote(app.NoteUpdateRequest{Location: ctx.Location, Note: note})
	return result, translateAppError(err)
}

func deleteNoteContext(ctx *NoteContext, note *models.Note) (app.NoteResult, error) {
	result, err := newWorkspaceAppService(ctx.Workspace).RemoveNote(app.NoteDeleteRequest{Location: ctx.Location, Note: note})
	return result, translateAppError(err)
}

func findNoteContext(ctx *NoteContext, query string) (*models.Note, error) {
	notes, err := notesNoteContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	return findNoteFromList(notes, query)
}

// findNoteIn locates a note in a directory by exact filename or title prefix match.
// An empty query is rejected: HasPrefix(anything, "") and Contains(anything, "")
// both return true, so without this guard an empty arg silently matches the first
// note (e.g. `tt note delete ""` would delete the first note found).
//
// When the query is not an exact filename/title match, the function uses
// prefix matching against both the title and the filename (without .md
// extension). If multiple notes share the same prefix, an ambiguous error is
// returned so a destructive command like `note delete` cannot silently target
// the wrong note.
func findNoteIn(dir, query string) (*models.Note, error) {
	// Try exact filename first
	if query != "" && strings.HasSuffix(query, ".md") {
		note, err := storage.LoadNoteFrom(dir, query)
		if err == nil {
			return note, nil
		}
	}

	notes, err := storage.ListNotesFrom(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	return findNoteFromList(notes, query)
}

func findNoteFromList(notes []*models.Note, query string) (*models.Note, error) {
	if query == "" {
		return nil, fmt.Errorf("note not found: %s", query)
	}

	// Two-pass search: prefer an exact title match (case-insensitive) over a
	// prefix match. Without this, querying "meeting" when notes
	// "meeting" and "meeting-followup" both exist silently returned whichever
	// note happened to be listed first.
	lowerQuery := strings.ToLower(query)
	for _, note := range notes {
		if strings.ToLower(note.Filename) == lowerQuery {
			return note, nil
		}
		if strings.ToLower(note.Title) == lowerQuery {
			return note, nil
		}
		nameWithoutExt := strings.TrimSuffix(note.Filename, ".md")
		if strings.ToLower(nameWithoutExt) == lowerQuery {
			return note, nil
		}
	}
	// Use prefix matching (not substring) for filenames so multiple notes
	// containing the query don't silently collapse to whichever one storage
	// happened to list first; collect every prefix match so we can report
	// ambiguity instead of returning the first hit.
	var matches []*models.Note
	seen := make(map[string]bool)
	for _, note := range notes {
		if strings.HasPrefix(strings.ToLower(note.Title), lowerQuery) {
			if !seen[note.Filename] {
				matches = append(matches, note)
				seen[note.Filename] = true
			}
			continue
		}
		nameWithoutExt := strings.TrimSuffix(note.Filename, ".md")
		if strings.HasPrefix(strings.ToLower(nameWithoutExt), lowerQuery) {
			if !seen[note.Filename] {
				matches = append(matches, note)
				seen[note.Filename] = true
			}
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("note not found: %s", query)
	case 1:
		return matches[0], nil
	default:
		limit := min(len(matches), 5)
		parts := make([]string, limit)
		for i := 0; i < limit; i++ {
			parts[i] = matches[i].Filename
		}
		extra := ""
		if len(matches) > 5 {
			extra = fmt.Sprintf(", ... (%d more)", len(matches)-5)
		}
		return nil, fmt.Errorf("ambiguous note query %q: matches %s%s", query, strings.Join(parts, ", "), extra)
	}
}
