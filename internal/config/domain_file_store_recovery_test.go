package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

// This file covers the checked-revision and backup halves of the store: a stale
// save that must refuse rather than clobber a concurrent writer, a corrupt
// primary recovered from its backup with a revision that stays usable as a
// transaction baseline, and the semantic failures a backup must never paper
// over. The fixtures are in domain_file_store_test.go.

func TestDomainFileStore_ConcurrentStaleObjectDetection(t *testing.T) {
	s := newIsolatedStore(t)

	// Seed an initial catalog.
	if err := s.SaveCatalog(NewWorkspaceCatalog()); err != nil {
		t.Fatalf("seed SaveCatalog: %v", err)
	}

	// Reader A loads the current revision.
	catA, revA, err := s.LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked A: %v", err)
	}

	// Writer B advances the on-disk artifact behind A's back.
	catB := NewWorkspaceCatalog()
	catB.Projects = append(catB.Projects, CatalogProject{ID: "pB", Slug: "b", Name: "B"})
	if saveErr := s.SaveCatalog(catB); saveErr != nil {
		t.Fatalf("writer B SaveCatalog: %v", saveErr)
	}

	// A's checked save must now fail as stale rather than clobbering B.
	catA.Projects = append(catA.Projects, CatalogProject{ID: "pA", Slug: "a", Name: "A"})
	err = s.SaveCatalogChecked(catA, revA)
	if !errors.Is(err, ErrStaleObject) {
		t.Fatalf("stale checked save = %v, want ErrStaleObject", err)
	}

	// B's changes survived; A's did not.
	got, err := s.LoadCatalog()
	if err != nil {
		t.Fatalf("reload after stale save: %v", err)
	}
	if len(got.Projects) != 1 || got.Projects[0].ID != "pB" {
		t.Fatalf("stale save clobbered concurrent writer: %+v", got.Projects)
	}

	// Re-loading gives a fresh revision that saves cleanly.
	fresh, revFresh, err := s.LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked fresh: %v", err)
	}
	fresh.Projects = append(fresh.Projects, CatalogProject{ID: "pA", Slug: "a", Name: "A"})
	if err := s.SaveCatalogChecked(fresh, revFresh); err != nil {
		t.Fatalf("checked save with fresh revision: %v", err)
	}
}

func TestDomainFileStore_RecoversFromBackupWithRepairedRevision(t *testing.T) {
	s := newIsolatedStore(t)

	planning := NewPlanningState()
	planning.SetToday([]ProjectTaskLocator{{TaskID: "version-7", ProjectID: "project"}})
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("first SavePlanning: %v", err)
	}
	// A second save produces a .bak of the first good file.
	planning.SetToday([]ProjectTaskLocator{{TaskID: "version-8", ProjectID: "project"}})
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("second SavePlanning: %v", err)
	}

	// Corrupt the primary; the first planning payload should be recovered.
	path := PlanningStatePath(s.DataDir())
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read planning backup: %v", err)
	}
	if writeErr := os.WriteFile(path, []byte("{not json"), FilePermissions); writeErr != nil {
		t.Fatalf("corrupt primary: %v", writeErr)
	}

	recovered, recoveredRevision, err := NewDomainFileStore(s.DataDir()).LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked after corruption: %v", err)
	}
	if len(recovered.Today) != 1 || recovered.Today[0].TaskID != "version-7" {
		t.Fatalf("recovered planning = %+v, want version-7 from backup", recovered.Today)
	}
	repairedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired primary: %v", err)
	}
	if !bytes.Equal(repairedData, backupData) {
		t.Fatal("repaired primary bytes differ from the recovered backup")
	}
	if want := revisionOf(repairedData); recoveredRevision != want {
		t.Fatalf("recovered revision = %q, want repaired-byte revision %q", recoveredRevision, want)
	}

	// The revision returned by the recovering load must be immediately usable as
	// a checked transaction baseline; hashing the corrupt pre-recovery bytes would
	// make this commit fail stale.
	recovered.SetToday([]ProjectTaskLocator{{TaskID: "version-9", ProjectID: "project"}})
	updated, err := encodeDomainEnvelope(domainPlanning, testWorkspaceID, recovered)
	if err != nil {
		t.Fatalf("encode recovered planning update: %v", err)
	}
	if _, commitErr := storage.NewCommitService(TransactionsDir(s.DataDir()), []string{s.DataDir()}).CommitArtifacts([]storage.ArtifactChange{{
		Path:             path,
		Data:             updated,
		Mode:             FilePermissions,
		ExpectedRevision: &recoveredRevision,
	}}); commitErr != nil {
		t.Fatalf("commit checked artifact after recovery: %v", commitErr)
	}
	fresh, err := NewDomainFileStore(s.DataDir()).LoadPlanning()
	if err != nil {
		t.Fatalf("reload committed planning: %v", err)
	}
	if len(fresh.Today) != 1 || fresh.Today[0].TaskID != "version-9" {
		t.Fatalf("committed planning = %+v, want version-9", fresh.Today)
	}
}

