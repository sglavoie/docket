package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestFreshTemporaryWorkspacesRenderInBothThemes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })

	for _, themeName := range []string{"dark", "light"} {
		t.Run("fresh-"+themeName, func(t *testing.T) {
			driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 35)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = workspace.Cleanup() })

			machine := driver.App().domain.runtime.Machine()
			machine.Theme = themeName
			if err := driver.App().domain.runtime.CommitMachine(machine); err != nil {
				t.Fatalf("commit fresh %s theme: %v", themeName, err)
			}
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
			assertIntegratedSmokeObservation(t, driver, "No tasks yet")
		})
	}
}

func assertIntegratedSmokeObservation(t *testing.T, driver *UITestDriver, visibleText string) {
	t.Helper()
	observation := driver.Observe(ObserveOptions{})
	if driver.App().currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("smoke view = %s, want Outliner", driver.App().currentView())
	}
	if !strings.Contains(observation.PlainScreen, visibleText) {
		t.Fatalf("smoke screen does not contain %q:\n%s", visibleText, observation.PlainScreen)
	}
	if len(observation.LayoutIssues) != 0 {
		t.Fatalf("smoke layout issues = %v", observation.LayoutIssues)
	}
}
