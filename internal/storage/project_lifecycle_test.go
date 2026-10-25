package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/transaction"
)

// TestPlanProjectDeleteRefusesAnUnsafeSlug proves the plan derives its path
// through containment rather than string joining, so a slug that escapes the
// projects directory is refused before any op is journaled.
func TestPlanProjectDeleteRefusesAnUnsafeSlug(t *testing.T) {
	dataDir := t.TempDir()
	for _, slug := range []string{"", "../outside", "nested/../../escape", "/absolute"} {
		if _, err := PlanProjectDelete(dataDir, slug); err == nil {
			t.Errorf("PlanProjectDelete(%q) returned no error, want a containment refusal", slug)
		}
	}
}

// TestPlanProjectDeleteStagesNothingForANeverPersistedProject covers the
// project that exists in the catalog but has no directory on disk. The delete
// still succeeds so the catalog entry can go; it simply carries no op.
func TestPlanProjectDeleteStagesNothingForANeverPersistedProject(t *testing.T) {
	dataDir := t.TempDir()
	ops, err := PlanProjectDelete(dataDir, "ghost")
	if err != nil {
		t.Fatalf("PlanProjectDelete: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %v, want none for a directory that does not exist", ops)
	}
}

// TestPlanProjectDeleteCoversTheWholeProjectDirectoryChildrenFirst pins the op
// set that takes tasks.md, notes, and attachments together. OpDelete is
// os.Remove, so a directory must be named after everything inside it or the
// journal refuses it as non-empty.
func TestPlanProjectDeleteCoversTheWholeProjectDirectoryChildrenFirst(t *testing.T) {
	dataDir := t.TempDir()
	projectDir := filepath.Join(dataDir, "projects", "alpha")
	files := []string{
		filepath.Join(projectDir, "tasks.md"),
		filepath.Join(projectDir, "notes", "design.md"),
		filepath.Join(projectDir, ".attachments", "spec.txt"),
	}
	for _, path := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	ops, err := PlanProjectDelete(dataDir, "alpha")
	if err != nil {
		t.Fatalf("PlanProjectDelete: %v", err)
	}

	position := make(map[string]int, len(ops))
	for index, op := range ops {
		if op.Kind != transaction.OpDelete {
			t.Errorf("op %d = %+v, want a delete", index, op)
		}
		// A delete names a target and stages nothing: bytes would be a second
		// source of truth for a path the undo ledger already imaged.
		if op.Source != "" || len(op.Data) != 0 {
			t.Errorf("op %d = %+v, want no source or staged content", index, op)
		}
		position[op.Target] = index
	}
	for _, path := range append(files,
		filepath.Join(projectDir, "notes"),
		filepath.Join(projectDir, ".attachments"),
		projectDir,
	) {
		if _, planned := position[path]; !planned {
			t.Errorf("plan omits %s", path)
		}
	}
	for _, path := range files {
		if position[path] > position[filepath.Dir(path)] {
			t.Errorf("%s is deleted after its directory, which cannot then be empty", path)
		}
	}
	if last := len(ops) - 1; position[projectDir] != last {
		t.Errorf("project directory is op %d, want the last one (%d)", position[projectDir], last)
	}
	// Planning is pure: nothing may leave the disk before the caller commits.
	if _, statErr := os.Stat(projectDir); statErr != nil {
		t.Errorf("planning removed %s, want the directory untouched: %v", projectDir, statErr)
	}
}
