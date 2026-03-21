package cmd

import (
	"encoding/json"
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
	RunE: runContextDelete,
}

func init() {
	contextCmd.AddCommand(contextDeleteCmd)
	contextDeleteCmd.Flags().BoolVarP(&contextDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(contextDeleteCmd)
}

func runContextDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("context name")
	}

	name := args[0]

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if !ConfirmYN(fmt.Sprintf("Delete context '%s'? [y/N]: ", name), contextDeleteForce) {
		fmt.Println("Canceled")
		return nil
	}

	if !cfg.DeleteCustomContext(name) {
		return fmt.Errorf("custom context '%s' not found", name)
	}

	if err := saveConfig(mgr, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, err := json.MarshalIndent(struct {
			Context string `json:"context"`
			Deleted bool   `json:"deleted"`
		}{Context: name, Deleted: true}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Deleted custom context '%s'\n", name)
	return nil
}
