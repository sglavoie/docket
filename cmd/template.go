package cmd

import (
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"tpl"},
	Short:   "Manage task templates",
	Long: `Commands for creating, listing, and managing task templates.

Task templates provide pre-configured task structures for common scenarios
like bug reports, feature requests, and release checklists.

EXAMPLES

  Initialize default templates:
    tt template init

  List all templates:
    tt template list

  Enable a template:
    tt template toggle def-bug-report

  Create a task from a template:
    tt template use def-bug-report "Login fails on mobile"

  Add a custom template:
    tt template add -n "My Workflow" --type task --item "Step 1" --item "Step 2"

DEFAULT TEMPLATES (Category: Bug)
  Bug Report          - Bug with checklist: Steps to Reproduce, Expected/Actual Behavior, Environment
  Production Incident - P1 Bug with checklist: Impact, Root Cause, Mitigation, Post-mortem

DEFAULT TEMPLATES (Category: Feature)
  Feature Request     - Story with checklist: User Story, Acceptance Criteria, Technical Notes
  API Endpoint        - Task with checklist: Define Contract, Implement Handler, Add Tests, Update Docs
  UI Component        - Task with checklist: Design Review, Implementation, Unit Tests, Accessibility

DEFAULT TEMPLATES (Category: Process)
  Code Review Checklist - Task with checklist: Logic, Tests, Edge Cases, Performance, Docs
  Release Checklist     - Task with checklist: Run Tests, Changelog, Tag, Deploy Staging, Smoke Tests, Deploy Prod
  Sprint Planning       - Epic with checklist: Review Backlog, Estimate, Assign, Define Goals`,
	RunE: runTemplateList,
}

func init() {
	rootCmd.AddCommand(templateCmd)
}
