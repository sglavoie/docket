package testutil

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

func TestMockEditorEditContentReturnsConfiguredEmptyContent(t *testing.T) {
	editor := &MockEditor{ReturnContentSet: true}

	got, err := editor.EditContent("original", ".md")
	if err != nil {
		t.Fatalf("EditContent() error = %v", err)
	}

	if got != "" {
		t.Fatalf("EditContent() = %q, want empty string", got)
	}
	if editor.EditedContent != "original" {
		t.Fatalf("EditedContent = %q, want original", editor.EditedContent)
	}
}

func TestMockEditorEditContentDefaultsToOriginalContent(t *testing.T) {
	editor := &MockEditor{}

	got, err := editor.EditContent("original", ".md")
	if err != nil {
		t.Fatalf("EditContent() error = %v", err)
	}

	if got != "original" {
		t.Fatalf("EditContent() = %q, want original", got)
	}
}

func TestMockEditorEditContentReturnsError(t *testing.T) {
	wantErr := errors.New("edit failed")
	editor := &MockEditor{EditError: wantErr}

	got, err := editor.EditContent("original", ".md")
	if !errors.Is(err, wantErr) {
		t.Fatalf("EditContent() error = %v, want %v", err, wantErr)
	}
	if got != "" {
		t.Fatalf("EditContent() = %q, want empty string on error", got)
	}
}

func TestSetupTestDepsRegistersCleanup(t *testing.T) {
	origSelector := external.DefaultSelector
	origRenderer := external.DefaultRenderer
	origEditor := external.DefaultEditor
	t.Cleanup(func() {
		external.DefaultSelector = origSelector
		external.DefaultRenderer = origRenderer
		external.DefaultEditor = origEditor
	})

	sentinelSelector := &MockSelector{}
	sentinelRenderer := &MockRenderer{}
	sentinelEditor := &MockEditor{}
	external.DefaultSelector = sentinelSelector
	external.DefaultRenderer = sentinelRenderer
	external.DefaultEditor = sentinelEditor

	var tempDir string
	t.Run("caller does not manually clean up", func(t *testing.T) {
		deps := SetupTestDeps(t, func(dataDir string) string { return dataDir })
		tempDir = deps.DataDir

		if external.DefaultSelector != deps.Selector {
			t.Fatal("SetupTestDeps() did not install mock selector")
		}
		if _, err := os.Stat(tempDir); err != nil {
			t.Fatalf("temp dir was not created: %v", err)
		}
	})

	if external.DefaultSelector != sentinelSelector ||
		external.DefaultRenderer != sentinelRenderer ||
		external.DefaultEditor != sentinelEditor {
		t.Fatal("SetupTestDeps() did not restore external defaults through t.Cleanup")
	}
	if _, err := os.Stat(tempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir still exists after t.Cleanup; stat error = %v", err)
	}
}

func TestMockRepositorySaveDeepCopiesMutableTaskFields(t *testing.T) {
	repo := NewMockRepository()
	project := NewTestProject("Test Project")
	task := NewTestTask("Original", models.TaskTypeEpic)
	baseline := time.Now()
	wantBaseline := baseline
	task.Tags = []string{"original"}
	task.DependsOn = []string{"dep-1"}
	task.RelatedTo = []string{"rel-1"}
	task.Comments = []*models.Comment{{Text: "comment"}}
	task.Decisions = []*models.Decision{{Text: "decision"}}
	task.Activity = []*models.ActivityEntry{{Field: "status", NewValue: string(models.StatusTodo)}}
	task.Checklist = []models.ChecklistItem{{Text: "step"}}
	task.Attachments = []models.Attachment{{Filename: "notes.md"}}
	task.Recurrence = &models.RecurrencePattern{
		Interval: 2,
		Unit:     models.RecurrenceWeekly,
		Weekdays: []int{1, 3},
	}
	task.ScoreDimensions = map[string]int{"impact": 3}
	task.EpicAttrs = &models.EpicAttrs{Milestone: "m1", ScopeBaselinedAt: &baseline}
	project.Tasks = []*models.Task{task}

	if err := repo.Save(project); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	task.Tags[0] = "mutated"
	task.DependsOn[0] = "dep-2"
	task.RelatedTo[0] = "rel-2"
	task.Comments[0].Text = "mutated"
	task.Decisions[0].Text = "mutated"
	task.Activity[0].NewValue = string(models.StatusDone)
	task.Checklist[0].Text = "mutated"
	task.Attachments[0].Filename = "mutated.md"
	task.Recurrence.Interval = 1
	task.Recurrence.Weekdays[0] = 5
	task.ScoreDimensions["impact"] = 5
	task.EpicAttrs.Milestone = "m2"
	*task.EpicAttrs.ScopeBaselinedAt = baseline.Add(time.Hour)

	saved := repo.Projects[project.ID].Tasks[0]
	if saved.Tags[0] != "original" ||
		saved.DependsOn[0] != "dep-1" ||
		saved.RelatedTo[0] != "rel-1" ||
		saved.Comments[0].Text != "comment" ||
		saved.Decisions[0].Text != "decision" ||
		saved.Activity[0].NewValue != string(models.StatusTodo) ||
		saved.Checklist[0].Text != "step" ||
		saved.Attachments[0].Filename != "notes.md" ||
		saved.Recurrence.Interval != 2 ||
		saved.Recurrence.Weekdays[0] != 1 ||
		saved.ScoreDimensions["impact"] != 3 ||
		saved.EpicAttrs.Milestone != "m1" ||
		!saved.EpicAttrs.ScopeBaselinedAt.Equal(wantBaseline) {
		t.Fatalf("saved task shared mutable data with original: %#v", saved)
	}
}

