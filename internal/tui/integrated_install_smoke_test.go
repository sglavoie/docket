package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

func TestFreshTemporaryWorkspacesRenderInBothThemes(t *testing.T) {
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
