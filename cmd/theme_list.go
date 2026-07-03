package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved themes",
	Long: `List all theme files saved in the themes/ folder of your tt data directory.

EXAMPLES

  tt theme list
  tt theme list --json`,
	RunE: runThemeList,
}

func init() {
	themeCmd.AddCommand(themeListCmd)
	addOutputFlags(themeListCmd)
}

func runThemeList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	_, configManager, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	themes, err := configManager.ListThemes()
	if err != nil {
		return fmt.Errorf("list themes: %w", err)
	}

	if out.JSON {
		if themes == nil {
			themes = []config.ThemeFile{}
		}
		return printJSON(themes)
	}

	if out.Quiet {
		return nil
	}

	if len(themes) == 0 {
		fmt.Println("No themes saved.")
		fmt.Println()
		fmt.Println("Use 'tt theme export <name>' to save the current theme.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tVARIANT\tCREATED")
	for _, tf := range themes {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", tf.Name, tf.Variant, tf.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	_ = w.Flush()
	return nil
}