func TestMockRepositoryLoadDeepCopiesAndRebuildsTaskTree(t *testing.T) {
	repo := NewMockRepository()
	stored := NewTestProjectWithTasks("Stored Project")
	stored.Tasks[0].Tags = []string{"stored"}
	stored.Tasks[0].ScoreDimensions = map[string]int{"impact": 2}
	repo.Projects[stored.ID] = stored

	loaded := &models.Project{ID: stored.ID, Name: "Config Name", Slug: "config-slug", Path: "/config/path"}
	if err := repo.Load(loaded); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Tasks) < 2 {
		t.Fatalf("loaded %d tasks, want at least 2", len(loaded.Tasks))
	}
	if loaded.Tasks[1].Parent == nil || loaded.Tasks[1].Parent.ID != loaded.Tasks[0].ID {
		t.Fatalf("loaded task tree was not rebuilt from loaded task copies")
	}
	if loaded.Tasks[1].Parent == stored.Tasks[0] {
		t.Fatal("loaded task parent points at stored task")
	}
	if len(loaded.Tasks[0].Children) == 0 || loaded.Tasks[0].Children[0] == stored.Tasks[1] {
		t.Fatal("loaded task children point at stored tasks")
	}

	loaded.Tasks[0].Tags[0] = "loaded"
	loaded.Tasks[0].ScoreDimensions["impact"] = 5
	if stored.Tasks[0].Tags[0] != "stored" || stored.Tasks[0].ScoreDimensions["impact"] != 2 {
		t.Fatal("loaded task shares mutable fields with stored task")
	}
	if loaded.Name != "Config Name" || loaded.Slug != "config-slug" || loaded.Path != "/config/path" {
		t.Fatalf("Load() overwrote config-owned project metadata: %#v", loaded)
	}
	if loaded.Tasks[0].Project != loaded {
		t.Fatal("loaded task project pointer does not point at loaded project")
	}
}

func TestMockRepositoryLoadPreservesInMemoryChildLinks(t *testing.T) {
	repo := NewMockRepository()
	stored := NewTestProject("Stored Project")
	parent := NewTestTask("Parent", models.TaskTypeEpic)
	child := NewTestTask("Child", models.TaskTypeTask)
	parent.Children = []*models.Task{child}
	stored.Tasks = []*models.Task{parent, child}
	repo.Projects[stored.ID] = stored

	loaded := &models.Project{ID: stored.ID}
	if err := repo.Load(loaded); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Tasks[0].Children) != 1 || loaded.Tasks[0].Children[0] != loaded.Tasks[1] {
		t.Fatal("Load() did not preserve in-memory child links between cloned tasks")
	}
	if loaded.Tasks[1].Parent != loaded.Tasks[0] {
		t.Fatal("Load() did not preserve in-memory parent link between cloned tasks")
	}
}

func TestMockRepositoryBackupAndRestore(t *testing.T) {
	repo := NewMockRepository()
	project := NewTestProject("Backup Project")
	original := NewTestTask("Original", models.TaskTypeTask)
	project.Tasks = []*models.Task{original}
	repo.Projects[project.ID] = project

	if repo.BackupExists(project) {
		t.Fatal("BackupExists() = true before Backup")
	}
	if err := repo.Backup(project); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if !repo.BackupExists(project) {
		t.Fatal("BackupExists() = false after Backup")
	}

	mutated := NewTestTask("Mutated", models.TaskTypeTask)
	project.Tasks = []*models.Task{mutated}
	repo.Projects[project.ID] = project

	if err := repo.RestoreFromBackup(project); err != nil {
		t.Fatalf("RestoreFromBackup() error = %v", err)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].Title != "Original" {
		t.Fatalf("RestoreFromBackup() restored tasks = %#v, want original snapshot", project.Tasks)
	}
	if repo.Projects[project.ID].Tasks[0].Title != "Original" {
		t.Fatal("RestoreFromBackup() did not update stored project")
	}
	if project.Tasks[0] == original {
		t.Fatal("RestoreFromBackup() reused original task pointer")
	}
}

func TestMockRepositoryRestoreFromBackupIsReversible(t *testing.T) {
	repo := NewMockRepository()
	project := NewTestProject("Backup Project")
	project.Tasks = []*models.Task{NewTestTask("First", models.TaskTypeTask)}
	repo.Projects[project.ID] = project

	if err := repo.Backup(project); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	project.Tasks = []*models.Task{NewTestTask("Second", models.TaskTypeTask)}
	repo.Projects[project.ID] = project

	if err := repo.RestoreFromBackup(project); err != nil {
		t.Fatalf("first RestoreFromBackup() error = %v", err)
	}
	if err := repo.RestoreFromBackup(project); err != nil {
		t.Fatalf("second RestoreFromBackup() error = %v", err)
	}

	if len(project.Tasks) != 1 || project.Tasks[0].Title != "Second" {
		t.Fatalf("second RestoreFromBackup() restored tasks = %#v, want second snapshot", project.Tasks)
	}
}
