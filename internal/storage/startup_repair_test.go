package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func seedStartupRepairProject(t *testing.T) (*models.Project, []byte) {
	t.Helper()
	dataDir := t.TempDir()
	project := testutil.NewTestProject("Startup Repair")
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	legacyTaskSectionField := "custom" + "_section"
	content := []byte("# Project: Startup Repair\n" +
		"<!-- sections: Next;Sprint-1 -->\n\n" +
		"## Task: Legacy dual section\n" +
		"<!-- id: task-1, status: todo, priority: 1, section: Next, " + legacyTaskSectionField + ": Sprint-1 -->\n")
	if err := os.WriteFile(tasksFilePath(project.Path), content, FilePermissions); err != nil {
		t.Fatalf("WriteFile(tasks.md): %v", err)
	}
	return project, content
}

func TestStartupRepairCommitUsesObservedRawRevision(t *testing.T) {
	project, original := seedStartupRepairProject(t)
	load, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings: %v", err)
	}
	if len(load.Warnings) == 0 || load.RawRevision == nil {
		t.Fatalf("load result = %+v, want repair warning and raw revision", load)
	}
	if got, want := *load.RawRevision, RawArtifactRevision(original); got != want {
		t.Fatalf("raw revision = %q, want %q", got, want)
	}

	dataDir := filepath.Dir(filepath.Dir(project.Path))
	txnID, err := NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}).
		CommitProjectRepair(project, *load.RawRevision)
	if err != nil {
		t.Fatalf("CommitProjectRepair: %v", err)
	}
	if txnID == "" {
		t.Fatal("CommitProjectRepair returned an empty transaction ID")
	}

	reloaded := testutil.NewTestProject("Startup Repair")
	reloaded.ID = project.ID
	reloaded.Path = project.Path
	after, err := LoadProjectWithWarnings(reloaded)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings(after repair): %v", err)
	}
	if len(after.Warnings) != 0 {
		t.Fatalf("warnings after repair = %v, want none", after.Warnings)
	}
	data, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("ReadFile(tasks.md): %v", err)
	}
	if !strings.Contains(string(data), "<!-- tt-schema: 2 -->") {
		t.Fatalf("repair did not write canonical schema-v2 bytes:\n%s", data)
	}
}

func TestStartupRepairCommitFailureLeavesProjectBytesUnchanged(t *testing.T) {
	project, original := seedStartupRepairProject(t)
	load, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings: %v", err)
	}
	if load.RawRevision == nil {
		t.Fatal("load did not retain a raw revision")
	}

	dataDir := filepath.Dir(filepath.Dir(project.Path))
	transactionsPath := filepath.Join(dataDir, "transactions-is-a-file")
	if err := os.WriteFile(transactionsPath, []byte("not a directory"), FilePermissions); err != nil {
		t.Fatalf("seed invalid transactions path: %v", err)
	}
	if _, err := NewCommitService(transactionsPath, []string{dataDir}).CommitProjectRepair(project, *load.RawRevision); err == nil {
		t.Fatal("CommitProjectRepair succeeded with an invalid journal directory")
	}
	after, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("ReadFile(tasks.md): %v", err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("failed repair changed project bytes\n got: %q\nwant: %q", after, original)
	}
}

func TestStartupRepairCommitRejectsStaleRawRevision(t *testing.T) {
	project, _ := seedStartupRepairProject(t)
	load, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("LoadProjectWithWarnings: %v", err)
	}
	if load.RawRevision == nil {
		t.Fatal("load did not retain a raw revision")
	}

	concurrent := testutil.NewTestProject("Startup Repair")
	concurrent.ID = project.ID
	concurrent.Path = project.Path
	if err := LoadProject(concurrent); err != nil {
		t.Fatalf("LoadProject(concurrent): %v", err)
	}
	base, err := LoadProjectDocument(concurrent)
	if err != nil {
		t.Fatalf("LoadProjectDocument(concurrent): %v", err)
	}
	concurrent.Tasks[0].Title = "CLI mutation survived"
	dataDir := filepath.Dir(filepath.Dir(project.Path))
	svc := NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
	if _, err := svc.Commit(ChangeSet{Project: concurrent, ExpectedRevision: base.Revision, Base: base}); err != nil {
		t.Fatalf("concurrent CLI-style Commit: %v", err)
	}
	cliBytes, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("ReadFile(after concurrent commit): %v", err)
	}

	_, err = svc.CommitProjectRepair(project, *load.RawRevision)
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("stale CommitProjectRepair error = %v, want ErrStaleArtifact", err)
	}
	after, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("ReadFile(after rejected repair): %v", err)
	}
	if !bytes.Equal(after, cliBytes) {
		t.Fatalf("stale repair overwrote concurrent bytes\n got: %q\nwant: %q", after, cliBytes)
	}
}
