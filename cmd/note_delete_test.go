package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

func resetNoteDeleteVars() {
	noteDeleteForce = false
}

func TestRunNoteDelete(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetNoteDeleteVars()
		err := runNoteDelete(noteDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID")
		}
	})
}

func TestNoteDeleteForceDeletesMalformedNote(t *testing.T) {
	deps, project := setupNoteLinkTest(t)
	defer deps.Cleanup()
	noteGlobal = false
	resetNoteDeleteVars()
	if err := noteDeleteCmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	if err := noteDeleteCmd.Flags().Set("quiet", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = noteDeleteCmd.Flags().Set("force", "false")
		_ = noteDeleteCmd.Flags().Set("quiet", "false")
		resetNoteDeleteVars()
	})

	dir := storage.NotesDir(project)
	if err := os.MkdirAll(dir, storage.DirPermissions); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "malformed-cli.md")
	if err := os.WriteFile(path, []byte("frontmatter is missing but these bytes are still deletable\n"), storage.FilePermissions); err != nil {
		t.Fatal(err)
	}

	if err := runNoteDelete(noteDeleteCmd, []string{"malformed-cli.md"}); err != nil {
		t.Fatalf("runNoteDelete --force: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("malformed note still exists after note delete --force: %v", err)
	}
}
