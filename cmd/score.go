package cmd

import (
	"github.com/spf13/cobra"
)

var scoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Manage scoring profiles and task scores",
	Long: `Commands for managing scoring profiles and computing task scores.

Scoring profiles define how tasks are ranked using configurable formulas
like RICE, WSJF, ICE, or custom weighted sums. Each profile has dimensions
(e.g., Impact, Confidence, Effort) that can be set manually on tasks or
derived automatically from existing task attributes.

EXAMPLES

  Add a preset RICE scoring profile:
    tt score profile add --preset rice

  Assign a profile to the current project:
    tt score assign rice

  Set scoring dimensions on a task:
    tt score set 42 impact=4 confidence=3

  Show score breakdown for a task:
    tt score show 42

  Rank tasks by composite score:
    tt score rank

AVAILABLE PRESETS

  rice  - Reach * Impact * Confidence / Effort
  wsjf  - (Business Value + Time Criticality + Risk Reduction) / Job Size
  ice   - Impact * Confidence * Ease`,
}

var scoreProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage scoring profiles",
	Long:  "Commands for creating, listing, viewing, editing, and deleting scoring profiles.",
}

func init() {
	rootCmd.AddCommand(scoreCmd)
	scoreCmd.AddCommand(scoreProfileCmd)
}
