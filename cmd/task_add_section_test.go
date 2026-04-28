package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunTaskAdd_SectionValidation(t *testing.T) {
	t.Run("rejects nonexistent section without --create-section", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskSectionFlag = "Nonexistent"
		_ = taskAddCmd.Flags().Set("section", "Nonexistent")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		if err == nil {
			t.Error("expected error for nonexistent section without --create-section")
		}
	})

	t.Run("accepts nonexistent section with --create-section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "NewSection"
		taskCreateSectionFlag = true
		_ = taskAddCmd.Flags().Set("section", "NewSection")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
			taskCreateSectionFlag = false
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		if !savedProject.HasSection("NewSection") {
			t.Error("expected NewSection to be created")
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.CustomSection, "NewSection")
		testutil.AssertEqual(t, task.Section, "")
	})

	t.Run("accepts existing GTD section without --create-section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "Next"
		_ = taskAddCmd.Flags().Set("section", "Next")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Section, "Next")
		testutil.AssertEqual(t, task.CustomSection, "")
	})
}