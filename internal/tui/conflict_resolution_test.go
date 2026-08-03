package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newConflictTUIEnv(t *testing.T) (*AppModel, *storage.ConflictStore, string) {
	t.Helper()
	cfg := newTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	cfg = testRuntimeAt(config.WorkspaceRootFromConfigPath(configPath), cfg)
	manager := config.NewManager(configPath)
	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	dataDir := config.WorkspaceRootFromConfigPath(configPath)
	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	return app, store, dataDir
}

func tuiTestDoc(t *testing.T, title string) *storage.ProjectDocument {
	t.Helper()
	task, err := models.NewGenericTask(uuid.NewString(), 1, title)
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	doc := &storage.ProjectDocument{
		ProjectName:   "Alpha",
		Tasks:         []*models.Task{task},
		SchemaVersion: storage.SchemaVersionV6,
	}
	doc.Revision = doc.ComputeRevision()
	return doc
}

func recordTUIConflict(t *testing.T, store *storage.ConflictStore, dataDir string) string {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")}
	base := tuiTestDoc(t, "base title")
	local := tuiTestDoc(t, "local title")
	remote := tuiTestDoc(t, "remote title")
	id, err := storage.RecordMergeConflict(store, project, base, local, remote,
		[]storage.MergeConflict{{Kind: storage.MergeConflictFieldEdit, Field: "title", Base: "base title", Local: "local title", Current: "remote title"}},
		"task edit", "")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return id
}

// TestOpenConflictsMsgOpensView verifies the message opens the resolution view
// when a conflict exists.
func TestOpenConflictsMsgOpensView(t *testing.T) {
	app, store, dataDir := newConflictTUIEnv(t)
	recordTUIConflict(t, store, dataDir)

	handled, _ := app.handleConflictMessages(OpenConflictsMsg{})
	if !handled {
		t.Fatal("OpenConflictsMsg should be handled")
	}
	if app.currentView() != ViewConflictResolution {
		t.Fatalf("current view = %v, want ConflictResolution", app.currentView())
	}
}

// TestOpenConflictsMsgNoneSetsStatus verifies the entry point reports a
// conflict-free workspace instead of opening an empty view.
func TestOpenConflictsMsgNoneSetsStatus(t *testing.T) {
	app, _, _ := newConflictTUIEnv(t)

	handled, _ := app.handleConflictMessages(OpenConflictsMsg{})
	if !handled {
		t.Fatal("OpenConflictsMsg should be handled")
	}
	if app.currentView() == ViewConflictResolution {
		t.Fatal("no conflicts should not open the resolution view")
	}
	if app.statusMessage == "" {
		t.Fatal("expected an informational status when there are no conflicts")
	}
}

// TestConflictResolutionAdoptRemote verifies the resolution view's adopt-remote
// action resolves the conflict and reports it.
func TestConflictResolutionAdoptRemote(t *testing.T) {
	app, store, dataDir := newConflictTUIEnv(t)
	id := recordTUIConflict(t, store, dataDir)

	bundle, err := store.Load(id)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	m := NewConflictResolutionModel(app.runtime, bundle)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	if cmd == nil {
		t.Fatal("expected a command from the adopt-remote action")
	}
	msg := cmd()
	resolved, ok := msg.(ConflictResolvedMsg)
	if !ok {
		t.Fatalf("expected ConflictResolvedMsg, got %T", msg)
	}
	if !resolved.Adopted {
		t.Fatalf("unexpected resolution message: %+v", resolved)
	}
	if records, _ := store.List(); len(records) != 0 {
		t.Fatalf("conflict should be cleared, %d remain", len(records))
	}
}

// TestConflictResolvedMsgPopsAndStatuses verifies the app pops the resolution
// view and surfaces a status when a resolution completes.
func TestConflictResolvedMsgPopsAndStatuses(t *testing.T) {
	app, store, dataDir := newConflictTUIEnv(t)
	recordTUIConflict(t, store, dataDir)

	// Open the view first so there is something to pop.
	if _, cmd := app.handleConflictMessages(OpenConflictsMsg{}); cmd != nil {
		_ = cmd()
	}
	if app.currentView() != ViewConflictResolution {
		t.Fatalf("setup: current view = %v, want ConflictResolution", app.currentView())
	}

	handled, _ := app.handleConflictMessages(ConflictResolvedMsg{})
	if !handled {
		t.Fatal("ConflictResolvedMsg should be handled")
	}
	if app.currentView() == ViewConflictResolution {
		t.Fatal("resolution view should have been popped")
	}
	if app.statusMessage == "" {
		t.Fatal("expected a success status after resolution")
	}
}
