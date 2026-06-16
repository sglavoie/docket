package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSectionList(t *testing.T) {
	t.Run("lists sections with counts", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.EnsureSection(models.SectionInbox)
		project.EnsureSection(models.SectionWaiting)

		task1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
		task1.Section = models.SectionInbox
		task2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)
		task2.Section = models.SectionWaiting
		project.Tasks = append(project.Tasks, task1, task2)

		deps.Repository.Projects[project.ID] = project

		err := runSectionList(nil, nil)
		testutil.AssertNoError(t, err)
	})

	t.Run("handles project with no sections", func(t *testing.T) {
		deps, project := setupSectionCommandTest(t)

		project.Sections = []string{}
		deps.Repository.Projects[project.ID] = project

		err := runSectionList(nil, nil)
		testutil.AssertNoError(t, err)
	})
}
