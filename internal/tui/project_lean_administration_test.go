package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestLeanProjectCreateAndRenameParticipateInWorkspaceUndo(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		fixture := newSession4LifecycleFixture(t, true)
		creator := NewProjectCreatorModel(fixture.app.runtime, fixture.app.configManager, fixture.app.appService)
		creator.nameInput.SetValue("Created Project")
		creator.contextInput.SetValue("work")

		raw := creator.createProject()()
		created, ok := raw.(ProjectCreatedMsg)
		if !ok || created.Project == nil {
			t.Fatalf("create result = %#v, want ProjectCreatedMsg", raw)
		}
		if created.Project.ColorSlot < 1 || created.Project.ColorSlot > config.ProjectColorSlotCount {
			t.Fatalf("created color slot = %d, want automatic durable slot", created.Project.ColorSlot)
		}
		tasksPath := storage.TasksFilePath(created.Project.Path)
		if _, err := os.Stat(tasksPath); err != nil {
			t.Fatalf("created tasks document: %v", err)
		}

		ledger := fixture.app.deps.historyLedger()
		entry, ok, err := ledger.SelectNewestUndo(storage.ProjectScope(created.Project.ID))
		if err != nil || !ok || entry.Action != "Create Project" {
			t.Fatalf("create undo entry = %#v/%v/%v", entry, ok, err)
		}
		if _, undoErr := ledger.Undo(storage.ProjectScope(created.Project.ID)); undoErr != nil {
			t.Fatalf("undo create: %v", undoErr)
		}
		reloaded, err := config.LoadRuntimeState(fixture.dataDir)
		if err != nil {
			t.Fatalf("reload after create undo: %v", err)
		}
		if reloaded.GetProject(created.Project.ID) != nil {
			t.Fatal("undo create left the project in the catalog")
		}
		if _, err := os.Stat(tasksPath); !os.IsNotExist(err) {
			t.Fatalf("undo create left tasks.md at %q: %v", tasksPath, err)
		}
	})

	t.Run("rename", func(t *testing.T) {
		project := newSession4Project(t, "Original")
		fixture := newSession4LifecycleFixture(t, true, project)
		live := fixture.app.projectByID(project.ID)
		oldPath := live.Path
		editor := NewProjectEditorModel(
			live,
			fixture.app.configManager,
			fixture.app.runtime,
			fixture.app.appService,
			false,
		)
		editor.nameInput.SetValue("Renamed Project")
		editor.descriptionInput.SetValue("lean metadata")

		raw := editor.saveProject()()
		updated, ok := raw.(ProjectUpdatedMsg)
		if !ok || updated.Project == nil {
			t.Fatalf("edit result = %#v, want ProjectUpdatedMsg", raw)
		}
		newPath := filepath.Join(fixture.dataDir, "projects", "renamed-project")
		if live.Path != newPath {
			t.Fatalf("published path = %q, want %q", live.Path, newPath)
		}

		ledger := fixture.app.deps.historyLedger()
		entry, ok, err := ledger.SelectNewestUndo(storage.ProjectScope(project.ID))
		if err != nil || !ok || entry.Action != "Edit Project" {
			t.Fatalf("edit undo entry = %#v/%v/%v", entry, ok, err)
		}
		if _, undoErr := ledger.Undo(storage.ProjectScope(project.ID)); undoErr != nil {
			t.Fatalf("undo edit: %v", undoErr)
		}
		reloaded, err := config.LoadRuntimeState(fixture.dataDir)
		if err != nil {
			t.Fatalf("reload after edit undo: %v", err)
		}
		restored := reloaded.GetProject(project.ID)
		if restored == nil || restored.Name != "Original" || restored.Path != oldPath {
			t.Fatalf("undo edit restored %#v, want Original at %q", restored, oldPath)
		}
		if _, err := os.Stat(storage.TasksFilePath(oldPath)); err != nil {
			t.Fatalf("undo edit did not restore old tasks path: %v", err)
		}
		if _, err := os.Stat(newPath); !os.IsNotExist(err) {
			t.Fatalf("undo edit left renamed directory: %v", err)
		}
	})
}

func TestLeanProjectPinArchiveAndRestoreUseNamedUndoBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		action      string
		run         func(*AppModel, *models.Project)
	}{
		{
			name:        "pin",
			projectName: "Pinned Work",
			action:      "Pin Project",
			run: func(shell *AppModel, project *models.Project) {
				shell.handleProjectMessages(ProjectPinRequestMsg{Project: project, Pinned: true})
			},
		},
		{
			name:        "archive",
			projectName: "Archived Work",
			action:      "Archive Project",
			run: func(shell *AppModel, project *models.Project) {
				shell.commitProjectArchive(project)
			},
		},
		{
			name:        "restore",
			projectName: "Restored Work",
			action:      "Restore Project",
			run: func(shell *AppModel, project *models.Project) {
				project.Archive()
				session4CommitCatalogProject(t, shell, project)
				shell.commitProjectUnarchive(project)
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			project := newSession4Project(t, testCase.projectName)
			fixture := newSession4LifecycleFixture(t, true, project)
			live := fixture.app.projectByID(project.ID)
			testCase.run(fixture.app, live)

			entry, ok, err := fixture.app.deps.historyLedger().SelectNewestUndo(storage.ProjectScope(project.ID))
			if err != nil || !ok || entry.Action != testCase.action {
				t.Fatalf("%s undo entry = %#v/%v/%v", testCase.name, entry, ok, err)
			}
		})
	}
}

func TestSystemInboxEditKeepsIdentityImmutable(t *testing.T) {
	inbox := newSession4Project(t, "Inbox")
	fixture := newSession4LifecycleFixture(t, true, inbox)
	catalog := cloneWorkspaceCatalog(fixture.app.runtime.Catalog())
	catalog.SystemInboxProjectID = inbox.ID
	if err := fixture.app.runtime.CommitCatalog(catalog); err != nil {
		t.Fatalf("protect Inbox: %v", err)
	}
	live := fixture.app.projectByID(inbox.ID)
	editor := NewProjectEditorModel(
		live,
		fixture.app.configManager,
		fixture.app.runtime,
		fixture.app.appService,
		true,
	)
	editor.descriptionInput.SetValue("Workspace capture")
	editor.contextInput.SetValue("work")
	raw := editor.saveProject()()
	if _, ok := raw.(ProjectUpdatedMsg); !ok {
		t.Fatalf("mutable Inbox metadata edit = %#v, want success", raw)
	}
	if live.Name != "Inbox" || live.Slug != "inbox" || live.Description != "Workspace capture" || live.Context != "work" {
		t.Fatalf("edited Inbox = %#v", live)
	}

	editor = NewProjectEditorModel(
		live,
		fixture.app.configManager,
		fixture.app.runtime,
		fixture.app.appService,
		true,
	)
	editor.nameInput.SetValue("Renamed Inbox")
	raw = editor.saveProject()()
	if _, ok := raw.(ErrorMsg); !ok {
		t.Fatalf("Inbox identity edit = %#v, want ErrorMsg", raw)
	}
}
