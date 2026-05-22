package cmd

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSectionRename(t *testing.T) {
	t.Run("renames section successfully", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("OldName")
		deps.Repository.Projects[project.ID] = project

		err := runSectionRename(nil, []string{"OldName", "NewName"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if savedProject.HasSection("OldName") {
			t.Error("old section name still exists")
		}
		if !savedProject.HasSection("NewName") {
			t.Error("new section name was not created")
		}
	})

	t.Run("rejects renaming Next section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRename(nil, []string{models.SectionNext, "Something"})
		if err == nil {
			t.Error("runSectionRename() should error when trying to rename Next section")
		}
	})

	t.Run("rejects empty new name", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("Section")
		deps.Repository.Projects[project.ID] = project

		err := runSectionRename(nil, []string{"Section", ""})
		if err == nil {
			t.Error("runSectionRename() should error for empty new name")
		}
	})

	t.Run("rejects renaming nonexistent section", func(t *testing.T) {
		setupSectionCommandTest(t)

		err := runSectionRename(nil, []string{"Nonexistent", "NewName"})
		if err == nil {
			t.Error("runSectionRename() should error for nonexistent section")
		}
	})

	t.Run("rejects renaming to existing section", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("SectionA")
		project.EnsureSection("SectionB")
		deps.Repository.Projects[project.ID] = project

		err := runSectionRename(nil, []string{"SectionA", "SectionB"})
		if err == nil {
			t.Error("runSectionRename() should error when new name already exists")
		}
	})

	t.Run("rejects new name with spaces", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("MySection")
		deps.Repository.Projects[project.ID] = project

		err := runSectionRename(nil, []string{"MySection", "Has Space"})
		if err == nil {
			t.Error("runSectionRename() should error for new name with spaces")
		}
	})

	t.Run("updates task sections when renaming", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection("OldSection")
		task1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
		task1.Section = "OldSection"
		task2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)
		task2.Section = "OldSection"
		project.Tasks = append(project.Tasks, task1, task2)
		deps.Repository.Projects[project.ID] = project

		err := runSectionRename(nil, []string{"OldSection", "NewSection"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		for _, task := range savedProject.Tasks {
			if (task.ID == task1.ID || task.ID == task2.ID) && task.Section != "NewSection" {
				t.Errorf("task %s section = %s, want 'NewSection'", task.Title, task.Section)
			}
		}
	})
}

// TestSectionRenameRejectsGTDTarget is a regression test for a bug where
// `tt section rename Foo Inbox` succeeded, renaming a custom section to a
// built-in GTD name and creating a duplicate that conflicts with the
// built-in section in filters and views.
func TestSectionRenameRejectsGTDTarget(t *testing.T) {
	builtIns := []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}
	for _, name := range builtIns {
		t.Run("rejects target "+name, func(t *testing.T) {
			deps, project := setupSectionCommandTest(t)
			project.EnsureSection("Custom")
			deps.Repository.Projects[project.ID] = project

			err := runSectionRename(nil, []string{"Custom", name})
			if err == nil {
				t.Fatalf("runSectionRename to %q should error: built-in GTD name must be rejected as target", name)
			}
			saved := deps.Repository.Projects[project.ID]
			if saved == nil {
				return
			}
			// "Custom" should still exist after the failed rename. We don't assert
			// that the GTD name is absent from p.Sections because models.NewProject
			// pre-seeds p.Sections with SectionNext.
			foundCustom := slices.Contains(saved.Sections, "Custom")
			if !foundCustom {
				t.Fatalf("original section %q was lost on failed rename to %q", "Custom", name)
			}
		})
	}
}

func TestSectionRenameBuiltInSections(t *testing.T) {
	builtInSections := []string{
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}

	for _, section := range builtInSections {
		t.Run("rejects renaming built-in section "+section, func(t *testing.T) {
			setupSectionCommandTest(t)

			err := runSectionRename(nil, []string{section, "CustomName"})
			if err == nil {
				t.Errorf("runSectionRename() should error when trying to rename built-in section %q", section)
			}
		})
	}
}
