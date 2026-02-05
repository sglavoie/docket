package models

import (
	"fmt"
	"strings"
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
func (r BugReproducible) IsValid() bool {
	for _, v := range ValidBugReproducible {
		if r == v {
			return true
		}
	}
	return false
}

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
func (s BugSeverity) IsValid() bool {
	for _, v := range ValidBugSeverity {
		if s == v {
			return true
		}
	}
	return false
}

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
		return fmt.Errorf("invalid bug.reproducible: %q (valid: %s)", a.Reproducible, joinBugReproducible())
	}
	if a.Severity != "" && !a.Severity.IsValid() {
		return fmt.Errorf("invalid bug.severity: %q (valid: %s)", a.Severity, joinBugSeverity())
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
func (c IdeaCategory) IsValid() bool {
	for _, v := range ValidIdeaCategory {
		if c == v {
			return true
		}
	}
	return false
}

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
func (e EffortEstimate) IsValid() bool {
	for _, v := range ValidEffortEstimate {
		if e == v {
			return true
		}
	}
	return false
}

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
func (v IdeaValue) IsValid() bool {
	for _, val := range ValidIdeaValue {
		if v == val {
			return true
		}
	}
	return false
}

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
		return fmt.Errorf("invalid idea.category: %q (valid: %s)", a.Category, joinIdeaCategory())
	}
	if a.Effort != "" && !a.Effort.IsValid() {
		return fmt.Errorf("invalid idea.effort: %q (valid: %s)", a.Effort, joinEffortEstimate())
	}
	if a.Value != "" && !a.Value.IsValid() {
		return fmt.Errorf("invalid idea.value: %q (valid: %s)", a.Value, joinIdeaValue())
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
func (w WorkType) IsValid() bool {
	for _, v := range ValidWorkType {
		if w == v {
			return true
		}
	}
	return false
}

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
		return fmt.Errorf("invalid task.effort: %q (valid: %s)", a.Effort, joinEffortEstimate())
	}
	if a.WorkType != "" && !a.WorkType.IsValid() {
		return fmt.Errorf("invalid task.worktype: %q (valid: %s)", a.WorkType, joinWorkType())
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
func (s EpicScope) IsValid() bool {
	for _, v := range ValidEpicScope {
		if s == v {
			return true
		}
	}
	return false
}

// EpicAttrs contains epic-specific attributes.
type EpicAttrs struct {
	Scope          EpicScope `json:"scope,omitempty"`
	Milestone      string    `json:"milestone,omitempty"`       // free text
	SuccessMetrics string    `json:"success_metrics,omitempty"` // free text, supports multi-line
}

// Validate checks if all set fields have valid values.
func (a *EpicAttrs) Validate() error {
	if a == nil {
		return nil
	}
	if a.Scope != "" && !a.Scope.IsValid() {
		return fmt.Errorf("invalid epic.scope: %q (valid: %s)", a.Scope, joinEpicScope())
	}
	return nil
}

// IsEmpty returns true if all fields are empty/zero values.
func (a *EpicAttrs) IsEmpty() bool {
	return a == nil || (a.Scope == "" && a.Milestone == "" && a.SuccessMetrics == "")
}

// Helper functions to join valid values for error messages.

func joinBugReproducible() string {
	var s []string
	for _, v := range ValidBugReproducible {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinBugSeverity() string {
	var s []string
	for _, v := range ValidBugSeverity {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinIdeaCategory() string {
	var s []string
	for _, v := range ValidIdeaCategory {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinEffortEstimate() string {
	var s []string
	for _, v := range ValidEffortEstimate {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinIdeaValue() string {
	var s []string
	for _, v := range ValidIdeaValue {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinWorkType() string {
	var s []string
	for _, v := range ValidWorkType {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}

func joinEpicScope() string {
	var s []string
	for _, v := range ValidEpicScope {
		s = append(s, string(v))
	}
	return strings.Join(s, ", ")
}
