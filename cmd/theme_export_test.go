package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

	// TestThemeExport_AtomicReplacement verifies that exporting over an
	// existing theme file in the managed themes directory is atomic: a
	// concurrent reader must never see an empty or invalid JSON file during
	// the write. A non-atomic os.WriteFile truncates before writing, opening
	// a race window that this test reliably hits.
	t.Run("atomic replacement of existing theme file", func(t *testing.T) {
		deps, manager := setupThemeExportTest(t)
		defer deps.Cleanup()

		// Seed the theme file so subsequent exports overwrite an existing file.
		if err := runThemeExport(themeExportCmd, []string{"concurrent"}); err != nil {
			t.Fatalf("seed export: %v", err)
		}
		path, err := manager.ThemePath("concurrent")
		if err != nil {
			t.Fatalf("ThemePath: %v", err)
		}

		var stop atomic.Bool
		var wg sync.WaitGroup
		var observedEmpty atomic.Bool

		wg.Go(func() {
			for !stop.Load() {
				info, statErr := os.Stat(path)
				if statErr != nil {
					continue
				}
				if info.Size() == 0 {
					observedEmpty.Store(true)
					return
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					continue
				}
				if len(data) == 0 {
					observedEmpty.Store(true)
					return
				}
				var v map[string]any
				if json.Unmarshal(data, &v) != nil {
					observedEmpty.Store(true)
					return
				}
			}
		})

		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			if err := runThemeExport(themeExportCmd, []string{"concurrent"}); err != nil {
				stop.Store(true)
				wg.Wait()
				t.Fatalf("runThemeExport: %v", err)
			}
		}
		stop.Store(true)
		wg.Wait()

		if observedEmpty.Load() {
			t.Fatal("reader observed empty/partial file during concurrent theme export — write is not atomic")
		}
	})
}
