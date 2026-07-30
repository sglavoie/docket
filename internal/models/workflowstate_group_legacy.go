package models

// StateGroupReview and StateThemeReview retain the v4 presentation identifiers
// for appearance readers and unremoved rendering adapters. They are absent from
// target group enumeration and validation.
const (
	StateGroupReview WorkflowStateGroup     = "review"
	StateThemeReview WorkflowStateThemeRole = "review"
)
