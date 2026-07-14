package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/util"
)

var themeExportSlot string
var themeExportFile string

var themeExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export the current theme to a file",
	Long: `Export a theme slot to a named theme file in the themes/ folder of your tt data directory.

Use --slot to choose which slot to export (defaults to the active theme).
Use --file to write to an arbitrary path instead of the default themes/<name>.json.

EXAMPLES

  tt theme export mine
  tt theme export mine --slot light
  tt theme export mine --file /tmp/my-theme.json`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeExport,
}

func init() {
	themeCmd.AddCommand(themeExportCmd)
	themeExportCmd.Flags().StringVar(&themeExportSlot, "slot", "", `theme slot to export: "dark" or "light" (default: active theme)`)
	themeExportCmd.Flags().StringVar(&themeExportFile, "file", "", "write to this path instead of the default themes/<name>.json")
}

func runThemeExport(cmd *cobra.Command, args []string) error {
	name := args[0]

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	settings := workspace.Runtime.Settings()

	slot := themeExportSlot
	if slot == "" {
		slot = settings.ActiveTheme
		if slot == "" {
			slot = "dark"
		}
	}
	if slot != "dark" && slot != "light" {
		return fmt.Errorf(`--slot must be "dark" or "light"`)
	}

	var theme config.Theme
	if slot == "light" {
		theme = settings.ThemeLight
	} else {
		theme = settings.ThemeDark
	}

	data, err := config.MarshalThemeFile(name, slot, Version, theme)
	if err != nil {
		return fmt.Errorf("marshal theme: %w", err)
	}
	data = append(data, '\n')

	var destPath string
	if themeExportFile != "" {
		destPath = themeExportFile
	} else {
		destPath, err = workspace.Manager.ThemePath(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(workspace.Manager.ThemesDir(), 0700); err != nil {
			return fmt.Errorf("create themes dir: %w", err)
		}
	}

	if err := util.AtomicWriteFile(destPath, data, 0600); err != nil {
		return fmt.Errorf("write theme file: %w", err)
	}

	fmt.Printf("Exported %q (%s) to %s\n", name, slot, destPath)
	return nil
}
