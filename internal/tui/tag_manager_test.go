package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestTagManagerBuildListIncludesCatalogOnlyTags(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	project.Tasks = []*models.Task{newTagManagerTestTask(t, "1", 1, "frontend")}

	m := NewTagManagerModelForProject(project)

	got := summarizeTagInfos(m.tags)
	want := []string{"backend:0", "frontend:1"}
	if !slices.Equal(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
}

func TestTagManagerBuildListMarksPinnedTags(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend", "frontend"}
	project.PinnedTags = []string{"frontend"}

	m := NewTagManagerModelForProject(project)

	if m.tags[0].Name != "backend" || m.tags[0].Pinned {
		t.Fatalf("first tag = %#v, want unpinned backend", m.tags[0])
	}
	if m.tags[1].Name != "frontend" || !m.tags[1].Pinned {
		t.Fatalf("second tag = %#v, want pinned frontend", m.tags[1])
	}
}

func TestTagManagerViewShowsPinnedMarker(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	project.PinnedTags = []string{"backend"}
	m := NewTagManagerModelForProject(project)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "* backend") {
		t.Fatalf("tag manager view missing pinned marker:\n%s", plain)
	}
}

func TestTagManagerAddCatalogTagPersists(t *testing.T) {
	project := newTagManagerTestProject(t)
	m := NewTagManagerModelForProject(project)

	msg := m.addProjectTag("  #backend  ")()
	if _, ok := msg.(TagAddedMsg); !ok {
		t.Fatalf("addProjectTag() message = %T, want TagAddedMsg (%v)", msg, msg)
	}
	updated, _ := m.Update(msg)
	m = updated.(*TagManagerModel)

	if got, want := project.Tags, []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("project.Tags = %v, want %v", got, want)
	}
	assertReloadedTagCatalog(t, project, []string{"backend"})

	got := summarizeTagInfos(m.tags)
	want := []string{"backend:0"}
	if !slices.Equal(got, want) {
		t.Fatalf("manager tags = %v, want %v", got, want)
	}
}

func TestTagManagerTogglePinnedTagPersists(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	saveAndReloadTagManagerProject(t, project)
	m := NewTagManagerModelForProject(project)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = updated.(*TagManagerModel)
	if cmd == nil {
		t.Fatal("p key returned nil command, want pin toggle")
	}
	msg := cmd()
	pinned, ok := msg.(TagPinnedMsg)
	if !ok {
		t.Fatalf("p key message = %T, want TagPinnedMsg (%v)", msg, msg)
	}
	if pinned.Name != "backend" || !pinned.Pinned {
		t.Fatalf("TagPinnedMsg = %#v, want backend pinned", pinned)
	}
	updated, _ = m.Update(msg)
	m = updated.(*TagManagerModel)

	if !project.IsTagPinned("backend") {
		t.Fatal("backend should be pinned")
	}
	if len(m.tags) != 1 || !m.tags[0].Pinned {
		t.Fatalf("manager tags = %#v, want backend pinned", m.tags)
	}
	if got, want := reloadTagManagerProject(t, project).PinnedTagList(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("reloaded pinned tags = %v, want %v", got, want)
	}

	msg = m.togglePinnedTag("backend")()
	pinned, ok = msg.(TagPinnedMsg)
	if !ok {
		t.Fatalf("second toggle message = %T, want TagPinnedMsg (%v)", msg, msg)
	}
	if pinned.Pinned {
		t.Fatalf("second toggle = %#v, want unpinned", pinned)
	}
	if project.IsTagPinned("backend") {
		t.Fatal("backend should be unpinned")
	}
}

func TestTagManagerRemoveZeroUseCatalogTagPersists(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	project.PinnedTags = []string{"backend"}
	saveAndReloadTagManagerProject(t, project)
	m := NewTagManagerModelForProject(project)

	msg := m.removeProjectTag("backend")()
	if _, ok := msg.(TagRemovedMsg); !ok {
		t.Fatalf("removeProjectTag() message = %T, want TagRemovedMsg (%v)", msg, msg)
	}
	updated, _ := m.Update(msg)
	m = updated.(*TagManagerModel)

	if len(project.Tags) != 0 {
		t.Fatalf("project.Tags = %v, want empty", project.Tags)
	}
	if len(project.PinnedTags) != 0 {
		t.Fatalf("project.PinnedTags = %v, want empty", project.PinnedTags)
	}
	assertReloadedTagCatalog(t, project, nil)
	assertReloadedPinnedTags(t, project, nil)
	if len(m.tags) != 0 {
		t.Fatalf("manager tags = %v, want empty", m.tags)
	}
}

func TestTagManagerDeleteProtectsTaskUsedTag(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	project.Tasks = []*models.Task{newTagManagerTestTask(t, "1", 1, "backend")}
	m := NewTagManagerModelForProject(project)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = updated.(*TagManagerModel)

	if cmd != nil {
		t.Fatal("delete task-used tag returned a command, want inline validation error")
	}
	if m.errorMsg != "cannot delete: tag has 1 task(s)" {
		t.Fatalf("errorMsg = %q, want task-use protection", m.errorMsg)
	}
	if got, want := project.Tags, []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("project.Tags = %v, want %v", got, want)
	}
}

