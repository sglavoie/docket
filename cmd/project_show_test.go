package cmd

import (
	"testing"
)

func TestProjectShow(t *testing.T) {
	t.Run("runs without error for current project", func(t *testing.T) {
		setupProjectTest(t)

		err := runProjectShow(projectShowCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("archived project with nil ArchivedAt does not panic", func(t *testing.T) {
		deps, project := setupProjectTest(t)
		// Simulate legacy archived project with nil ArchivedAt
		project.Archived = true
		project.ArchivedAt = nil
		deps.Config.CurrentProject = project.ID

		// This should not panic
		err := runProjectShow(projectShowCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
