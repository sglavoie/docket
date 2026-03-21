package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerDescription string

var blockerAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new blocker",
	Long: `Add a new blocker to the current project.

A blocker represents an impediment that blocks progress on the project.
Blockers appear in 'tt blocker list' and can be resolved with 'tt blocker resolve'.

Examples:
  tt blocker add "Waiting for API access"
  tt blocker add "Manager absent, cannot get review" -d "Need approval by next week"
  tt b a "Build server down"`,
	RunE: runBlockerAdd,
}

func init() {
	blockerCmd.AddCommand(blockerAddCmd)
	blockerAddCmd.Flags().StringVarP(&blockerDescription, "description", "d", "", "blocker description")
	addOutputFlags(blockerAddCmd)
}

func runBlockerAdd(cmd *cobra.Command, args []string) error {
	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return tterrors.NewRequiredFieldError("title")
	}
	if len(title) > 500 {
		return tterrors.NewInvalidValueError("title", len(title), "max 500 characters")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	blocker, err := WithBlockerCreate(func() (*models.Blocker, error) {
		b := models.NewBlocker(title)
		b.Description = blockerDescription
		return b, nil
	})
	if err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputBlockerJSON(blocker)
	}

	fmt.Printf("Created blocker '%s' [%s]\n", title, blocker.ID[:8])
	return nil
}
