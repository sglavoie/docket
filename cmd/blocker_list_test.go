package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetBlockerListVars() {
	showAllBlockers = false
	showResolvedBlockers = false
	showActiveBlockers = false
}

func setupBlockerListTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	resetBlockerListVars()

	deps, project := setupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

func TestRunBlockerList(t *testing.T) {
	t.Run("runs without error when project has no blockers file", func(t *testing.T) {
		deps, _ := setupBlockerListTest(t)
		defer deps.Cleanup()

		err := runBlockerList(blockerListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

// TestBlockerListActiveFlagFiltersResolved verifies that the documented
// `--active` flag is actually honored. Previously the flag variable was
// defined and bound to a Cobra flag but never read, so passing `--active`
// alongside `--all` (or `--resolved`) silently ignored the user's filter.
func TestBlockerListActiveFlagFiltersResolved(t *testing.T) {
	deps, project := setupBlockerListTest(t)
	defer deps.Cleanup()

	active := models.NewBlocker("Still blocked")
	resolved := models.NewBlocker("Already done")
	resolved.Resolve()

	if err := storage.SaveBlockers(project, []*models.Blocker{active, resolved}); err != nil {
		t.Fatalf("save blockers: %v", err)
	}

	loaded, err := storage.LoadBlockers(project)
	testutil.AssertNoError(t, err)
	if len(loaded) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(loaded))
	}

	// With --all and --active both set, --active should win and filter out resolved.
	resetBlockerListVars()
	showAllBlockers = true
	showActiveBlockers = true

	visible := filterBlockersForList(loaded, showAllBlockers, showResolvedBlockers, showActiveBlockers)
	if len(visible) != 1 {
		t.Fatalf("expected 1 active blocker when --active is set, got %d", len(visible))
	}
	if visible[0].IsResolved() {
		t.Errorf("expected only active blocker, got resolved one: %q", visible[0].Title)
	}
}

// TestBlockerListShortIDDoesNotPanic ensures that a blocker whose stored ID
// is shorter than 8 characters (possible after manual edits or corruption)
// does not crash the table renderer when slicing the ID for display.
func TestBlockerListShortIDDoesNotPanic(t *testing.T) {
	deps, project := setupBlockerListTest(t)
	defer deps.Cleanup()

	short := models.NewBlocker("Short ID blocker")
	short.ID = "abc" // simulate manually-edited blockers.md with a non-UUID id
	if err := storage.SaveBlockers(project, []*models.Blocker{short}); err != nil {
		t.Fatalf("save blockers: %v", err)
	}

	resetBlockerListVars()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runBlockerList panicked on short ID: %v", r)
		}
	}()

	if err := runBlockerList(blockerListCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBlockerListActiveAndResolvedConflict ensures the two mutually
// exclusive filters return an error rather than silently picking one.
func TestBlockerListActiveAndResolvedConflict(t *testing.T) {
	deps, _ := setupBlockerListTest(t)
	defer deps.Cleanup()

	resetBlockerListVars()
	showActiveBlockers = true
	showResolvedBlockers = true

	err := runBlockerList(blockerListCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --active and --resolved are both set")
	}
}
