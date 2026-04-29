package models

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// BugReproducible represents whether a bug can be reproduced.
type BugReproducible string

const (
	BugReproducibleYes       BugReproducible = "yes"
	BugReproducibleNo        BugReproducible = "no"
	BugReproducibleSometimes BugReproducible = "sometimes"
)

// ValidBugReproducible contains all valid BugReproducible values.
var ValidBugReproducible = []BugReproducible{
	BugReproducibleYes,
	BugReproducibleNo,
	BugReproducibleSometimes,
}

// IsValid returns true if the value is a valid BugReproducible.
func (r BugReproducible) IsValid() bool { return isValidEnum(r, ValidBugReproducible) }

// BugSeverity represents the severity level of a bug.
type BugSeverity string

const (
	BugSeverityCritical BugSeverity = "critical"
	BugSeverityMajor    BugSeverity = "major"
	BugSeverityMinor    BugSeverity = "minor"
	BugSeverityCosmetic BugSeverity = "cosmetic"
)

// ValidBugSeverity contains all valid BugSeverity values.
var ValidBugSeverity = []BugSeverity{
	BugSeverityCritical,
	BugSeverityMajor,
	BugSeverityMinor,
	BugSeverityCosmetic,
}

// IsValid returns true if the value is a valid BugSeverity.
func (s BugSeverity) IsValid() bool { return isValidEnum(s, ValidBugSeverity) }

// BugAttrs contains bug-specific attributes.
type BugAttrs struct {
	Reproducible BugReproducible `json:"reproducible,omitempty"`
	Severity     BugSeverity     `json:"severity,omitempty"`
	Workaround   string          `json:"workaround,omitempty"` // free text, supports multi-line
}

