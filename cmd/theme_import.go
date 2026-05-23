package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var themeImportInto string

var themeImportCmd = &cobra.Command{
	Use:   "import <name-or-path>",
	Short: "Import a theme into config",
	Long: `Import a named theme file and apply it to a theme slot.

Bare names are resolved inside ~/.tt/themes/. Arguments containing a path
separator or ending in .json are treated as file paths.

Use --into to override which slot the theme is applied to (default: the
variant stored in the file).

EXAMPLES

  tt theme import mine
  tt theme import mine --into dark
  tt theme import /tmp/shared-theme.json --into light`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeImport,
}

func init() {
	themeCmd.AddCommand(themeImportCmd)
	themeImportCmd.Flags().StringVar(&themeImportInto, "into", "", `apply theme into this slot: "dark" or "light" (default: file's variant)`)
}

func runThemeImport(cmd *cobra.Command, args []string) error {
	arg := args[0]

	cfg, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Resolve path: bare name → themes dir; path separators or .json → file path.
	var filePath string
	if filepath.IsAbs(arg) || strings.ContainsAny(arg, `/\`) || strings.HasSuffix(arg, ".json") {
		filePath = arg
	} else {
		p, themeErr := configManager.ThemePath(arg)
		if themeErr != nil {
			return themeErr
		}
		filePath = p
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read theme file: %w", err)
	}

	tf, err := config.UnmarshalThemeFile(data)
	if err != nil {
		return err
	}

	slot := tf.Variant
	if themeImportInto != "" {
		if themeImportInto != "dark" && themeImportInto != "light" {
			return fmt.Errorf(`--into must be "dark" or "light"`)
		}
		slot = themeImportInto
	}

	cfg.SetActiveTheme(slot, tf.Theme)
	if slot == "dark" {
		cfg.ThemeDarkRev = config.DarkThemeDefaultsRev
		cfg.ThemeDarkName = tf.Name
	} else {
		cfg.ThemeLightRev = config.LightThemeDefaultsRev
		cfg.ThemeLightName = tf.Name
	}
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	fmt.Printf("Imported %q into %s slot\n", tf.Name, slot)
	return nil
}
