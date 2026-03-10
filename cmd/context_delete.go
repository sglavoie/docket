package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var contextDeleteForce bool

var contextDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"del", "rm"},
	Short:   "Delete a custom context",
	Long: `Delete a custom context. This does not delete any projects.

Examples:
  tt context delete Work
  tt ctx del "Side Projects"`,
	Args: cobra.ExactArgs(1),
	RunE: runContextDelete,
}

func init() {
	contextCmd.AddCommand(contextDeleteCmd)
	contextDeleteCmd.Flags().BoolVarP(&contextDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runContextDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	if !ConfirmYN(fmt.Sprintf("Delete context '%s'? [y/N]: ", name), contextDeleteForce) {
		fmt.Println("Canceled.")
		return nil
	}

	if !cfg.DeleteCustomContext(name) {
		return fmt.Errorf("custom context '%s' not found", name)
	}

	if err := mgr.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Deleted custom context '%s'\n", name)
	return nil
}