func TestTagManagerRenameCatalogOnlyTagPersists(t *testing.T) {
	project := newTagManagerTestProject(t)
	project.Tags = []string{"backend"}
	project.PinnedTags = []string{"backend"}
	saveAndReloadTagManagerProject(t, project)
	m := NewTagManagerModelForProject(project)

	msg := m.renameTag("backend", "frontend")()
	renamed, ok := msg.(TagRenamedMsg)
	if !ok {
		t.Fatalf("renameTag() message = %T, want TagRenamedMsg (%v)", msg, msg)
	}
	if renamed.OldName != "backend" || renamed.NewName != "frontend" {
		t.Fatalf("rename message = %#v, want backend -> frontend", renamed)
	}
	updated, _ := m.Update(msg)
	m = updated.(*TagManagerModel)

	if got, want := project.Tags, []string{"frontend"}; !slices.Equal(got, want) {
		t.Fatalf("project.Tags = %v, want %v", got, want)
	}
	if got, want := project.PinnedTags, []string{"frontend"}; !slices.Equal(got, want) {
		t.Fatalf("project.PinnedTags = %v, want %v", got, want)
	}
	assertReloadedTagCatalog(t, project, []string{"frontend"})
	assertReloadedPinnedTags(t, project, []string{"frontend"})

	got := summarizeTagInfos(m.tags)
	want := []string{"frontend:0"}
	if !slices.Equal(got, want) {
		t.Fatalf("manager tags = %v, want %v", got, want)
	}
}

func TestTagManagerRenameTaskUsedTagPersistsAndDedupes(t *testing.T) {
	project := newTagManagerTestProject(t)
	task1 := newTagManagerTestTask(t, "1", 1, "frontend", "backend")
	task2 := newTagManagerTestTask(t, "2", 2, "frontend")
	task3 := newTagManagerTestTask(t, "3", 3, "backend")
	project.Tasks = []*models.Task{task1, task2, task3}
	saveAndReloadTagManagerProject(t, project)
	m := NewTagManagerModelForProject(project)

	msg := m.renameTag("frontend", "backend")()
	if _, ok := msg.(TagRenamedMsg); !ok {
		t.Fatalf("renameTag() message = %T, want TagRenamedMsg (%v)", msg, msg)
	}
	updated, _ := m.Update(msg)
	m = updated.(*TagManagerModel)

	if got, want := project.Tasks[0].Tags, []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("task1 tags = %v, want %v", got, want)
	}
	if got, want := project.Tasks[1].Tags, []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("task2 tags = %v, want %v", got, want)
	}
	if got, want := project.Tasks[2].Tags, []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("task3 tags = %v, want %v", got, want)
	}
	assertReloadedTaskTags(t, project, map[int][]string{
		1: {"backend"},
		2: {"backend"},
		3: {"backend"},
	})

	got := summarizeTagInfos(m.tags)
	want := []string{"backend:3"}
	if !slices.Equal(got, want) {
		t.Fatalf("manager tags = %v, want %v", got, want)
	}
}

func newTagManagerTestProject(t *testing.T) *models.Project {
	t.Helper()
	project, err := models.NewProject("test", "Test", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func newTagManagerTestTask(t *testing.T, id string, shortID int, tags ...string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, models.TaskTypeTask, "Task "+id)
	if err != nil {
		t.Fatal(err)
	}
	task.Tags = tags
	return task
}

func saveAndReloadTagManagerProject(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
}

func assertReloadedTagCatalog(t *testing.T, project *models.Project, want []string) {
	t.Helper()
	reloaded := reloadTagManagerProject(t, project)
	if !slices.Equal(reloaded.Tags, want) {
		t.Fatalf("reloaded tags = %v, want %v", reloaded.Tags, want)
	}
}

func assertReloadedPinnedTags(t *testing.T, project *models.Project, want []string) {
	t.Helper()
	reloaded := reloadTagManagerProject(t, project)
	if !slices.Equal(reloaded.PinnedTagList(), want) {
		t.Fatalf("reloaded pinned tags = %v, want %v", reloaded.PinnedTagList(), want)
	}
}

func assertReloadedTaskTags(t *testing.T, project *models.Project, want map[int][]string) {
	t.Helper()
	reloaded := reloadTagManagerProject(t, project)
	for _, task := range reloaded.Tasks {
		if expected, ok := want[task.ShortID]; ok && !slices.Equal(task.Tags, expected) {
			t.Fatalf("reloaded task #%d tags = %v, want %v", task.ShortID, task.Tags, expected)
		}
	}
}

func reloadTagManagerProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	reloaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject(reloaded) error = %v", err)
	}
	return reloaded
}
