package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	blockerEditTitle       string
	blockerEditDescription string
	blockerUnresolve       bool
)

var blockerEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a blocker",
	Long: `Edit a blocker's title, description, or resolved status.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker edit abc12345 --title "New title"
  tt blocker edit abc1 -d "Updated description"
  tt blocker edit abc1 --unresolve`,
	RunE: runBlockerEdit,
}

func init() {
	blockerCmd.AddCommand(blockerEditCmd)

	blockerEditCmd.Flags().StringVarP(&blockerEditTitle, "title", "t", "", "new title for the blocker")
	blockerEditCmd.Flags().StringVarP(&blockerEditDescription, "description", "d", "", "new description for the blocker")
	blockerEditCmd.Flags().BoolVar(&blockerUnresolve, "unresolve", false, "mark blocker as unresolved")
	addOutputFlags(blockerEditCmd)
}

func runBlockerEdit(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("blocker ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	return WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		modified := false

		if blockerEditTitle != "" {
			blocker.Title = strings.TrimSpace(blockerEditTitle)
			modified = true
		}

		if blockerEditDescription != "" {
			blocker.Description = strings.TrimSpace(blockerEditDescription)
			modified = true
		}

		if blockerUnresolve {
			if blocker.IsResolved() {
				blocker.Unresolve()
				modified = true
			}
		}

		if !modified {
			return blockers, tterrors.NewNoChangesError("--title, --description, or --unresolve flags")
		}

		if out.Quiet {
			return blockers, nil
		}
		if out.JSON {
			data, err := json.MarshalIndent(blocker, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshaling blocker: %w", err)
			}
			fmt.Println(string(data))
			return blockers, nil
		}
		fmt.Printf("Updated blocker: %s\n", blocker.Title)
		return blockers, nil
	})
}
