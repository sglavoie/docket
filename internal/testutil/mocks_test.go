package testutil

import (
	"errors"
	"testing"
	"time"

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
