package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSystemInboxBootstrapMovesLegacyActionInboxAtomically(t *testing.T) {
	workspace, workspaceErr := newUITestWorkspace()
	if workspaceErr != nil {
		t.Fatal(workspaceErr)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	legacy := testutil.NewTestProject("Action Inbox")
	legacy.Slug = config.LegacyActionInboxSlug
	legacy.Path = filepath.Join(config.InternalDir(workspace.DataDir), config.LegacyActionInboxSlug)
	legacy.Internal = true
	legacy.Tasks = []*models.Task{testutil.NewTestTask("Keep this capture", models.TaskTypeTask)}
	if err := storage.CreateProjectDir(legacy); err != nil {
		t.Fatalf("seed legacy Action Inbox: %v", err)
	}
	catalog := cloneWorkspaceCatalog(workspace.Runtime.Catalog())
	catalog.UpsertProject(legacy)
	catalog.SystemInboxProjectID = legacy.ID
	workspace.Runtime.SetCatalogInMemory(catalog)

	driver, launchErr := workspace.launch(100, 35)
	if launchErr != nil {
		t.Fatal(launchErr)
	}
	inbox := driver.App().runtime.SystemInboxProject()
	if inbox == nil || inbox.ID != legacy.ID || inbox.Name != config.SystemInboxName || inbox.Internal {
		t.Fatalf("upgraded Inbox = %+v", inbox)
	}
	if err := storage.LoadProject(inbox); err != nil {
		t.Fatalf("load upgraded Inbox: %v", err)
	}
	if len(inbox.Tasks) != 1 || inbox.Tasks[0].Title != "Keep this capture" {
		t.Fatalf("upgraded Inbox tasks = %+v", inbox.Tasks)
	}
	if _, err := os.Stat(legacy.Path); !os.IsNotExist(err) {
		t.Fatalf("legacy Action Inbox path still exists: %v", err)
	}
}
