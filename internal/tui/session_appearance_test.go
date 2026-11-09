package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// A host following the system appearance names the session's theme at launch,
// so the first frame already paints it: the saved theme is not painted first,
// and nothing is written.
func TestSessionAppearanceReplacesTheSavedThemeAtStartWithoutSavingIt(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	t.Cleanup(func() { theme.ApplyNamedTheme("") })

	machine := driver.App().domain.runtime.Machine()
	machine.Theme = "dark"
	if err := driver.App().domain.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("persist theme: %v", err)
	}

	for _, want := range []string{"light", "dark"} {
		reloaded, err := config.LoadRuntimeState(workspace.DataDir)
		if err != nil {
			t.Fatalf("reload runtime: %v", err)
		}
		theme.ApplyNamedTheme("")
		app, err := newAppModelFromSnapshot(reloaded, config.NewManager(workspace.ConfigPath), WithSessionAppearance(want))
		if err != nil {
			t.Fatalf("NewAppModel: %v", err)
		}
		if theme.ActiveThemeName != want {
			t.Fatalf("appearance %s: started in %q", want, theme.ActiveThemeName)
		}
		if got := app.domain.runtime.Machine().Theme; got != "dark" {
			t.Fatalf("appearance %s: saved theme = %q, want dark left alone", want, got)
		}
	}
}

// Docket.app starts its terminal on the background Docket's built-in themes
// paint, before Docket has drawn anything, so the two must name one color.
func TestDocketAppStartsOnTheBuiltInThemeBackgrounds(t *testing.T) {
	t.Cleanup(func() { theme.ApplyNamedTheme("") })
	source, err := os.ReadFile(filepath.Join("..", "..", "macos", "DocketApp", "Shell", "AppPreferences.swift"))
	if err != nil {
		t.Fatalf("read Docket.app preferences: %v", err)
	}
	for _, name := range []string{"dark", "light"} {
		theme.ApplyNamedTheme(name)
		line := "case ." + name + `: return "` + theme.ThemeBgColor + `"`
		if !strings.Contains(string(source), line) {
			t.Errorf("AppPreferences.swift lacks %q: DocketAppearanceScheme.background must match the %s theme", line, name)
		}
	}
}
