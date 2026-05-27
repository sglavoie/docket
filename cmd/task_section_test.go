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

	t.Run("does not save when custom section already matches", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		project.EnsureSection("Active")
		task.CustomSection = "Active"
		deps.Repository.Projects[project.ID] = project

		originalUpdatedAt := task.UpdatedAt

		err := runTaskSection(nil, []string{"1", "Active"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.UpdatedAt != originalUpdatedAt {
			t.Error("UpdatedAt should not change when custom section already matches")
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

	t.Run("assigns custom section to CustomSection field", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		project.EnsureSection("MyCustom")
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", "MyCustom"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.CustomSection != "MyCustom" {
			t.Errorf("task CustomSection = %q, want %q", savedTask.CustomSection, "MyCustom")
		}
	})

	t.Run("clearing section clears CustomSection when Section is already empty", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		// Task has a custom section but Section field is empty
		task := project.Tasks[0]
		task.Section = ""
		task.CustomSection = "Sprint-1"
		project.EnsureSection("Sprint-1")
		deps.Repository.Projects[project.ID] = project

		err := runTaskSection(nil, []string{"1", ""})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.CustomSection != "" {
			t.Errorf("task CustomSection = %q, want empty after clearing", savedTask.CustomSection)
		}
		if savedTask.Section != "" {
			t.Errorf("task Section = %q, want empty after clearing", savedTask.Section)
		}
	})

	t.Run("rejects case-variant GTD section name instead of silently no-opping", func(t *testing.T) {
		// "inbox" (lowercase) is a case-variant of the canonical "Inbox" GTD section.
		// Without the case-variant guard at the CLI boundary, the command silently
		// reported success ("Moved task #X from 'Inbox' to 'inbox'") while leaving
		// task.Section/CustomSection untouched — corrupting the user's mental model
		// of what just happened.
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = models.SectionInbox
		task.CustomSection = ""
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
		if savedTask.CustomSection != "" {
			t.Errorf("task CustomSection = %q, want empty (case-variant must not leak into CustomSection)", savedTask.CustomSection)
		}
		// project.Sections must not have gained a phantom "inbox" entry.
		for _, s := range savedProject.Sections {
			if s == "inbox" {
				t.Errorf("project.Sections gained phantom entry %q from case-variant input", s)
			}
		}
	})

	t.Run("clearing section skips when both Section and CustomSection are already empty", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		task := project.Tasks[0]
		task.Section = ""
		task.CustomSection = ""
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
