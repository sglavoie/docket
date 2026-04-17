package cmd

import (
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage named themes",
	Long: `Commands for exporting, importing, listing, and resetting named themes.

Themes are stored as portable JSON files in ~/.tt/themes/ and can be shared.

EXAMPLES

  Export the active theme as "mine":
    tt theme export mine

  Export the light slot:
    tt theme export mine --slot light

  Import a theme:
    tt theme import mine

  Import and apply to a different slot:
    tt theme import mine --into dark

  List saved themes:
    tt theme list

  Reset the dark slot to defaults:
    tt theme reset --slot dark --yes`,
	RunE: runThemeList,
}

func init() {
	rootCmd.AddCommand(themeCmd)
}
