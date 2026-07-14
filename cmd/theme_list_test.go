package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupThemeListTest(t *testing.T) (*testDeps, *config.Manager) {
	t.Helper()
	resetThemeExportVars()
	deps := setupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() {
		viper.Set("config", "")
		resetThemeExportVars()
	})
	return deps, manager
}

func TestThemeList(t *testing.T) {
	t.Run("empty list returns no error", func(t *testing.T) {
		deps, _ := setupThemeListTest(t)
		defer deps.Cleanup()

		err := runThemeList(themeListCmd, nil)
		testutil.AssertNoError(t, err)
	})

	t.Run("lists exported themes", func(t *testing.T) {
		deps, manager := setupThemeListTest(t)
		defer deps.Cleanup()

		// Export two themes.
		deps.Config.Settings().ThemeDark = config.Theme{CursorColor: "#aaaaaa"}
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"alpha"}))
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"beta"}))
		themeExportSlot = ""

		themes, err := manager.ListThemes()
		testutil.AssertNoError(t, err)
		if len(themes) != 2 {
			t.Fatalf("expected 2 themes, got %d", len(themes))
		}

		names := map[string]bool{themes[0].Name: true, themes[1].Name: true}
		if !names["alpha"] || !names["beta"] {
			t.Errorf("expected alpha and beta in list, got %v", names)
		}
	})

	t.Run("list command succeeds after export", func(t *testing.T) {
		deps, manager := setupThemeListTest(t)
		defer deps.Cleanup()

		deps.Config.Settings().ThemeDark = config.Theme{CursorColor: "#bbbbbb"}
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"listme"}))
		themeExportSlot = ""

		err := runThemeList(themeListCmd, nil)
		testutil.AssertNoError(t, err)
	})
}
