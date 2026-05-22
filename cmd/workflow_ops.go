// Package cmd contains the CLI command implementations for tt.
//
// Workflow-related functionality is split across multiple files:
//   - project_ops.go: Save pipeline workflow evaluation
//   - workflow_project_rules.go: Project-level rule evaluation and application
//   - workflow_flags.go: Helper functions for building workflow objects from CLI flags
//   - workflow_diff.go: Diff formatting and comparison utilities
//   - workflow_validation.go: Validation helper functions
package cmd
