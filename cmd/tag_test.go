package cmd

import (
	"os"
	"slices"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetTagCommandVars(t *testing.T) {
	t.Helper()
	tagPinGlobal = false
	tagUnpinGlobal = false
	tagListPinned = false
	tagListGlobal = false
	_ = tagPinCmd.Flags().Set("global", "false")
	_ = tagPinCmd.Flags().Set("json", "false")
	_ = tagPinCmd.Flags().Set("quiet", "false")
	_ = tagUnpinCmd.Flags().Set("global", "false")
	_ = tagUnpinCmd.Flags().Set("json", "false")
	_ = tagUnpinCmd.Flags().Set("quiet", "false")
	_ = tagListCmd.Flags().Set("global", "false")
	_ = tagListCmd.Flags().Set("pinned", "false")
	_ = tagListCmd.Flags().Set("json", "false")
	_ = tagListCmd.Flags().Set("quiet", "false")
}

func setupTagCommandTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	resetTagCommandVars(t)
	return setupSectionCommandTest(t)
}

func TestTagPinAndUnpinProjectTag(t *testing.T) {
	deps, project := setupTagCommandTest(t)

	err := runTagPin(nil, []string{"#backend"})
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[project.ID]
	if got, want := saved.PinnedTagList(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("PinnedTagList after pin = %v, want %v", got, want)
	}

	err = runTagUnpin(nil, []string{"backend"})
	testutil.AssertNoError(t, err)

	saved = deps.Repository.Projects[project.ID]
	if got := saved.PinnedTagList(); len(got) != 0 {
		t.Fatalf("PinnedTagList after unpin = %v, want empty", got)
	}
}

func TestTagPinAndUnpinGlobalTag(t *testing.T) {
	deps, _ := setupTagCommandTest(t)
	manager := config.NewManager(deps.ConfigPath)

	tagPinGlobal = true
	err := runTagPin(nil, []string{"#urgent"})
	testutil.AssertNoError(t, err)

	loaded, err := manager.LoadRuntimeState()
	testutil.AssertNoError(t, err)
	if got, want := loaded.Settings().GlobalPinnedTagList(), []string{"urgent"}; !slices.Equal(got, want) {
		t.Fatalf("GlobalPinnedTagList after pin = %v, want %v", got, want)
	}

	tagPinGlobal = false
	tagUnpinGlobal = true
	err = runTagUnpin(nil, []string{"urgent"})
	testutil.AssertNoError(t, err)

	loaded, err = manager.LoadRuntimeState()
	testutil.AssertNoError(t, err)
	if got := loaded.Settings().GlobalPinnedTagList(); len(got) != 0 {
		t.Fatalf("GlobalPinnedTagList after unpin = %v, want empty", got)
	}
}

func TestBuildProjectTagListEntriesPinnedOnly(t *testing.T) {
	project := testutil.NewTestProject("Tags")
	task := testutil.NewTestTask("Backend work", models.TaskTypeTask)
	task.Tags = []string{"backend"}
	project.Tasks = []*models.Task{task}
	project.Tags = []string{"frontend"}
	project.PinnedTags = []string{"backend", "unused"}

	entries := buildProjectTagListEntries(project, true)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2: %#v", len(entries), entries)
	}
	if entries[0].Tag != "backend" || entries[0].Tasks != 1 || !entries[0].Pinned {
		t.Fatalf("entries[0] = %#v, want pinned backend with one task", entries[0])
	}
	if entries[1].Tag != "unused" || entries[1].Tasks != 0 || !entries[1].Pinned {
		t.Fatalf("entries[1] = %#v, want pinned unused with zero tasks", entries[1])
	}
}

func TestSandboxResetClearsTagsAndPinnedTags(t *testing.T) {
	resetTagCommandVars(t)
	deps := setupTestDeps(t)

	sandbox := testutil.NewTestProject("Sandbox")
	sandbox.ID = "sandbox"
	sandbox.Slug = "_sandbox"
	sandbox.Name = "Sandbox"
	sandbox.Path = deps.Config.ProjectPath(sandbox.Slug)
	sandbox.Sandbox = true
	sandbox.Tags = []string{"catalog"}
	sandbox.PinnedTags = []string{"catalog", "task-tag"}
	task := testutil.NewTestTask("Tagged", models.TaskTypeTask)
	task.Tags = []string{"task-tag"}
	sandbox.Tasks = []*models.Task{task}
	sandbox.NextShortID = 2
	note := models.NewNote("Reset with sandbox")
	note.Body = "the CLI must delete this through the application transaction"
	if err := storage.SaveNote(sandbox, note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	notePath, err := storage.NoteFilePath(sandbox, note.Filename)
	if err != nil {
		t.Fatalf("NoteFilePath: %v", err)
	}

	setTestProjects(deps.Config, []*models.Project{sandbox})
	deps.Repository.Projects[sandbox.ID] = sandbox

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err = saveTestRuntime(manager, deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	sandboxResetForce = true
	t.Cleanup(func() { sandboxResetForce = false })

	err = runSandboxReset(nil, nil)
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[sandbox.ID]
	if len(saved.Tasks) != 0 {
		t.Fatalf("saved.Tasks len = %d, want 0", len(saved.Tasks))
	}
	if len(saved.Tags) != 0 {
		t.Fatalf("saved.Tags = %v, want empty", saved.Tags)
	}
	if len(saved.PinnedTags) != 0 {
		t.Fatalf("saved.PinnedTags = %v, want empty", saved.PinnedTags)
	}
	if saved.NextShortID != 1 {
		t.Fatalf("saved short-id = %d, want reset", saved.NextShortID)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("sandbox note exists after CLI reset, stat error=%v", err)
	}
}
