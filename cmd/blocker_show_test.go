package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunBlockerShow(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		err := blockerShowCmd.Args(blockerShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing blocker ID")
		}
	})
}

// TestRunBlockerShowRejectsEmptyPrefix ensures that calling `tt blocker show ""`
// returns a "not found" error rather than silently matching the first blocker.
// storage.FindBlockerByPrefix uses HasPrefix(id, "") which is always true, so
// without an explicit guard an empty arg would return the first blocker.
func TestRunBlockerShowRejectsEmptyPrefix(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Show Test")
	defer deps.Cleanup()

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	if err := manager.Save(deps.Config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	// Seed a blocker so the project has something to potentially match.
	b := models.NewBlocker("First blocker")
	if err := storage.SaveBlockers(project, []*models.Blocker{b}); err != nil {
		t.Fatalf("save blockers: %v", err)
	}

	err := runBlockerShow(blockerShowCmd, []string{""})
	if err == nil {
		t.Fatal("expected error for empty blocker prefix")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}
