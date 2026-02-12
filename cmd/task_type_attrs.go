package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

// TypeAttrValues holds type-specific attribute values extracted from CLI flags.
// Boolean/int flags that have zero-value defaults use a *Set field to indicate
// whether the flag was explicitly provided.
type TypeAttrValues struct {
	BugReproducible string
	BugSeverity     string
	BugWorkaround   string

	IdeaCategory     string
	IdeaEffort       string
	IdeaValue        string
	IdeaValidatedSet bool
	IdeaValidated    bool

	TaskEffort   string
	TaskWorkType string

	StoryPointsSet bool
	StoryPoints    int
	StoryPersona   string

	EpicScope     string
	EpicMilestone string
	EpicMetrics   string
}

// registerTypeAttrFlags registers the type-specific attribute flags on a command,
// binding them to the fields of the given TypeAttrValues struct.
func registerTypeAttrFlags(cmd *cobra.Command, v *TypeAttrValues) {
	// Bug attributes
	cmd.Flags().StringVar(&v.BugReproducible, "bug.reproducible", "", "bug reproducibility (yes/no/sometimes)")
	cmd.Flags().StringVar(&v.BugSeverity, "bug.severity", "", "bug severity (critical/major/minor/cosmetic)")
	cmd.Flags().StringVar(&v.BugWorkaround, "bug.workaround", "", "bug workaround text")

	// Idea attributes
	cmd.Flags().StringVar(&v.IdeaCategory, "idea.category", "", "idea category (feature/improvement/refactor/research)")
	cmd.Flags().StringVar(&v.IdeaEffort, "idea.effort", "", "idea effort estimate (small/medium/large/unknown)")
	cmd.Flags().StringVar(&v.IdeaValue, "idea.value", "", "idea value (low/medium/high)")
	cmd.Flags().BoolVar(&v.IdeaValidated, "idea.validated", false, "idea is validated")

	// Task attributes
	cmd.Flags().StringVar(&v.TaskEffort, "task.effort", "", "task effort (small/medium/large)")
	cmd.Flags().StringVar(&v.TaskWorkType, "task.worktype", "", "task work type (development/documentation/testing/devops/design)")

	// Story attributes
	cmd.Flags().IntVar(&v.StoryPoints, "story.points", 0, "story points")
	cmd.Flags().StringVar(&v.StoryPersona, "story.persona", "", "story persona")

	// Epic attributes
	cmd.Flags().StringVar(&v.EpicScope, "epic.scope", "", "epic scope (defined/evolving/undefined)")
	cmd.Flags().StringVar(&v.EpicMilestone, "epic.milestone", "", "epic milestone")
	cmd.Flags().StringVar(&v.EpicMetrics, "epic.metrics", "", "epic success metrics")
}

// applyTypeAttrValues applies type-specific attributes to a task.
// It ensures the appropriate attrs struct exists (creating it if nil),
// assigns the provided values, and validates the result.
func applyTypeAttrValues(task *models.Task, vals TypeAttrValues) error {
	// Bug attributes
	hasBugAttrs := vals.BugReproducible != "" || vals.BugSeverity != "" || vals.BugWorkaround != ""
	if hasBugAttrs {
		if task.Type != models.TaskTypeBug {
			return fmt.Errorf("bug.* flags can only be used with bug type tasks")
		}
		if task.BugAttrs == nil {
			task.BugAttrs = &models.BugAttrs{}
		}
		if vals.BugReproducible != "" {
			task.BugAttrs.Reproducible = models.BugReproducible(vals.BugReproducible)
		}
		if vals.BugSeverity != "" {
			task.BugAttrs.Severity = models.BugSeverity(vals.BugSeverity)
		}
		if vals.BugWorkaround != "" {
			task.BugAttrs.Workaround = vals.BugWorkaround
		}
		if err := task.BugAttrs.Validate(); err != nil {
			return err
		}
	}

	// Idea attributes
	hasIdeaAttrs := vals.IdeaCategory != "" || vals.IdeaEffort != "" || vals.IdeaValue != "" || vals.IdeaValidatedSet
	if hasIdeaAttrs {
		if task.Type != models.TaskTypeIdea {
			return fmt.Errorf("idea.* flags can only be used with idea type tasks")
		}
		if task.IdeaAttrs == nil {
			task.IdeaAttrs = &models.IdeaAttrs{}
		}
		if vals.IdeaCategory != "" {
			task.IdeaAttrs.Category = models.IdeaCategory(vals.IdeaCategory)
		}
		if vals.IdeaEffort != "" {
			task.IdeaAttrs.Effort = models.EffortEstimate(vals.IdeaEffort)
		}
		if vals.IdeaValue != "" {
			task.IdeaAttrs.Value = models.IdeaValue(vals.IdeaValue)
		}
		if vals.IdeaValidatedSet {
			task.IdeaAttrs.Validated = vals.IdeaValidated
		}
		if err := task.IdeaAttrs.Validate(); err != nil {
			return err
		}
	}

	// Task attributes
	hasTaskAttrs := vals.TaskEffort != "" || vals.TaskWorkType != ""
	if hasTaskAttrs {
		if task.Type != models.TaskTypeTask {
			return fmt.Errorf("task.* flags can only be used with task type tasks")
		}
		if task.TaskAttrs == nil {
			task.TaskAttrs = &models.TaskAttrs{}
		}
		if vals.TaskEffort != "" {
			task.TaskAttrs.Effort = models.EffortEstimate(vals.TaskEffort)
		}
		if vals.TaskWorkType != "" {
			task.TaskAttrs.WorkType = models.WorkType(vals.TaskWorkType)
		}
		if err := task.TaskAttrs.Validate(); err != nil {
			return err
		}
	}

	// Story attributes
	hasStoryAttrs := vals.StoryPointsSet || vals.StoryPersona != ""
	if hasStoryAttrs {
		if task.Type != models.TaskTypeStory {
			return fmt.Errorf("story.* flags can only be used with story type tasks")
		}
		if task.StoryAttrs == nil {
			task.StoryAttrs = &models.StoryAttrs{}
		}
		if vals.StoryPointsSet {
			task.StoryAttrs.StoryPoints = vals.StoryPoints
		}
		if vals.StoryPersona != "" {
			task.StoryAttrs.Persona = vals.StoryPersona
		}
		if err := task.StoryAttrs.Validate(); err != nil {
			return err
		}
	}

	// Epic attributes
	hasEpicAttrs := vals.EpicScope != "" || vals.EpicMilestone != "" || vals.EpicMetrics != ""
	if hasEpicAttrs {
		if task.Type != models.TaskTypeEpic {
			return fmt.Errorf("epic.* flags can only be used with epic type tasks")
		}
		if task.EpicAttrs == nil {
			task.EpicAttrs = &models.EpicAttrs{}
		}
		if vals.EpicScope != "" {
			task.EpicAttrs.Scope = models.EpicScope(vals.EpicScope)
		}
		if vals.EpicMilestone != "" {
			task.EpicAttrs.Milestone = vals.EpicMilestone
		}
		if vals.EpicMetrics != "" {
			task.EpicAttrs.SuccessMetrics = vals.EpicMetrics
		}
		if err := task.EpicAttrs.Validate(); err != nil {
			return err
		}
	}

	return nil
}
