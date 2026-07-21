package tui

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestDeletedProjectTagAfterReloadIsAbsentFromTagSurfaces(t *testing.T) {
	project := reloadedProjectAfterDeletingTag(t, "backend")

	if got, want := PinnedTagTabs(project), []string{"All", "#frontend"}; !slices.Equal(got, want) {
		t.Fatalf("PinnedTagTabs() = %v, want %v", got, want)
	}

	tagList := BuildProjectTagList(project, false)
	if got, want := summarizeTagInfos(tagList), []string{"frontend:2", "zero-use:0"}; !slices.Equal(got, want) {
		t.Fatalf("BuildProjectTagList() = %v, want %v", got, want)
	}

	selector := NewTagFilterSelectorModel(ShowTagFilterMsg{
		Tags:     tagList,
		Selected: map[string]bool{"backend": true, "frontend": true},
		Source:   ViewOutliner,
	})
	if slices.ContainsFunc(selector.Items, func(tag TagInfo) bool { return tag.Name == "backend" }) {
		t.Fatalf("tag filter selector items = %#v, want deleted tag omitted", selector.Items)
	}

	app := newTestAppModel()
	setTestAppProjects(app, project)
	setTestCurrentProject(app, project.ID)
	app.currentProject = project
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)

	app.outliner.filter.Tags = map[string]bool{"backend": true, "frontend": true}
	app.refreshAfterProjectTagChange("backend", "")
	if app.outliner.filter.Tags["backend"] || !app.outliner.filter.Tags["frontend"] {
		t.Fatalf("outliner tags after refresh = %v, want frontend only", app.outliner.filter.Tags)
	}

	staleFilter := config.SavedFilter{
		Name:       "Deleted Tag Snapshot",
		Mode:       "snapshot",
		FilterTags: []string{"backend", "frontend"},
	}
	app.applySavedFilterToOutliner(staleFilter)
	if app.outliner.filter.Tags["backend"] || !app.outliner.filter.Tags["frontend"] {
		t.Fatalf("outliner saved filter tags = %v, want frontend only", app.outliner.filter.Tags)
	}
	if got := app.outliner.activePinnedTagFilter(); got != "" {
		t.Fatalf("outliner activePinnedTagFilter() = %q, want empty", got)
	}
}

func reloadedProjectAfterDeletingTag(t *testing.T, tag string) *models.Project {
	t.Helper()

	dir, cleanup := testutil.TempProjectDir(t)
	t.Cleanup(cleanup)

	project := testutil.NewTestProject("Deleted Tag Surfaces")
	project.Path = filepath.Join(dir, "deleted-tag-surfaces")
	project.Tags = []string{"backend", "frontend", "zero-use"}
	project.PinnedTags = []string{"backend", "frontend"}

	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Backend and Frontend")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend", "frontend"}
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Frontend Only")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"frontend"}
	project.Tasks = []*models.Task{task1, task2}

	if err = storage.SaveProject(project); err != nil {
		t.Fatalf("initial SaveProject failed: %v", err)
	}
	if _, err = project.DeleteTag(tag); err != nil {
		t.Fatalf("DeleteTag(%q) failed: %v", tag, err)
	}
	if err = storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject after DeleteTag failed: %v", err)
	}

	reloaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	return reloaded
}
