package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetThemeExportVars() {
	themeExportSlot = ""
	themeExportFile = ""
}

func setupThemeExportTest(t *testing.T) (*testutil.TestDeps, *config.Manager) {
	t.Helper()
	resetThemeExportVars()
	deps := testutil.SetupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, manager.Save(deps.Config))
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() {
		viper.Set("config", "")
		resetThemeExportVars()
	})
	return deps, manager
}

func TestThemeExport(t *testing.T) {
	t.Run("exports dark theme to themes dir", func(t *testing.T) {
		deps, manager := setupThemeExportTest(t)
		defer deps.Cleanup()

		err := runThemeExport(themeExportCmd, []string{"mytest"})
		testutil.AssertNoError(t, err)

		tf, err := manager.ReadTheme("mytest")
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, tf.Name, "mytest")
		testutil.AssertEqual(t, tf.Variant, "dark")
		testutil.AssertEqual(t, tf.Schema, "tt.theme/v1")
	})

	t.Run("exports light theme with --slot light", func(t *testing.T) {
		deps, manager := setupThemeExportTest(t)
		defer deps.Cleanup()

		themeExportSlot = "light"
		err := runThemeExport(themeExportCmd, []string{"light-test"})
		testutil.AssertNoError(t, err)

		tf, err := manager.ReadTheme("light-test")
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, tf.Variant, "light")
	})

	t.Run("exports to custom --file path", func(t *testing.T) {
		deps, _ := setupThemeExportTest(t)
		defer deps.Cleanup()

		tmp := filepath.Join(t.TempDir(), "out.json")
		themeExportFile = tmp
		err := runThemeExport(themeExportCmd, []string{"custom"})
		testutil.AssertNoError(t, err)

		if _, err := os.Stat(tmp); err != nil {
			t.Fatalf("expected file at %s: %v", tmp, err)
		}
	})

	t.Run("rejects path traversal in name", func(t *testing.T) {
		deps, _ := setupThemeExportTest(t)
		defer deps.Cleanup()

		err := runThemeExport(themeExportCmd, []string{"../evil"})
		if err == nil {
			t.Error("expected error for path traversal name")
		}
	})

	t.Run("round-trip preserves theme fields", func(t *testing.T) {
		deps, manager := setupThemeExportTest(t)
		defer deps.Cleanup()

		deps.Config.ThemeDark = config.Theme{
			BackgroundColor: "#aabbcc",
			CursorColor:     "#112233",
			CursorTextColor: "#ffffff",
		}
		deps.Config.ThemeDarkRev = config.DarkThemeDefaultsRev
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runThemeExport(themeExportCmd, []string{"roundtrip"})
		testutil.AssertNoError(t, err)

		tf, err := manager.ReadTheme("roundtrip")
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, tf.Theme.BackgroundColor, "#aabbcc")
		testutil.AssertEqual(t, tf.Theme.CursorColor, "#112233")
		testutil.AssertEqual(t, tf.Theme.CursorTextColor, "#ffffff")
	})

	t.Run("rejects invalid slot", func(t *testing.T) {
		deps, _ := setupThemeExportTest(t)
		defer deps.Cleanup()

		themeExportSlot = "invalid"
		err := runThemeExport(themeExportCmd, []string{"mytest"})
		if err == nil {
			t.Error("expected error for invalid slot")
		}
	})
}
