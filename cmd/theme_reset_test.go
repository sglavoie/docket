package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetThemeResetVars() {
	themeResetSlot = ""
	themeResetYes = false
}

func setupThemeResetTest(t *testing.T) (*testDeps, *config.Manager) {
	t.Helper()
	resetThemeResetVars()
	deps := setupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() {
		viper.Set("config", "")
		resetThemeResetVars()
	})
	return deps, manager
}

func TestThemeReset(t *testing.T) {
	t.Run("reset dark slot writes DefaultDarkTheme with --yes", func(t *testing.T) {
		deps, manager := setupThemeResetTest(t)
		defer deps.Cleanup()

		// Set a custom dark theme.
		deps.Config.Settings().ThemeDark = config.Theme{
			BackgroundColor: "#aaaaaa",
			CursorColor:     "#bbbbbb",
		}
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeResetSlot = "dark"
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		want := config.DefaultDarkTheme()
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.CursorColor, want.CursorColor)
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.CursorTextColor, want.CursorTextColor)
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.BackgroundColor, want.BackgroundColor)
	})

	t.Run("reset light slot writes DefaultLightTheme with --yes", func(t *testing.T) {
		deps, manager := setupThemeResetTest(t)
		defer deps.Cleanup()

		// Set a custom light theme.
		deps.Config.Settings().ThemeLight = config.Theme{
			BackgroundColor: "#111111",
			CursorColor:     "#222222",
		}
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		themeResetSlot = "light"
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		want := config.DefaultLightTheme()
		testutil.AssertEqual(t, reloaded.Settings().ThemeLight.BackgroundColor, want.BackgroundColor)
		testutil.AssertEqual(t, reloaded.Settings().ThemeLight.CursorColor, want.CursorColor)
	})

	t.Run("rejects invalid slot", func(t *testing.T) {
		deps, _ := setupThemeResetTest(t)
		defer deps.Cleanup()

		themeResetSlot = "bogus"
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		if err == nil {
			t.Error("expected error for invalid slot")
		}
	})

	t.Run("defaults to active theme slot when --slot omitted", func(t *testing.T) {
		deps, manager := setupThemeResetTest(t)
		defer deps.Cleanup()

		deps.Config.Settings().ActiveTheme = "dark"
		deps.Config.Settings().ThemeDark = config.Theme{
			BackgroundColor: "#ffffff",
			CursorColor:     "#eeeeee",
		}
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		// No --slot; should default to active theme ("dark").
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)
		want := config.DefaultDarkTheme()
		testutil.AssertEqual(t, reloaded.Settings().ThemeDark.CursorColor, want.CursorColor)
	})
}
