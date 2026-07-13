package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

// TypeAttrValues is the CLI alias for the presentation-neutral attribute set the
// models layer applies. The CLI binds it to flags via registerTypeAttrFlags and
// hands it to the application layer, which applies the same attributes through
// models.ApplyTypeAttrValues so CLI and typed capture stay in parity.
type TypeAttrValues = models.TypeAttrValues

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

// applyTypeAttrValues applies type-specific attributes to a task through the
// shared models implementation, keeping the CLI call sites unchanged.
func applyTypeAttrValues(task *models.Task, vals TypeAttrValues) error {
	return models.ApplyTypeAttrValues(task, vals)
}
