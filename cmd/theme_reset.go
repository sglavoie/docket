package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var themeResetSlot string
var themeResetYes bool

var themeResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a theme slot to built-in defaults",
	Long: `Reset a theme slot to the built-in curated defaults.

Without --yes, you will be prompted to confirm. Use --slot to choose which
slot to reset (defaults to the active theme).

EXAMPLES

  tt theme reset
  tt theme reset --slot light
  tt theme reset --slot dark --yes`,
	RunE: runThemeReset,
}

func init() {
	themeCmd.AddCommand(themeResetCmd)
	themeResetCmd.Flags().StringVar(&themeResetSlot, "slot", "", `theme slot to reset: "dark" or "light" (default: active theme)`)
	themeResetCmd.Flags().BoolVar(&themeResetYes, "yes", false, "skip confirmation prompt")
}

func runThemeReset(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	settings := domains.Settings

	slot := themeResetSlot
	if slot == "" {
		slot = settings.ActiveTheme
		if slot == "" {
			slot = "dark"
		}
	}
	if slot != "dark" && slot != "light" {
		return fmt.Errorf(`--slot must be "dark" or "light"`)
	}

	if !themeResetYes {
		fmt.Printf("Reset %s theme to defaults? [y/N] ", slot)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	settings.SetActiveTheme(slot, config.DefaultTheme(slot))
	if err := commitCLISettings(workspace, settings); err != nil {
		return err
	}

	fmt.Printf("Reset %s theme to defaults\n", slot)
	return nil
}