func TestDomainFileStore_RecoversTransactionalDomainBackup(t *testing.T) {
	s := newIsolatedStore(t)
	planning := NewPlanningState()
	planning.SetToday([]ProjectTaskLocator{{TaskID: "version-7", ProjectID: "project"}})
	if err := s.SavePlanning(planning); err != nil {
		t.Fatalf("seed planning: %v", err)
	}
	path := PlanningStatePath(s.DataDir())
	before, revision, err := s.LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked: %v", err)
	}
	before.SetToday([]ProjectTaskLocator{{TaskID: "version-8", ProjectID: "project"}})
	updated, err := encodeDomainEnvelope(domainPlanning, testWorkspaceID, before)
	if err != nil {
		t.Fatalf("encode planning update: %v", err)
	}
	change := (DomainArtifact{Path: path, Data: updated, ExpectedRevision: &revision}).StorageChange()
	if _, commitErr := storage.NewCommitService(TransactionsDir(s.DataDir()), []string{s.DataDir()}).CommitArtifacts([]storage.ArtifactChange{change}); commitErr != nil {
		t.Fatalf("CommitArtifacts: %v", commitErr)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read transactional backup: %v", err)
	}
	if writeErr := os.WriteFile(path, []byte("{not json"), FilePermissions); writeErr != nil {
		t.Fatalf("corrupt primary: %v", writeErr)
	}
	recovered, _, err := NewDomainFileStore(s.DataDir()).LoadPlanningChecked()
	if err != nil {
		t.Fatalf("LoadPlanningChecked after corruption: %v", err)
	}
	if len(recovered.Today) != 1 || recovered.Today[0].TaskID != "version-7" {
		t.Fatalf("recovered planning = %+v, want version-7", recovered.Today)
	}
	if repaired, err := os.ReadFile(path); err != nil {
		t.Fatalf("read repaired primary: %v", err)
	} else if !bytes.Equal(repaired, backup) {
		t.Fatal("repaired primary differs from transactional backup")
	}
}

func TestDomainFileStore_BackupDoesNotOverrideSemanticPrimaryErrors(t *testing.T) {
	goodBackup, err := encodeDomainEnvelope(domainPlanning, testWorkspaceIDA, &PlanningState{
		Today: []ProjectTaskLocator{{TaskID: "backup", ProjectID: "project"}},
	})
	if err != nil {
		t.Fatalf("encode backup: %v", err)
	}

	var unsupported domainEnvelope
	if decodeErr := json.Unmarshal(goodBackup, &unsupported); decodeErr != nil {
		t.Fatalf("decode backup fixture: %v", decodeErr)
	}
	unsupported.SchemaVersion = CurrentDomainSchemaVersion + 1
	unsupportedData, err := json.MarshalIndent(unsupported, "", "  ")
	if err != nil {
		t.Fatalf("encode unsupported primary: %v", err)
	}
	mismatchedData, err := encodeDomainEnvelope(domainPlanning, testWorkspaceIDB, &PlanningState{
		Today: []ProjectTaskLocator{{TaskID: "mismatch", ProjectID: "project"}},
	})
	if err != nil {
		t.Fatalf("encode mismatched primary: %v", err)
	}
	invalidPayloadData, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   testWorkspaceIDA,
		Kind:          domainPlanning.name(),
		Data:          json.RawMessage(`"not a planning object"`),
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode invalid-payload primary: %v", err)
	}

	tests := []struct {
		name    string
		primary []byte
		check   func(error) bool
	}{
		{name: "unsupported-schema", primary: unsupportedData, check: func(err error) bool { return strings.Contains(err.Error(), "not supported") }},
		{name: "workspace-mismatch", primary: mismatchedData, check: func(err error) bool { return errors.Is(err, ErrWorkspaceIDMismatch) }},
		{name: "invalid-payload", primary: invalidPayloadData, check: func(err error) bool { return strings.Contains(err.Error(), "planning.json payload") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "workspace")
			path := PlanningStatePath(dataDir)
			if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
				t.Fatalf("mkdir domain dir: %v", err)
			}
			if err := os.WriteFile(path, tc.primary, FilePermissions); err != nil {
				t.Fatalf("write primary: %v", err)
			}
			if err := os.WriteFile(path+".bak", goodBackup, FilePermissions); err != nil {
				t.Fatalf("write backup: %v", err)
			}

			_, _, loadErr := newBoundTestDomainFileStore(t, dataDir, testWorkspaceIDA).LoadPlanningChecked()
			if loadErr == nil || !tc.check(loadErr) {
				t.Fatalf("LoadPlanningChecked error = %v, want %s primary failure", loadErr, tc.name)
			}
			primaryAfter, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read primary after load: %v", err)
			}
			if !bytes.Equal(primaryAfter, tc.primary) {
				t.Fatal("semantic primary error incorrectly triggered backup replacement")
			}
		})
	}
}

// TestDomainFileStore_UnrecoverableParseFailureNamesThePathAndANextStep keeps
// the error that stops a workspace from opening actionable: the whole path of
// the broken file, and what to do about it, beside the parse failure itself.
func TestDomainFileStore_UnrecoverableParseFailureNamesThePathAndANextStep(t *testing.T) {
	for _, tc := range []struct {
		name   string
		backup []byte
		want   string
	}{
		{name: "no-backup", want: "(no backup available)"},
		{name: "corrupt-backup", backup: []byte("{also broken"), want: "its backup is also corrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "workspace")
			path := PlanningStatePath(dataDir)
			if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
				t.Fatalf("mkdir domain dir: %v", err)
			}
			if err := os.WriteFile(path, []byte("{broken"), FilePermissions); err != nil {
				t.Fatalf("write primary: %v", err)
			}
			if tc.backup != nil {
				if err := os.WriteFile(path+".bak", tc.backup, FilePermissions); err != nil {
					t.Fatalf("write backup: %v", err)
				}
			}

			_, _, err := newBoundTestDomainFileStore(t, dataDir, testWorkspaceIDA).LoadPlanningChecked()
			if err == nil {
				t.Fatal("a corrupt planning.json with no usable backup loaded")
			}
			for _, want := range []string{
				"failed to parse " + path,
				tc.want,
				"fix the JSON in " + path + " by hand",
				PreScopeV7BackupsDir(dataDir),
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}
