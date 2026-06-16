package tui

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectDiscoveryProjectsIncludesArchivedAfterActive(t *testing.T) {
	cfg := testutil.NewTestConfig()
	active := testutil.NewTestProject("Active Project")
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	sandbox := testutil.NewTestProject("Sandbox Project")
	sandbox.Sandbox = true
	sandbox.Archive()
	cfg.Projects = []*models.Project{archived, sandbox, active}

	projects := projectDiscoveryProjects(cfg)
	if len(projects) != 2 {
		t.Fatalf("projectDiscoveryProjects len = %d, want 2", len(projects))
	}
	if projects[0] != active {
		t.Fatalf("first discovery project = %q, want active project", projects[0].Name)
	}
	if projects[1] != archived {
		t.Fatalf("second discovery project = %q, want archived project", projects[1].Name)
	}
}

func TestProjectPickerDisplaysAndFiltersArchivedProjects(t *testing.T) {
	active := testutil.NewTestProject("Zoo Active")
	archived := testutil.NewTestProject("SIS Shield")
	archived.Context = "work"
	archived.Archive()

	picker := NewProjectPickerModel([]*models.Project{active, archived})
	picker.SetSize(80, 18)
	if picker.Items[0].Project != active {
		t.Fatalf("first picker item = %q, want active project before archived project", picker.Items[0].Project.Name)
	}

	picker.Filter("sis shield")
	if len(picker.Filtered) != 1 || picker.Filtered[0].Project != archived {
		t.Fatalf("name filter returned %#v, want archived project", picker.Filtered)
	}

	picker.Filter("archived")

	if len(picker.Filtered) != 1 || picker.Filtered[0].Project != archived {
		t.Fatalf("archived filter returned %#v, want archived project", picker.Filtered)
	}
	if !strings.Contains(picker.Filtered[0].DisplayText(), archivedProjectMarker) {
		t.Fatalf("DisplayText() = %q, want archived marker", picker.Filtered[0].DisplayText())
	}
	if rendered := picker.renderList(80); !strings.Contains(rendered, archivedProjectMarker) {
		t.Fatalf("renderList() missing archived marker:\n%s", rendered)
	}
}

func TestShowProjectPickerIncludesArchivedProjects(t *testing.T) {
	app := newTestAppModel()
	active := testutil.NewTestProject("Active Project")
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	app.config.Projects = []*models.Project{active, archived}

	handled, _ := app.handleProjectMessages(ShowProjectPickerMsg{})
	if !handled {
		t.Fatal("ShowProjectPickerMsg should be handled")
	}
	picker, ok := app.views.Get(ViewProjectPicker).(*ProjectPickerModel)
	if !ok {
		t.Fatalf("project picker view = %T, want *ProjectPickerModel", app.views.Get(ViewProjectPicker))
	}

	var names []string
	for _, item := range picker.Items {
		names = append(names, item.Project.Name)
	}
	if !slices.Contains(names, "SIS Shield") {
		t.Fatalf("project picker names = %v, want archived project", names)
	}
}

func TestCommandPaletteIncludesArchivedProjectButNotArchivedTasks(t *testing.T) {
	cfg := testutil.NewTestConfig()
	active := testutil.NewTestProjectWithTasks("Active Project")
	archived := testutil.NewTestProjectWithTasks("SIS Shield")
	active.Path = filepath.Join(t.TempDir(), active.Slug)
	archived.Path = filepath.Join(t.TempDir(), archived.Slug)
	if err := storage.SaveProject(active); err != nil {
		t.Fatalf("SaveProject active: %v", err)
	}
	if err := storage.SaveProject(archived); err != nil {
		t.Fatalf("SaveProject archived: %v", err)
	}
	archived.Archive()
	cfg.Projects = []*models.Project{active, archived}

	palette := NewCommandPaletteModel(cfg, nil, NewCommandRegistry(), ViewDashboard, nil)

	var foundArchivedProject, foundActiveTask, foundArchivedTask bool
	for _, item := range palette.allItems {
		if item.Category == ItemCategoryProjects && item.Project == archived {
			foundArchivedProject = true
			if !strings.Contains(item.Description, archivedProjectMarker) {
				t.Fatalf("archived project description = %q, want marker", item.Description)
			}
		}
		if item.Category == ItemCategoryTasks && item.Project == active {
			foundActiveTask = true
		}
		if item.Category == ItemCategoryTasks && item.Project == archived {
			foundArchivedTask = true
		}
	}

	if !foundArchivedProject {
		t.Fatal("command palette missing archived project row")
	}
	if !foundActiveTask {
		t.Fatal("command palette should still load active project tasks")
	}
	if foundArchivedTask {
		t.Fatal("command palette should not load archived project tasks")
	}

	palette.searchInput.SetValue("sis shield")
	palette.filterItems()
	if !slices.ContainsFunc(palette.filteredItems, func(item filteredPaletteItem) bool {
		return item.Item.Category == ItemCategoryProjects && item.Item.Project == archived
	}) {
		t.Fatal("command palette search should find archived project by name")
	}
}
