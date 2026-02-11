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
  tt task attr 1 --epic.scope=defined --epic.milestone="Q1 2025"
  tt task attr 5 --clear`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskAttr,
}

var (
	// Attribute flags for task attr command
	attrBugReproducible string
	attrBugSeverity     string
	attrBugWorkaround   string
	attrIdeaCategory    string
	attrIdeaEffort      string
	attrIdeaValue       string
	attrIdeaValidated   bool
	attrTaskEffort      string
	attrTaskWorkType    string
	attrStoryPoints     int
	attrStoryPersona    string
	attrEpicScope       string
	attrEpicMilestone   string
	attrEpicMetrics     string
	attrClear           bool
)

func init() {
	taskCmd.AddCommand(taskAttrCmd)

	// Bug attributes
	taskAttrCmd.Flags().StringVar(&attrBugReproducible, "bug.reproducible", "", "bug reproducibility (yes/no/sometimes)")
	taskAttrCmd.Flags().StringVar(&attrBugSeverity, "bug.severity", "", "bug severity (critical/major/minor/cosmetic)")
	taskAttrCmd.Flags().StringVar(&attrBugWorkaround, "bug.workaround", "", "bug workaround text")

	// Idea attributes
	taskAttrCmd.Flags().StringVar(&attrIdeaCategory, "idea.category", "", "idea category (feature/improvement/refactor/research)")
	taskAttrCmd.Flags().StringVar(&attrIdeaEffort, "idea.effort", "", "idea effort estimate (small/medium/large/unknown)")
	taskAttrCmd.Flags().StringVar(&attrIdeaValue, "idea.value", "", "idea value (low/medium/high)")
	taskAttrCmd.Flags().BoolVar(&attrIdeaValidated, "idea.validated", false, "idea is validated")

	// Task attributes
	taskAttrCmd.Flags().StringVar(&attrTaskEffort, "task.effort", "", "task effort (small/medium/large)")
	taskAttrCmd.Flags().StringVar(&attrTaskWorkType, "task.worktype", "", "task work type (development/documentation/testing/devops/design)")

	// Story attributes
	taskAttrCmd.Flags().IntVar(&attrStoryPoints, "story.points", 0, "story points")
	taskAttrCmd.Flags().StringVar(&attrStoryPersona, "story.persona", "", "story persona")

	// Epic attributes
	taskAttrCmd.Flags().StringVar(&attrEpicScope, "epic.scope", "", "epic scope (defined/evolving/undefined)")
	taskAttrCmd.Flags().StringVar(&attrEpicMilestone, "epic.milestone", "", "epic milestone")
	taskAttrCmd.Flags().StringVar(&attrEpicMetrics, "epic.metrics", "", "epic success metrics")

	// Clear flag
	taskAttrCmd.Flags().BoolVar(&attrClear, "clear", false, "clear all type-specific attributes")
}

func runTaskAttr(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Check if any flags were provided
	anyFlagSet := attrClear ||
		attrBugReproducible != "" || attrBugSeverity != "" || attrBugWorkaround != "" ||
		attrIdeaCategory != "" || attrIdeaEffort != "" || attrIdeaValue != "" || cmd.Flags().Changed("idea.validated") ||
		attrTaskEffort != "" || attrTaskWorkType != "" ||
		cmd.Flags().Changed("story.points") || attrStoryPersona != "" ||
		attrEpicScope != "" || attrEpicMilestone != "" || attrEpicMetrics != ""

	if !anyFlagSet {
		return fmt.Errorf("no attribute flags provided; use --help to see available options")
	}

	updatedTask, err := WithTaskUpdate(taskID, func(task *models.Task) error {
		// Handle clear flag
		if attrClear {
			task.ClearTypeAttrs()
			return nil
		}

		return applyTypeAttrValues(task, TypeAttrValues{
			BugReproducible:  attrBugReproducible,
			BugSeverity:      attrBugSeverity,
			BugWorkaround:    attrBugWorkaround,
			IdeaCategory:     attrIdeaCategory,
			IdeaEffort:       attrIdeaEffort,
			IdeaValue:        attrIdeaValue,
			IdeaValidatedSet: cmd.Flags().Changed("idea.validated"),
			IdeaValidated:    attrIdeaValidated,
			TaskEffort:       attrTaskEffort,
			TaskWorkType:     attrTaskWorkType,
			StoryPointsSet:   cmd.Flags().Changed("story.points"),
			StoryPoints:      attrStoryPoints,
			StoryPersona:     attrStoryPersona,
			EpicScope:        attrEpicScope,
			EpicMilestone:    attrEpicMilestone,
			EpicMetrics:      attrEpicMetrics,
		})
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
