package cmd

import (
	"testing"
)

func TestProjectShow(t *testing.T) {
	t.Run("runs without error for current project", func(t *testing.T) {
		setupProjectTest(t)
		defer func() { projectShowJSON = false }()

		err := runProjectShow(projectShowCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
