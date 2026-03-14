package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskAttrCmd = &cobra.Command{
	Use:   "attr <id>",
	Short: "Set type-specific attributes on a task",
	Long: `Set type-specific attributes on a task.

Each task type has its own attributes:

Bug:
  --bug.reproducible    yes/no/sometimes
  --bug.severity        critical/major/minor/cosmetic
  --bug.workaround      text describing workaround

Idea:
  --idea.category       feature/improvement/refactor/research
  --idea.effort         small/medium/large/unknown
  --idea.value          low/medium/high
  --idea.validated      mark idea as validated

Task:
  --task.effort         small/medium/large
  --task.worktype       development/documentation/testing/devops/design

Story:
  --story.points        story points (integer)
  --story.persona       user persona text

Epic:
  --epic.scope          defined/evolving/undefined
  --epic.milestone      milestone text
  --epic.metrics        success metrics text

Use --clear to remove all type-specific attributes.

Examples:
  tt task attr 5 --bug.severity=major --bug.reproducible=yes
  tt task attr 3 --idea.category=feature --idea.value=high
  tt task attr 7 --task.effort=small --task.worktype=development
  tt task attr 2 --story.points=5 --story.persona="Admin user"
  tt task attr 1 --epic.scope=defined --epic.milestone="Q1 2026"
  tt task attr 5 --clear`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskAttr,
}

var (
	attrTypeAttrs TypeAttrValues
	attrClear     bool
)

func init() {
	taskCmd.AddCommand(taskAttrCmd)

	registerTypeAttrFlags(taskAttrCmd, &attrTypeAttrs)

	// Clear flag
	taskAttrCmd.Flags().BoolVar(&attrClear, "clear", false, "clear all type-specific attributes")
}

func runTaskAttr(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Check if any flags were provided
	v := attrTypeAttrs
	anyFlagSet := attrClear ||
		v.BugReproducible != "" || v.BugSeverity != "" || v.BugWorkaround != "" ||
		v.IdeaCategory != "" || v.IdeaEffort != "" || v.IdeaValue != "" || cmd.Flags().Changed("idea.validated") ||
		v.TaskEffort != "" || v.TaskWorkType != "" ||
		cmd.Flags().Changed("story.points") || v.StoryPersona != "" ||
		v.EpicScope != "" || v.EpicMilestone != "" || v.EpicMetrics != ""

	if !anyFlagSet {
		return fmt.Errorf("no attribute flags provided; use --help to see available options")
	}

	updatedTask, err := WithTaskUpdate(taskID, func(task *models.Task) error {
		// Handle clear flag
		if attrClear {
			task.ClearTypeAttrs()
			return nil
		}

		attrTypeAttrs.IdeaValidatedSet = cmd.Flags().Changed("idea.validated")
		attrTypeAttrs.StoryPointsSet = cmd.Flags().Changed("story.points")
		return applyTypeAttrValues(task, attrTypeAttrs)
	})

	if err != nil {
		if errors.Is(err, tterrors.ErrTaskNotFound) {
			return tterrors.NewTaskNotFoundError(taskID)
		}
		return err
	}

	if attrClear {
		fmt.Printf("Cleared attributes for %s #%d: %s\n",
			updatedTask.Type.HeadingPrefix(),
			updatedTask.ShortID,
			updatedTask.Title)
	} else {
		fmt.Printf("Updated attributes for %s #%d: %s\n",
			updatedTask.Type.HeadingPrefix(),
			updatedTask.ShortID,
			updatedTask.Title)
	}

	return nil
}
