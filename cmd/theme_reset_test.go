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

func setupThemeResetTest(t *testing.T) (*testutil.TestDeps, *config.Manager) {
	t.Helper()
	resetThemeResetVars()
	deps := testutil.SetupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, manager.Save(deps.Config))
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
		deps.Config.ThemeDark = config.Theme{
			BackgroundColor: "#aaaaaa",
			CursorColor:     "#bbbbbb",
		}
		testutil.AssertNoError(t, manager.Save(deps.Config))

		themeResetSlot = "dark"
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.Load()
		testutil.AssertNoError(t, err)
		want := config.DefaultDarkTheme()
		testutil.AssertEqual(t, reloaded.ThemeDark.CursorColor, want.CursorColor)
		testutil.AssertEqual(t, reloaded.ThemeDark.CursorTextColor, want.CursorTextColor)
		testutil.AssertEqual(t, reloaded.ThemeDark.BackgroundColor, want.BackgroundColor)
	})

	t.Run("reset light slot writes DefaultLightTheme with --yes", func(t *testing.T) {
		deps, manager := setupThemeResetTest(t)
		defer deps.Cleanup()

		// Set a custom light theme.
		deps.Config.ThemeLight = config.Theme{
			BackgroundColor: "#111111",
			CursorColor:     "#222222",
		}
		testutil.AssertNoError(t, manager.Save(deps.Config))

		themeResetSlot = "light"
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.Load()
		testutil.AssertNoError(t, err)
		want := config.DefaultLightTheme()
		testutil.AssertEqual(t, reloaded.ThemeLight.BackgroundColor, want.BackgroundColor)
		testutil.AssertEqual(t, reloaded.ThemeLight.CursorColor, want.CursorColor)
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

		deps.Config.ActiveTheme = "dark"
		deps.Config.ThemeDark = config.Theme{
			BackgroundColor: "#ffffff",
			CursorColor:     "#eeeeee",
		}
		testutil.AssertNoError(t, manager.Save(deps.Config))

		// No --slot; should default to active theme ("dark").
		themeResetYes = true
		err := runThemeReset(themeResetCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, err := manager.Load()
		testutil.AssertNoError(t, err)
		want := config.DefaultDarkTheme()
		testutil.AssertEqual(t, reloaded.ThemeDark.CursorColor, want.CursorColor)
	})
}
