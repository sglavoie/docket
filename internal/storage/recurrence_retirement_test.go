package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRecurrenceCannotEnterLivePersistenceMergeExportOrHistory(t *testing.T) {
	if _, ok := reflect.TypeOf(models.Task{}).FieldByName("Recurrence"); ok {
		t.Fatal("live Task still exposes recurrence")
	}
	for _, field := range mergeableTaskFields {
		if field.jsonName == "recurrence" {
			t.Fatal("merge field inventory still exposes recurrence")
		}
	}

	dataDir := t.TempDir()
	ledgerDir := filepath.Join(dataDir, ".tt", "undo")
	ledger := NewWorkspaceLedger(ledgerDir, []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := newCommitTestProject(t, dataDir, "retirement", "Finish once")
	if _, err := service.Commit(ChangeSet{Project: project, UndoAction: "create task"}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	assertTreeHasNoRecurrenceMetadata(t, project.Path)
	assertTreeHasNoRecurrenceMetadata(t, ledgerDir)

	document := ToProjectDocument(project)
	store := NewConflictStore(filepath.Join(dataDir, ".tt", "conflicts"))
	conflictID, err := store.Save(ConflictRecord{
		ProjectID: project.ID,
		Conflicts: []MergeConflict{{Kind: MergeConflictFieldEdit, TaskID: project.Tasks[0].ID, Field: "title"}},
	}, document, document, document)
	if err != nil {
		t.Fatalf("save conflict: %v", err)
	}
	assertTreeHasNoRecurrenceMetadata(t, filepath.Join(store.Dir(), conflictID))

	for name, content := range map[string]string{
		"schema-6 document": document.canonicalContent(),
		"clean export":      WriteProjectMarkdownClean(project),
	} {
		assertNoRecurrenceMetadata(t, name, []byte(content))
	}
}

func assertTreeHasNoRecurrenceMetadata(t *testing.T, root string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		assertNoRecurrenceMetadata(t, path, data)
		return nil
	}); err != nil {
		t.Fatalf("inspect %s: %v", root, err)
	}
}

func assertNoRecurrenceMetadata(t *testing.T, source string, data []byte) {
	t.Helper()
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"recur:", `"recurrence"`} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("%s contains retired recurrence metadata %q", source, forbidden)
		}
	}
}
