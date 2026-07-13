package models

import "fmt"

// TypeAttrValues holds type-specific attribute values for a task, decoupled from
// any presentation layer. Boolean/int attributes that have zero-value defaults use
// a companion *Set field so a caller can distinguish "explicitly set to the zero
// value" from "not provided". It is plain data so both the CLI adapter (which
// binds it to flags) and the application layer (typed task capture) apply the same
// attributes through ApplyTypeAttrValues.
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

// ApplyTypeAttrValues applies type-specific attributes to a task. It ensures the
// appropriate attrs struct exists (creating it if nil), assigns the provided
// values, and validates the result. A value for a type that does not match the
// task's type is rejected so a mismatched attribute never persists.
func ApplyTypeAttrValues(task *Task, vals TypeAttrValues) error { //nolint:gocyclo
	// Bug attributes
	hasBugAttrs := vals.BugReproducible != "" || vals.BugSeverity != "" || vals.BugWorkaround != ""
	if hasBugAttrs {
		if task.Type != TaskTypeBug {
			return fmt.Errorf("bug.* attributes can only be used with bug type tasks")
		}
		a := task.EnsureBugAttrs()
		if vals.BugReproducible != "" {
			a.Reproducible = BugReproducible(vals.BugReproducible)
		}
		if vals.BugSeverity != "" {
			a.Severity = BugSeverity(vals.BugSeverity)
		}
		if vals.BugWorkaround != "" {
			a.Workaround = vals.BugWorkaround
		}
		if err := a.Validate(); err != nil {
			return err
		}
	}

	// Idea attributes
	hasIdeaAttrs := vals.IdeaCategory != "" || vals.IdeaEffort != "" || vals.IdeaValue != "" || vals.IdeaValidatedSet
	if hasIdeaAttrs {
		if task.Type != TaskTypeIdea {
			return fmt.Errorf("idea.* attributes can only be used with idea type tasks")
		}
		a := task.EnsureIdeaAttrs()
		if vals.IdeaCategory != "" {
			a.Category = IdeaCategory(vals.IdeaCategory)
		}
		if vals.IdeaEffort != "" {
			a.Effort = EffortEstimate(vals.IdeaEffort)
		}
		if vals.IdeaValue != "" {
			a.Value = IdeaValue(vals.IdeaValue)
		}
		if vals.IdeaValidatedSet {
			a.Validated = vals.IdeaValidated
		}
		if err := a.Validate(); err != nil {
			return err
		}
	}

	// Task attributes
	hasTaskAttrs := vals.TaskEffort != "" || vals.TaskWorkType != ""
	if hasTaskAttrs {
		if task.Type != TaskTypeTask {
			return fmt.Errorf("task.* attributes can only be used with task type tasks")
		}
		a := task.EnsureTaskAttrs()
		if vals.TaskEffort != "" {
			a.Effort = EffortEstimate(vals.TaskEffort)
		}
		if vals.TaskWorkType != "" {
			a.WorkType = WorkType(vals.TaskWorkType)
		}
		if err := a.Validate(); err != nil {
			return err
		}
	}

	// Story attributes
	hasStoryAttrs := vals.StoryPointsSet || vals.StoryPersona != ""
	if hasStoryAttrs {
		if task.Type != TaskTypeStory {
			return fmt.Errorf("story.* attributes can only be used with story type tasks")
		}
		a := task.EnsureStoryAttrs()
		if vals.StoryPointsSet {
			a.StoryPoints = vals.StoryPoints
		}
		if vals.StoryPersona != "" {
			a.Persona = vals.StoryPersona
		}
		if err := a.Validate(); err != nil {
			return err
		}
	}

	// Epic attributes
	hasEpicAttrs := vals.EpicScope != "" || vals.EpicMilestone != "" || vals.EpicMetrics != ""
	if hasEpicAttrs {
		if task.Type != TaskTypeEpic {
			return fmt.Errorf("epic.* attributes can only be used with epic type tasks")
		}
		a := task.EnsureEpicAttrs()
		if vals.EpicScope != "" {
			a.Scope = EpicScope(vals.EpicScope)
		}
		if vals.EpicMilestone != "" {
			a.Milestone = vals.EpicMilestone
		}
		if vals.EpicMetrics != "" {
			a.SuccessMetrics = vals.EpicMetrics
		}
		if err := a.Validate(); err != nil {
			return err
		}
	}

	return nil
}
