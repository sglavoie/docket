package models

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDedupeTagsNormalizesValidatesAndDedupes(t *testing.T) {
	got := DedupeTags([]string{
		" backend ",
		"#frontend",
		"backend",
		"has,comma",
		"",
		"frontend",
		"  #urgent  ",
	})
	want := []string{"backend", "frontend", "urgent"}
	if !slices.Equal(got, want) {
		t.Fatalf("DedupeTags() = %v, want %v", got, want)
	}
}

func TestProjectAddTagNormalizesAndPreventsDuplicates(t *testing.T) {
	project := newProjectTagsTestProject(t)

	added, err := project.AddTag("  #backend  ")
	if err != nil {
		t.Fatalf("AddTag() error = %v", err)
	}
	if !added {
		t.Fatal("AddTag() added = false, want true")
	}
	if !slices.Equal(project.Tags, []string{"backend"}) {
		t.Fatalf("project.Tags = %v, want [backend]", project.Tags)
	}
	if !project.HasCatalogTag("#backend") {
		t.Fatal("HasCatalogTag(#backend) = false, want true")
	}
	if !project.HasTag("backend") {
		t.Fatal("HasTag(backend) = false, want true")
	}

	added, err = project.AddTag("backend")
	if err != nil {
		t.Fatalf("duplicate AddTag() error = %v", err)
	}
	if added {
		t.Fatal("duplicate AddTag() added = true, want false")
	}
	if !slices.Equal(project.Tags, []string{"backend"}) {
		t.Fatalf("project.Tags after duplicate = %v, want [backend]", project.Tags)
	}
}

func TestProjectAddTagRejectsInvalidTags(t *testing.T) {
	project := newProjectTagsTestProject(t)
	tests := []string{
		"",
		"   ",
		"#",
		"has,comma",
		"has;semicolon",
		"has\nnewline",
		strings.Repeat("a", MaxTagLength+1),
	}

	for _, tag := range tests {
		t.Run(tag, func(t *testing.T) {
			added, err := project.AddTag(tag)
			if err == nil {
				t.Fatalf("AddTag(%q) error = nil, want error", tag)
			}
			if added {
				t.Fatalf("AddTag(%q) added = true, want false", tag)
			}
			if len(project.Tags) != 0 {
				t.Fatalf("project.Tags = %v, want empty", project.Tags)
			}
		})
	}
}

func TestProjectAllTagsIncludesZeroUseCatalogTagsAndTaskTags(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"zero-use", "backend", "backend"}

	task1 := newProjectTagsTestTask(t, "t1", 1)
	task1.Tags = []string{"frontend", "backend", "backend"}
	task2 := newProjectTagsTestTask(t, "t2", 2)
	task2.Tags = []string{"ops"}
	project.Tasks = []*Task{task1, task2}

	wantTags := []string{"backend", "frontend", "ops", "zero-use"}
	if got := project.AllTags(); !slices.Equal(got, wantTags) {
		t.Fatalf("AllTags() = %v, want %v", got, wantTags)
	}

	counts := project.TagCounts()
	if counts["zero-use"] != 0 {
		t.Fatalf("zero-use count = %d, want 0", counts["zero-use"])
	}
	if counts["backend"] != 1 {
		t.Fatalf("backend count = %d, want 1", counts["backend"])
	}
}

func TestProjectRemoveTagKeepsTaskUsageInAllTags(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"backend"}
	task := newProjectTagsTestTask(t, "t1", 1)
	task.AddTag("backend")
	project.Tasks = []*Task{task}

	if !project.RemoveTag("#backend") {
		t.Fatal("RemoveTag(#backend) = false, want true")
	}
	if project.HasCatalogTag("backend") {
		t.Fatal("HasCatalogTag(backend) = true, want false")
	}
	if !project.HasTag("backend") {
		t.Fatal("HasTag(backend) = false, want true because task still uses it")
	}
	if got := project.AllTags(); !slices.Equal(got, []string{"backend"}) {
		t.Fatalf("AllTags() = %v, want [backend]", got)
	}
}

func TestProjectRenameTagDedupeWhenTaskAlreadyHasDestination(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"old"}
	task := newProjectTagsTestTask(t, "t1", 1)
	task.Tags = []string{"old", "new"}
	project.Tasks = []*Task{task}

	changed, err := project.RenameTag("#old", " new ")
	if err != nil {
		t.Fatalf("RenameTag() error = %v", err)
	}
	if !changed {
		t.Fatal("RenameTag() changed = false, want true")
	}
	if !slices.Equal(task.Tags, []string{"new"}) {
		t.Fatalf("task.Tags = %v, want [new]", task.Tags)
	}
	if project.HasCatalogTag("old") {
		t.Fatal("old catalog tag still present")
	}
	if got := project.AllTags(); !slices.Equal(got, []string{"new"}) {
		t.Fatalf("AllTags() = %v, want [new]", got)
	}
}

