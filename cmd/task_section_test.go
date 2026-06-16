package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskSectionSkipsAlreadyAssigned(t *testing.T) {
	t.Run("does not save when GTD section already matches", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = models.SectionWaiting
		deps.Repository.Projects[project.ID] = project

		originalUpdatedAt := task.UpdatedAt

		err := runTaskSection(nil, []string{"1", models.SectionWaiting})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.UpdatedAt != originalUpdatedAt {
			t.Error("UpdatedAt should not change when section already matches")
		}
	})

	t.Run("assigns GTD section to task in different section", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = models.SectionSomeday
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", models.SectionWaiting})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Section != models.SectionWaiting {
			t.Errorf("task section = %q, want %q", savedTask.Section, models.SectionWaiting)
		}
	})

	t.Run("rejects non-GTD section", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", "MyCustom"})
		if err == nil {
			t.Fatal("expected error for non-GTD section")
		}

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Section != task.Section {
			t.Errorf("task Section = %q, want unchanged %q", savedTask.Section, task.Section)
		}
	})

	t.Run("clearing section works when Section is already empty", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = ""
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", ""})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Section != "" {
			t.Errorf("task Section = %q, want empty after clearing", savedTask.Section)
		}
	})

	t.Run("rejects case-variant GTD section name instead of silently no-opping", func(t *testing.T) {
		// "inbox" (lowercase) is a case-variant of the canonical "Inbox" GTD section.
		// Without the case-variant guard at the CLI boundary, the command silently
		// reported success ("Moved task #X from 'Inbox' to 'inbox'") while leaving
		// task.Section untouched and corrupting the user's mental model of what
		// just happened.
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = models.SectionInbox
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", "inbox"})
		if err == nil {
			t.Fatalf("expected error for case-variant GTD section name %q, got nil", "inbox")
		}

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Section != models.SectionInbox {
			t.Errorf("task Section = %q, want %q (must be unchanged)", savedTask.Section, models.SectionInbox)
		}
		// project.Sections must not have gained a phantom "inbox" entry.
		for _, s := range savedProject.Sections {
			if s == "inbox" {
				t.Errorf("project.Sections gained phantom entry %q from case-variant input", s)
			}
		}
	})

	t.Run("clearing section skips when Section is already empty", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = ""
		deps.Repository.Projects[project.ID] = project

		originalUpdatedAt := task.UpdatedAt

		err := runTaskSection(nil, []string{"1", ""})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.UpdatedAt != originalUpdatedAt {
			t.Error("UpdatedAt should not change when section is already empty")
		}
	})
}
