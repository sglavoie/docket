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
	cfg := newTestConfig()
	active := testutil.NewTestProject("Active Project")
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	sandbox := testutil.NewTestProject("Sandbox Project")
	sandbox.Sandbox = true
	sandbox.Archive()
	setTestProjects(cfg, []*models.Project{archived, sandbox, active})

	// Discovery now reads the domain-native catalog snapshot, which rehydrates
	// fresh project pointers (paths derived from DataDir), so assert by name.
	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	projects := projectDiscoveryProjects(runtime)
	if len(projects) != 2 {
		t.Fatalf("projectDiscoveryProjects len = %d, want 2", len(projects))
	}
	if projects[0].Name != active.Name {
		t.Fatalf("first discovery project = %q, want active project", projects[0].Name)
	}
	if projects[1].Name != archived.Name {
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
	setTestAppProjects(app, active, archived)
	syncTestRuntime(app)

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
	cfg := newTestConfig()
	// Root the workspace at one dir so catalog-derived paths (DataDir/projects/
	// <slug>) match where the projects are saved and the palette can load tasks.
	dataDir := t.TempDir()
	cfg = testRuntimeAt(dataDir, cfg)
	active := testutil.NewTestProjectWithTasks("Active Project")
	archived := testutil.NewTestProjectWithTasks("SIS Shield")
	active.Path = filepath.Join(dataDir, "projects", active.Slug)
	archived.Path = filepath.Join(dataDir, "projects", archived.Slug)
	if err := storage.SaveProject(active); err != nil {
		t.Fatalf("SaveProject active: %v", err)
	}
	if err := storage.SaveProject(archived); err != nil {
		t.Fatalf("SaveProject archived: %v", err)
	}
	archived.Archive()
	setTestProjects(cfg, []*models.Project{active, archived})

	palette := NewCommandPaletteModel(testRuntimeAt(cfg.DataDir(), cfg), NewCommandRegistry(), ViewDashboard, nil)

	// Discovery rows carry rehydrated catalog project pointers, so match by ID.
	var foundArchivedProject, foundActiveTask, foundArchivedTask bool
	for _, item := range palette.allItems {
		if item.Category == ItemCategoryProjects && item.Project != nil && item.Project.ID == archived.ID {
			foundArchivedProject = true
			if !strings.Contains(item.Description, archivedProjectMarker) {
				t.Fatalf("archived project description = %q, want marker", item.Description)
			}
		}
		if item.Category == ItemCategoryTasks && item.Project != nil && item.Project.ID == active.ID {
			foundActiveTask = true
		}
		if item.Category == ItemCategoryTasks && item.Project != nil && item.Project.ID == archived.ID {
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
		return item.Item.Category == ItemCategoryProjects && item.Item.Project != nil && item.Item.Project.ID == archived.ID
	}) {
		t.Fatal("command palette search should find archived project by name")
	}
}
