// Package tui contains the TUI implementation for tt.
//
// This file is intentionally empty after refactoring.
// All save-related tests have been split into smaller, focused test files:
//   - task_form_save_validation_test.go  - validation error handling
//   - task_form_save_duedate_test.go     - due date parsing
//   - task_form_save_startdate_test.go   - start date parsing
//   - task_form_save_estimateddays_test.go - estimated days parsing
//   - task_form_save_attrs_test.go       - buildDraftTaskAttrs
//   - task_form_save_create_test.go      - create happy path tests
//   - task_form_save_edit_test.go        - edit happy path tests
//   - task_form_save_checklist_test.go   - checklist overlay test
package tui
