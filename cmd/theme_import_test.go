package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetThemeImportVars() {
	themeImportInto = ""
}

func setupThemeImportTest(t *testing.T) (*testDeps, *config.Manager) {
	t.Helper()
	resetThemeImportVars()
	resetThemeExportVars()
	deps := setupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() {
		viper.Set("config", "")
		resetThemeImportVars()
		resetThemeExportVars()
	})
	return deps, manager
}

func TestThemeImport(t *testing.T) {
	t.Run("round-trip dark export then import", func(t *testing.T) {
		deps, manager := setupThemeImportTest(t)
		defer deps.Cleanup()

		// Set a custom dark theme and save it.
		deps.Config.Settings().ThemeDark = config.Theme{
			BackgroundColor: "#aabbcc",
			CursorColor:     "#112233",
			CursorTextColor: "#ffffff",
		}
		deps.Config.Settings().ThemeDarkRev = config.DarkThemeDefaultsRev
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		// Export it.
		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"rt"}))
		themeExportSlot = ""

		// Wipe the dark theme and save.
		deps.Config.Settings().ThemeDark = config.Theme{CursorColor: "#000001"} // keep non-zero
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		// Import the exported theme back.
		testutil.AssertNoError(t, runThemeImport(themeImportCmd, []string{"rt"}))

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.BackgroundColor, "#aabbcc")
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.CursorColor, "#112233")
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.CursorTextColor, "#ffffff")
	})

	t.Run("--into overrides variant", func(t *testing.T) {
		deps, manager := setupThemeImportTest(t)
		defer deps.Cleanup()

		// Export a custom dark theme.
		deps.Config.Settings().ThemeDark = config.Theme{
			BackgroundColor: "#112233",
			CursorColor:     "#aabbcc",
			CursorTextColor: "#ddee00",
		}
		deps.Config.Settings().ThemeDarkRev = config.DarkThemeDefaultsRev
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"into-test"}))
		themeExportSlot = ""

		// Import into the light slot.
		themeImportInto = "light"
		testutil.AssertNoError(t, runThemeImport(themeImportCmd, []string{"into-test"}))

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, reloaded.Settings().ThemeLight.BackgroundColor, "#112233")
		testutil.AssertEqual(t, reloaded.Settings().ThemeLight.CursorColor, "#aabbcc")
	})

	t.Run("rejects schema mismatch", func(t *testing.T) {
		deps, _ := setupThemeImportTest(t)
		defer deps.Cleanup()

		bad := `{"schema":"tt.theme/v99","name":"bad","variant":"dark"}`
		p := filepath.Join(t.TempDir(), "bad.json")
		testutil.AssertNoError(t, os.WriteFile(p, []byte(bad), 0600))

		err := runThemeImport(themeImportCmd, []string{p})
		if err == nil {
			t.Error("expected error for schema mismatch")
		}
	})

	t.Run("rejects path traversal in bare name", func(t *testing.T) {
		deps, _ := setupThemeImportTest(t)
		defer deps.Cleanup()

		// ".." has no slash so is treated as bare name → validateThemeName rejects it.
		err := runThemeImport(themeImportCmd, []string{".."})
		if err == nil {
			t.Error("expected error for path traversal bare name")
		}
	})

	t.Run("records preset name on slot", func(t *testing.T) {
		deps, manager := setupThemeImportTest(t)
		defer deps.Cleanup()

		// Export a custom dark theme under a known name.
		deps.Config.Settings().ThemeDark = config.Theme{BackgroundColor: "#abcdef"}
		deps.Config.Settings().ThemeDarkRev = config.DarkThemeDefaultsRev
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"named-preset"}))
		themeExportSlot = ""

		// Import it back; expect ThemeDarkName to reflect the imported preset.
		testutil.AssertNoError(t, runThemeImport(themeImportCmd, []string{"named-preset"}))

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, reloaded.Settings().ThemeDarkName, "named-preset")

		// Cross-slot import (--into light) should populate ThemeLightName.
		themeImportInto = "light"
		testutil.AssertNoError(t, runThemeImport(themeImportCmd, []string{"named-preset"}))
		themeImportInto = ""

		reloaded, err = manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, reloaded.Settings().ThemeLightName, "named-preset")
	})

	t.Run("rejects invalid --into value", func(t *testing.T) {
		deps, _ := setupThemeImportTest(t)
		defer deps.Cleanup()

		// Export a valid file first.
		themeExportSlot = "dark"
		testutil.AssertNoError(t, runThemeExport(themeExportCmd, []string{"valid"}))
		themeExportSlot = ""

		themeImportInto = "bogus"
		err := runThemeImport(themeImportCmd, []string{"valid"})
		if err == nil {
			t.Error("expected error for invalid --into value")
		}
	})
}