func TestProjectRenameZeroUseCatalogTag(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"old"}
	project.PinnedTags = []string{"old"}

	changed, err := project.RenameTag("old", "#new")
	if err != nil {
		t.Fatalf("RenameTag() error = %v", err)
	}
	if !changed {
		t.Fatal("RenameTag() changed = false, want true")
	}
	if !slices.Equal(project.Tags, []string{"new"}) {
		t.Fatalf("project.Tags = %v, want [new]", project.Tags)
	}
	if !slices.Equal(project.PinnedTags, []string{"new"}) {
		t.Fatalf("project.PinnedTags = %v, want [new]", project.PinnedTags)
	}
}

func TestProjectRemoveZeroUseCatalogTagUnpinsOnlyWhenNoTasksUseTag(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"unused", "used"}
	project.PinnedTags = []string{"unused", "used"}
	task := newProjectTagsTestTask(t, "t1", 1)
	task.Tags = []string{"used"}
	project.Tasks = []*Task{task}

	if !project.RemoveTag("unused") {
		t.Fatal("RemoveTag(unused) = false, want true")
	}
	if project.IsTagPinned("unused") {
		t.Fatal("unused should be unpinned after deleting zero-use catalog tag")
	}

	if !project.RemoveTag("used") {
		t.Fatal("RemoveTag(used) = false, want true")
	}
	if !project.IsTagPinned("used") {
		t.Fatal("used should remain pinned while tasks still use it")
	}
}

func TestProjectPinnedTagHelpersNormalizeValidateAndToggle(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.PinnedTags = []string{" backend ", "bad,tag", "backend"}

	if !project.IsTagPinned("#backend") {
		t.Fatal("IsTagPinned(#backend) = false, want true")
	}
	if got, want := project.PinnedTagList(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("PinnedTagList() = %v, want %v", got, want)
	}

	added, err := project.PinTag(" #urgent ")
	if err != nil {
		t.Fatalf("PinTag() error = %v", err)
	}
	if !added {
		t.Fatal("PinTag() added = false, want true")
	}
	if got, want := project.PinnedTags, []string{"backend", "urgent"}; !slices.Equal(got, want) {
		t.Fatalf("project.PinnedTags = %v, want %v", got, want)
	}

	added, err = project.PinTag("urgent")
	if err != nil {
		t.Fatalf("duplicate PinTag() error = %v", err)
	}
	if added {
		t.Fatal("duplicate PinTag() added = true, want false")
	}

	pinned, err := project.TogglePinnedTag("urgent")
	if err != nil {
		t.Fatalf("TogglePinnedTag(unpin) error = %v", err)
	}
	if pinned || project.IsTagPinned("urgent") {
		t.Fatal("TogglePinnedTag(urgent) should unpin urgent")
	}

	pinned, err = project.TogglePinnedTag("#frontend")
	if err != nil {
		t.Fatalf("TogglePinnedTag(pin) error = %v", err)
	}
	if !pinned || !project.IsTagPinned("frontend") {
		t.Fatal("TogglePinnedTag(frontend) should pin frontend")
	}

	if !project.UnpinTag("#backend") {
		t.Fatal("UnpinTag(#backend) = false, want true")
	}
	if project.IsTagPinned("backend") {
		t.Fatal("backend should not remain pinned")
	}

	if _, err := project.PinTag("bad,tag"); err == nil {
		t.Fatal("PinTag(bad,tag) error = nil, want error")
	}
}

func TestProjectTagsAreNotConfigJSON(t *testing.T) {
	project := newProjectTagsTestProject(t)
	project.Tags = []string{"backend"}
	project.PinnedTags = []string{"urgent"}

	data, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("json.Marshal(Project) error = %v", err)
	}
	if strings.Contains(string(data), "backend") || strings.Contains(string(data), "urgent") || strings.Contains(string(data), "tags") {
		t.Fatalf("project tags should not be marshaled in config JSON: %s", data)
	}
}

func newProjectTagsTestProject(t *testing.T) *Project {
	t.Helper()
	project, err := NewProject("test", "Test Project", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func newProjectTagsTestTask(t *testing.T, id string, shortID int) *Task {
	t.Helper()
	task, err := NewTask(id, shortID, TaskTypeTask, "Test task")
	if err != nil {
		t.Fatal(err)
	}
	return task
}
