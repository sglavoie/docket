package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunTaskAdd_SectionValidation(t *testing.T) {
	t.Run("rejects non-GTD section", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskSectionFlag = "Backlog"
		_ = taskAddCmd.Flags().Set("section", "Backlog")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		if err == nil {
			t.Error("expected error for non-GTD section")
		}
	})

	t.Run("does not create non-GTD section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "NewSection"
		_ = taskAddCmd.Flags().Set("section", "NewSection")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		if err == nil {
			t.Fatal("expected error for non-GTD section")
		}

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount {
			t.Fatalf("expected %d tasks, got %d", initialCount, len(savedProject.Tasks))
		}
		if savedProject.HasSection("NewSection") {
			t.Error("expected NewSection not to be created")
		}
	})

	t.Run("accepts existing GTD section", func(t *testing.T) {
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
	})

	t.Run("accepts explicit empty section as active view", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = ""
		_ = taskAddCmd.Flags().Set("section", "")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My active task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Section, "")
	})
}
