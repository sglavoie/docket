package cmd

import (
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