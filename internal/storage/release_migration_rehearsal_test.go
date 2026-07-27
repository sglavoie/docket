package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestReleaseMigrationRehearsalUsesEveryLegacyProjectFixture(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "legacy_workspace", "*.tasks.md"))
	if err != nil {
		t.Fatalf("glob legacy project fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no legacy project fixtures found")
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			legacy, readErr := os.ReadFile(fixture)
			if readErr != nil {
				t.Fatalf("read legacy fixture: %v", readErr)
			}
			legacyDoc, parseErr := ParseProjectDocument(string(legacy))
			if parseErr != nil {
				t.Fatalf("parse legacy fixture: %v", parseErr)
			}

			projectPath := t.TempDir()
			copiedPath := tasksFilePath(projectPath)
			if writeErr := os.WriteFile(copiedPath, legacy, FilePermissions); writeErr != nil {
				t.Fatalf("copy legacy fixture: %v", writeErr)
			}
			project := &models.Project{
				ID:   strings.TrimSuffix(filepath.Base(fixture), ".tasks.md"),
				Name: legacyDoc.ProjectName,
				Path: projectPath,
			}
			load, loadErr := LoadProjectWithWarnings(project)
			if loadErr != nil {
				t.Fatalf("load copied legacy fixture: %v", loadErr)
			}
			if load.MigrationAudit == nil || len(load.MigrationAudit.Entries) != len(legacyDoc.Tasks) {
				t.Fatalf("migration audit entries = %+v, want %d", load.MigrationAudit, len(legacyDoc.Tasks))
			}
			wantStates := taskStatesByID(legacyDoc.Tasks)
			if got := taskStatesByID(project.Tasks); !equalTaskStates(got, wantStates) {
				t.Fatalf("loaded states = %v, want %v", got, wantStates)
			}

			if saveErr := SaveProject(project); saveErr != nil {
				t.Fatalf("canonical save: %v", saveErr)
			}
			canonical, readErr := os.ReadFile(copiedPath)
			if readErr != nil {
				t.Fatalf("read canonical copy: %v", readErr)
			}
			if !bytes.Contains(canonical, []byte("<!-- tt-schema: 2 -->")) {
				t.Fatal("canonical copy is missing the schema-v2 marker")
			}
			for _, retired := range []string{"status:", "section:", "stage:", "active:", "<!-- sections:"} {
				if bytes.Contains(canonical, []byte(retired)) {
					t.Fatalf("canonical copy retained %q", retired)
				}
			}
			canonicalDoc, parseErr := ParseProjectDocument(string(canonical))
			if parseErr != nil {
				t.Fatalf("parse canonical copy: %v", parseErr)
			}
			if got := taskStatesByID(canonicalDoc.Tasks); !equalTaskStates(got, wantStates) {
				t.Fatalf("canonical states = %v, want %v", got, wantStates)
			}

			if saveErr := SaveProject(project); saveErr != nil {
				t.Fatalf("idempotent canonical save: %v", saveErr)
			}
			again, readErr := os.ReadFile(copiedPath)
			if readErr != nil {
				t.Fatalf("read idempotent copy: %v", readErr)
			}
			if !bytes.Equal(again, canonical) {
				t.Fatal("second canonical save changed the migrated project bytes")
			}
		})
	}
}

func TestReleaseMigrationRehearsalInvalidProjectRollsBack(t *testing.T) {
	projectPath := t.TempDir()
	path := tasksFilePath(projectPath)
	invalid := []byte("# Project: Invalid\n\n" + strings.Repeat("x", 2*1024*1024) + "\n")
	if err := os.WriteFile(path, invalid, FilePermissions); err != nil {
		t.Fatalf("write invalid legacy copy: %v", err)
	}
	sentinel := &models.Task{ID: "sentinel", Title: "Unchanged"}
	project := &models.Project{
		ID:    "invalid",
		Name:  "Invalid",
		Path:  projectPath,
		Tasks: []*models.Task{sentinel},
	}

	if _, err := LoadProjectWithWarnings(project); err == nil {
		t.Fatal("invalid legacy copy unexpectedly loaded")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rejected legacy copy: %v", err)
	}
	if !bytes.Equal(got, invalid) {
		t.Fatal("failed migration changed the invalid source bytes")
	}
	if len(project.Tasks) != 1 || project.Tasks[0] != sentinel {
		t.Fatalf("failed migration changed the in-memory project: %+v", project.Tasks)
	}
}

func taskStatesByID(tasks []*models.Task) map[string]models.WorkflowState {
	states := make(map[string]models.WorkflowState, len(tasks))
	for _, task := range tasks {
		if task != nil {
			states[task.ID] = task.State()
		}
	}
	return states
}

func equalTaskStates(left, right map[string]models.WorkflowState) bool {
	if len(left) != len(right) {
		return false
	}
	for id, state := range left {
		if right[id] != state {
			return false
		}
	}
	return true
}
