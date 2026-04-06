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
}