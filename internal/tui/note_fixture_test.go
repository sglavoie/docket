package tui

import (
	"os"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func saveTestNote(dir string, note *models.Note) error {
	if err := os.MkdirAll(dir, storage.DirPermissions); err != nil {
		return err
	}
	path, err := storage.NoteFilePathIn(dir, note.Filename)
	if err != nil {
		return err
	}
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), storage.FilePermissions)
}
