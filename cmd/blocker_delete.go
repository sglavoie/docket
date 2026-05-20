package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerDeleteForce bool

var blockerDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a blocker",
	Long: `Delete a blocker permanently.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker delete abc12345
  tt blocker delete abc1 --force`,
	RunE: runBlockerDelete,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		blockers, err := storage.LoadBlockers(project)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var completions []string
		for _, b := range blockers {
			completions = append(completions, b.ID[:8]+"\t"+b.Title)
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	blockerCmd.AddCommand(blockerDeleteCmd)
	blockerDeleteCmd.Flags().BoolVarP(&blockerDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(blockerDeleteCmd)
}

func runBlockerDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("blocker ID (use 'tt blocker list' to see blockers)")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var deleted *models.Blocker
	canceled := false
	if err := WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		if !ConfirmYN(fmt.Sprintf("Delete blocker '%s'? [y/N]: ", blocker.Title), blockerDeleteForce) {
			canceled = true
			return blockers, nil
		}

		var newBlockers []*models.Blocker
		for _, b := range blockers {
			if b.ID != blocker.ID {
				newBlockers = append(newBlockers, b)
			}
		}
		deleted = blocker
		return newBlockers, nil
	}); err != nil {
		return err
	}

	if canceled {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	if out.Quiet || deleted == nil {
		return nil
	}
	if out.JSON {
		return printJSON(deleted)
	}
	fmt.Printf("Deleted blocker: %s\n", deleted.Title)
	return nil
}
