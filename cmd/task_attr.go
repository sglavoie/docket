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

	var changes []string
	updatedTask, err := WithTaskUpdate(taskID, func(task *models.Task) error {
		// Handle clear flag
		if attrClear {
			task.ClearTypeAttrs()
			return nil
		}

		// Capture old values for delta output
		changes = collectAttrDeltas(task, attrTypeAttrs, cmd)

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
		for _, c := range changes {
			fmt.Printf("  %s\n", c)
		}
	}

	return nil
}

// collectAttrDeltas captures the old values for attributes that are about to change
// and returns formatted "field: old -> new" strings.
func collectAttrDeltas(task *models.Task, vals TypeAttrValues, cmd *cobra.Command) []string {
	var deltas []string
	delta := func(name, oldVal, newVal string) {
		if oldVal == "" {
			oldVal = "(none)"
		}
		deltas = append(deltas, fmt.Sprintf("%s: %s -> %s", name, oldVal, newVal))
	}

	// Bug
	if vals.BugReproducible != "" {
		old := ""
		if task.BugAttrs != nil {
			old = string(task.BugAttrs.Reproducible)
		}
		delta("reproducible", old, vals.BugReproducible)
	}
	if vals.BugSeverity != "" {
		old := ""
		if task.BugAttrs != nil {
			old = string(task.BugAttrs.Severity)
		}
		delta("severity", old, vals.BugSeverity)
	}
	if vals.BugWorkaround != "" {
		old := ""
		if task.BugAttrs != nil {
			old = task.BugAttrs.Workaround
		}
		delta("workaround", old, vals.BugWorkaround)
	}

	// Idea
	if vals.IdeaCategory != "" {
		old := ""
		if task.IdeaAttrs != nil {
			old = string(task.IdeaAttrs.Category)
		}
		delta("category", old, vals.IdeaCategory)
	}
	if vals.IdeaEffort != "" {
		old := ""
		if task.IdeaAttrs != nil {
			old = string(task.IdeaAttrs.Effort)
		}
		delta("effort", old, vals.IdeaEffort)
	}
	if vals.IdeaValue != "" {
		old := ""
		if task.IdeaAttrs != nil {
			old = string(task.IdeaAttrs.Value)
		}
		delta("value", old, vals.IdeaValue)
	}
	if cmd.Flags().Changed("idea.validated") {
		old := "false"
		if task.IdeaAttrs != nil && task.IdeaAttrs.Validated {
			old = "true"
		}
		delta("validated", old, fmt.Sprintf("%t", vals.IdeaValidated))
	}

	// Task
	if vals.TaskEffort != "" {
		old := ""
		if task.TaskAttrs != nil {
			old = string(task.TaskAttrs.Effort)
		}
		delta("effort", old, vals.TaskEffort)
	}
	if vals.TaskWorkType != "" {
		old := ""
		if task.TaskAttrs != nil {
			old = string(task.TaskAttrs.WorkType)
		}
		delta("worktype", old, vals.TaskWorkType)
	}

	// Story
	if cmd.Flags().Changed("story.points") {
		old := "0"
		if task.StoryAttrs != nil {
			old = fmt.Sprintf("%d", task.StoryAttrs.StoryPoints)
		}
		delta("points", old, fmt.Sprintf("%d", vals.StoryPoints))
	}
	if vals.StoryPersona != "" {
		old := ""
		if task.StoryAttrs != nil {
			old = task.StoryAttrs.Persona
		}
		delta("persona", old, vals.StoryPersona)
	}

	// Epic
	if vals.EpicScope != "" {
		old := ""
		if task.EpicAttrs != nil {
			old = string(task.EpicAttrs.Scope)
		}
		delta("scope", old, vals.EpicScope)
	}
	if vals.EpicMilestone != "" {
		old := ""
		if task.EpicAttrs != nil {
			old = task.EpicAttrs.Milestone
		}
		delta("milestone", old, vals.EpicMilestone)
	}
	if vals.EpicMetrics != "" {
		old := ""
		if task.EpicAttrs != nil {
			old = task.EpicAttrs.SuccessMetrics
		}
		delta("metrics", old, vals.EpicMetrics)
	}

	return deltas
}
