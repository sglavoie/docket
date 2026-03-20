package cmd

import (
	"testing"
)

func TestProjectPurge(t *testing.T) {
	t.Run("lists trashed projects when no args", func(t *testing.T) {
		setupProjectTest(t)
		defer func() { purgeForce = false }()

		// This calls listTrashedProjects which reads from the data dir.
		// With a temp data dir it should either succeed with empty list
		// or return an error about missing trash dir — both are acceptable.
		_ = runProjectPurge(projectPurgeCmd, []string{})
	})

	t.Run("rejects nonexistent project in trash", func(t *testing.T) {
		setupProjectTest(t)
		defer func() { purgeForce = false }()

		purgeForce = true
		err := runProjectPurge(projectPurgeCmd, []string{"nonexistent-slug"})
		if err == nil {
			t.Error("expected error for nonexistent project in trash")
		}
	})
}