// Validate checks if all set fields have valid values.
func (a *BugAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.Reproducible != "" && !a.Reproducible.IsValid() {
		return fmt.Errorf("invalid bug.reproducible: %q (valid: %s)", a.Reproducible, joinEnum(ValidBugReproducible))
	}
	if a.Severity != "" && !a.Severity.IsValid() {
		return fmt.Errorf("invalid bug.severity: %q (valid: %s)", a.Severity, joinEnum(ValidBugSeverity))
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *BugAttrs) IsEmpty() bool {
	return a == nil || (a.Reproducible == "" && a.Severity == "" && a.Workaround == "")
}

// IdeaCategory represents the category of an idea.
type IdeaCategory string

const (
	IdeaCategoryFeature     IdeaCategory = "feature"
	IdeaCategoryImprovement IdeaCategory = "improvement"
	IdeaCategoryRefactor    IdeaCategory = "refactor"
	IdeaCategoryResearch    IdeaCategory = "research"
)

// ValidIdeaCategory contains all valid IdeaCategory values.
var ValidIdeaCategory = []IdeaCategory{
	IdeaCategoryFeature,
	IdeaCategoryImprovement,
	IdeaCategoryRefactor,
	IdeaCategoryResearch,
}

// IsValid returns true if the value is a valid IdeaCategory.
func (c IdeaCategory) IsValid() bool { return isValidEnum(c, ValidIdeaCategory) }

// EffortEstimate represents the effort level for an idea or task.
type EffortEstimate string

const (
	EffortSmall   EffortEstimate = "small"
	EffortMedium  EffortEstimate = "medium"
	EffortLarge   EffortEstimate = "large"
	EffortUnknown EffortEstimate = "unknown"
)

// ValidEffortEstimate contains all valid EffortEstimate values.
var ValidEffortEstimate = []EffortEstimate{
	EffortSmall,
	EffortMedium,
	EffortLarge,
	EffortUnknown,
}

// IsValid returns true if the value is a valid EffortEstimate.
func (e EffortEstimate) IsValid() bool { return isValidEnum(e, ValidEffortEstimate) }

// IdeaValue represents the value assessment of an idea.
type IdeaValue string

const (
	IdeaValueLow    IdeaValue = "low"
	IdeaValueMedium IdeaValue = "medium"
	IdeaValueHigh   IdeaValue = "high"
)

// ValidIdeaValue contains all valid IdeaValue values.
var ValidIdeaValue = []IdeaValue{
	IdeaValueLow,
	IdeaValueMedium,
	IdeaValueHigh,
}

// IsValid returns true if the value is a valid IdeaValue.
func (v IdeaValue) IsValid() bool { return isValidEnum(v, ValidIdeaValue) }

// IdeaAttrs contains idea-specific attributes.
type IdeaAttrs struct {
	Category  IdeaCategory   `json:"category,omitempty"`
	Effort    EffortEstimate `json:"effort,omitempty"`
	Value     IdeaValue      `json:"value,omitempty"`
	Validated bool           `json:"validated,omitempty"`
}

// Validate checks if all set fields have valid values.
func (a *IdeaAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.Category != "" && !a.Category.IsValid() {
		return fmt.Errorf("invalid idea.category: %q (valid: %s)", a.Category, joinEnum(ValidIdeaCategory))
	}
	if a.Effort != "" && !a.Effort.IsValid() {
		return fmt.Errorf("invalid idea.effort: %q (valid: %s)", a.Effort, joinEnum(ValidEffortEstimate))
	}
	if a.Value != "" && !a.Value.IsValid() {
		return fmt.Errorf("invalid idea.value: %q (valid: %s)", a.Value, joinEnum(ValidIdeaValue))
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *IdeaAttrs) IsEmpty() bool {
	return a == nil || (a.Category == "" && a.Effort == "" && a.Value == "" && !a.Validated)
}

// WorkType represents the type of work for a task.
type WorkType string

const (
	WorkTypeDevelopment   WorkType = "development"
	WorkTypeDocumentation WorkType = "documentation"
	WorkTypeTesting       WorkType = "testing"
	WorkTypeDevOps        WorkType = "devops"
	WorkTypeDesign        WorkType = "design"
)

// ValidWorkType contains all valid WorkType values.
var ValidWorkType = []WorkType{
	WorkTypeDevelopment,
	WorkTypeDocumentation,
	WorkTypeTesting,
	WorkTypeDevOps,
	WorkTypeDesign,
}

// IsValid returns true if the value is a valid WorkType.
func (w WorkType) IsValid() bool { return isValidEnum(w, ValidWorkType) }

// TaskAttrs contains task-specific attributes (for TaskTypeTask).
type TaskAttrs struct {
	Effort   EffortEstimate `json:"effort,omitempty"`
	WorkType WorkType       `json:"worktype,omitempty"`
}

// Validate checks if all set fields have valid values.
func (a *TaskAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.Effort != "" && !a.Effort.IsValid() {
		return fmt.Errorf("invalid task.effort: %q (valid: %s)", a.Effort, joinEnum(ValidEffortEstimate))
	}
	if a.WorkType != "" && !a.WorkType.IsValid() {
		return fmt.Errorf("invalid task.worktype: %q (valid: %s)", a.WorkType, joinEnum(ValidWorkType))
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *TaskAttrs) IsEmpty() bool {
	return a == nil || (a.Effort == "" && a.WorkType == "")
}

// StoryAttrs contains story-specific attributes.
type StoryAttrs struct {
	StoryPoints int    `json:"story_points,omitempty"`
	Persona     string `json:"persona,omitempty"` // free text, supports multi-line
}

// Validate checks if all set fields have valid values.
func (a *StoryAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.StoryPoints < 0 {
		return fmt.Errorf("invalid story.points: %d (must be non-negative)", a.StoryPoints)
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *StoryAttrs) IsEmpty() bool {
	return a == nil || (a.StoryPoints == 0 && a.Persona == "")
}

// EpicScope represents the definition level of an epic's scope.
type EpicScope string

const (
	EpicScopeDefined   EpicScope = "defined"
	EpicScopeEvolving  EpicScope = "evolving"
	EpicScopeUndefined EpicScope = "undefined"
)

// ValidEpicScope contains all valid EpicScope values.
var ValidEpicScope = []EpicScope{
	EpicScopeDefined,
	EpicScopeEvolving,
	EpicScopeUndefined,
}

// IsValid returns true if the value is a valid EpicScope.
func (s EpicScope) IsValid() bool { return isValidEnum(s, ValidEpicScope) }

// EpicAttrs contains epic-specific attributes.
type EpicAttrs struct {
	Scope             EpicScope  `json:"scope,omitempty"`
	Milestone         string     `json:"milestone,omitempty"`           // free text
	SuccessMetrics    string     `json:"success_metrics,omitempty"`     // free text, supports multi-line
	InitialChildCount int        `json:"initial_child_count,omitempty"` // set by explicit baseline action
	ScopeBaselinedAt  *time.Time `json:"scope_baselined_at,omitempty"`  // when the baseline was last set
}

// Validate checks if all set fields have valid values.
func (a *EpicAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.Scope != "" && !a.Scope.IsValid() {
		return fmt.Errorf("invalid epic.scope: %q (valid: %s)", a.Scope, joinEnum(ValidEpicScope))
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *EpicAttrs) IsEmpty() bool {
	return a == nil || (a.Scope == "" && a.Milestone == "" && a.SuccessMetrics == "" &&
		a.InitialChildCount == 0 && a.ScopeBaselinedAt == nil)
}

// Lazy-init helpers that ensure a type-specific attrs struct exists on a Task.

// EnsureBugAttrs initializes BugAttrs if nil and returns it.
func (t *Task) EnsureBugAttrs() *BugAttrs {
	if t.BugAttrs == nil {
		t.BugAttrs = &BugAttrs{}
	}
	return t.BugAttrs
}

// EnsureIdeaAttrs initializes IdeaAttrs if nil and returns it.
func (t *Task) EnsureIdeaAttrs() *IdeaAttrs {
	if t.IdeaAttrs == nil {
		t.IdeaAttrs = &IdeaAttrs{}
	}
	return t.IdeaAttrs
}

// EnsureTaskAttrs initializes TaskAttrs if nil and returns it.
func (t *Task) EnsureTaskAttrs() *TaskAttrs {
	if t.TaskAttrs == nil {
		t.TaskAttrs = &TaskAttrs{}
	}
	return t.TaskAttrs
}

// EnsureStoryAttrs initializes StoryAttrs if nil and returns it.
func (t *Task) EnsureStoryAttrs() *StoryAttrs {
	if t.StoryAttrs == nil {
		t.StoryAttrs = &StoryAttrs{}
	}
	return t.StoryAttrs
}

// EnsureEpicAttrs initializes EpicAttrs if nil and returns it.
func (t *Task) EnsureEpicAttrs() *EpicAttrs {
	if t.EpicAttrs == nil {
		t.EpicAttrs = &EpicAttrs{}
	}
	return t.EpicAttrs
}

// ScopeDelta returns the change in child count since the scope was baselined.
// Returns (delta, true) when ScopeBaselinedAt is set, otherwise (0, false).
// A positive delta means scope has grown; negative means tasks were removed.
func (t *Task) ScopeDelta() (int, bool) {
	if t.EpicAttrs == nil || t.EpicAttrs.ScopeBaselinedAt == nil {
		return 0, false
	}
	return len(t.Children) - t.EpicAttrs.InitialChildCount, true
}

// Generic enum helpers.

func isValidEnum[T comparable](val T, valid []T) bool {
	return slices.Contains(valid, val)
}

func joinEnum[T ~string](valid []T) string {
	s := make([]string, len(valid))
	for i, v := range valid {
		s[i] = string(v)
	}
	return strings.Join(s, ", ")
}
