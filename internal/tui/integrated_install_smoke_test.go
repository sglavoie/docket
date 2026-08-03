package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

const v5IntegratedSmokeWorkspaceID = "11111111-1111-4111-8111-111111111115"

//nolint:govet // Theme subtests keep runtime setup and commit errors local.
func TestFreshAndMigratedTemporaryWorkspacesRenderInBothThemes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

	for _, theme := range []string{"dark", "light"} {
		t.Run("fresh-"+theme, func(t *testing.T) {
			driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 35)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = workspace.Cleanup() })

			machine := driver.App().runtime.Machine()
			machine.Theme = theme
			if err := driver.App().runtime.CommitMachine(machine); err != nil {
				t.Fatalf("commit fresh %s theme: %v", theme, err)
			}
			ApplyTheme(ThemeConfig{Name: theme})
			assertIntegratedSmokeObservation(t, driver, "No tasks yet")
		})
	}

	migratedRoot := filepath.Join(t.TempDir(), "migrated-workspace")
	fixtureRoot := filepath.Join("..", "storage", "testdata", "v5_consolidation")
	if err := os.CopyFS(migratedRoot, os.DirFS(filepath.Join(fixtureRoot, "workspace"))); err != nil {
		t.Fatalf("copy schema-5 workspace: %v", err)
	}
	machineSource := filepath.Join(
		fixtureRoot, "machine", "runtime", v5IntegratedSmokeWorkspaceID, "machine.json",
	)
	machineData, err := os.ReadFile(machineSource)
	if err != nil {
		t.Fatalf("read schema-5 machine fixture: %v", err)
	}
	machinePath, err := config.MachineStatePath(v5IntegratedSmokeWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(machinePath), config.DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(machinePath, machineData, config.FilePermissions); err != nil {
		t.Fatal(err)
	}

	manager := config.NewManager(config.ConfigPathForDataDir(migratedRoot))
	if err := manager.MigrateWorkspaceToV6(); err != nil {
		t.Fatalf("migrate schema-5 smoke workspace: %v", err)
	}
	for _, theme := range []string{"dark", "light"} {
		t.Run("migrated-"+theme, func(t *testing.T) {
			runtime, err := config.LoadRuntimeState(migratedRoot)
			if err != nil {
				t.Fatalf("load migrated runtime: %v", err)
			}
			machine := runtime.Machine()
			machine.Theme = theme
			if err := runtime.CommitMachine(machine); err != nil {
				t.Fatalf("commit migrated %s theme: %v", theme, err)
			}
			app, err := NewAppModel(runtime, manager)
			if err != nil {
				t.Fatalf("launch migrated workspace: %v", err)
			}
			driver := NewUITestDriver(app, 100, 35)
			if app.startupMessage != nil {
				driver.dispatch(app.startupMessage)
				driver.drain()
			}
			assertIntegratedSmokeObservation(t, driver, "Active Doing source")
		})
	}
}

func assertIntegratedSmokeObservation(t *testing.T, driver *UITestDriver, visibleText string) {
	t.Helper()
	observation := driver.Observe(ObserveOptions{})
	if driver.App().currentView() != ViewCrossProjectOutliner {
		t.Fatalf("smoke view = %s, want Outliner", driver.App().currentView())
	}
	if !strings.Contains(observation.PlainScreen, visibleText) {
		t.Fatalf("smoke screen does not contain %q:\n%s", visibleText, observation.PlainScreen)
	}
	if len(observation.LayoutIssues) != 0 {
		t.Fatalf("smoke layout issues = %v", observation.LayoutIssues)
	}
}
