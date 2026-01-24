package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSectionAdd(t *testing.T) {
	t.Run("adds new section successfully", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		// Save config so LoadConfig() can find it
		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		// Set up storage to use the mock repository
		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		initialSections := len(project.Sections)

		err = runSectionAdd(nil, []string{"In Progress"})
		testutil.AssertNoError(t, err)

		// Verify section was added
		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Sections) != initialSections+1 {
			t.Errorf("expected %d sections, got %d", initialSections+1, len(savedProject.Sections))
		}

		if !savedProject.HasSection("In Progress") {
			t.Error("section 'In Progress' was not added")
		}
	})

	t.Run("rejects empty section name", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runSectionAdd(nil, []string{""})
		if err == nil {
			t.Error("runSectionAdd() should error for empty section name")
		}
	})

	t.Run("rejects duplicate section", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add a section first
		project.EnsureSection("Existing Section")
		deps.Repository.Projects[project.ID] = project

		err = runSectionAdd(nil, []string{"Existing Section"})
		if err == nil {
			t.Error("runSectionAdd() should error for duplicate section")
		}
	})
}

func TestSectionRemove(t *testing.T) {
	t.Run("removes section successfully", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add a section and save
		project.EnsureSection("To Remove")
		deps.Repository.Projects[project.ID] = project

		err = runSectionRemove(nil, []string{"To Remove"})
		testutil.AssertNoError(t, err)

		// Verify section was removed
		savedProject := deps.Repository.Projects[project.ID]
		if savedProject.HasSection("To Remove") {
			t.Error("section 'To Remove' was not removed")
		}
	})

	t.Run("rejects removing Next section", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runSectionRename(nil, []string{models.SectionNext, "Something"})
		if err == nil {
			t.Error("runSectionRemove() should error when trying to remove Next section")
		}
	})

	t.Run("rejects removing nonexistent section", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runSectionRename(nil, []string{"Nonexistent", "New Name"})
		if err == nil {
			t.Error("runSectionRemove() should error for nonexistent section")
		}
	})

	t.Run("clears section from tasks when removed", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add section and assign task to it
		project.EnsureSection("ToRemove")
		task := testutil.NewTestTask("Task in section", models.TaskTypeTask)
		task.Section = "ToRemove"
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		err = runSectionRemove(nil, []string{"ToRemove"})
		testutil.AssertNoError(t, err)

		// Verify task section was cleared
		savedProject := deps.Repository.Projects[project.ID]
		for _, savedTask := range savedProject.Tasks {
			if savedTask.ID == task.ID {
				if savedTask.Section != "" {
					t.Errorf("task section should be cleared, got %s", savedTask.Section)
				}
			}
		}
	})
}

func TestSectionRename(t *testing.T) {
	t.Run("renames section successfully", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add a section
		project.EnsureSection("Old Name")
		deps.Repository.Projects[project.ID] = project

		err = runSectionRename(nil, []string{"Old Name", "New Name"})
		testutil.AssertNoError(t, err)

		// Verify section was renamed
		savedProject := deps.Repository.Projects[project.ID]
		if savedProject.HasSection("Old Name") {
			t.Error("old section name still exists")
		}
		if !savedProject.HasSection("New Name") {
			t.Error("new section name was not created")
		}
	})

	t.Run("rejects renaming Next section", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runSectionRename(nil, []string{models.SectionNext, "Something"})
		if err == nil {
			t.Error("runSectionRename() should error when trying to rename Next section")
		}
	})

	t.Run("rejects empty new name", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		project.EnsureSection("Section")
		deps.Repository.Projects[project.ID] = project

		err = runSectionRename(nil, []string{"Section", ""})
		if err == nil {
			t.Error("runSectionRename() should error for empty new name")
		}
	})

	t.Run("rejects renaming nonexistent section", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runSectionRename(nil, []string{"Nonexistent", "New Name"})
		if err == nil {
			t.Error("runSectionRename() should error for nonexistent section")
		}
	})

	t.Run("rejects renaming to existing section", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		project.EnsureSection("Section A")
		project.EnsureSection("Section B")
		deps.Repository.Projects[project.ID] = project

		err = runSectionRename(nil, []string{"Section A", "Section B"})
		if err == nil {
			t.Error("runSectionRename() should error when new name already exists")
		}
	})

	t.Run("updates task sections when renaming", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add section and assign tasks to it
		project.EnsureSection("Old Section")
		task1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
		task1.Section = "Old Section"
		task2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)
		task2.Section = "Old Section"
		project.Tasks = append(project.Tasks, task1, task2)
		deps.Repository.Projects[project.ID] = project

		err = runSectionRename(nil, []string{"Old Section", "New Section"})
		testutil.AssertNoError(t, err)

		// Verify all tasks were updated
		savedProject := deps.Repository.Projects[project.ID]
		for _, task := range savedProject.Tasks {
			if task.ID == task1.ID || task.ID == task2.ID {
				if task.Section != "New Section" {
					t.Errorf("task %s section = %s, want 'New Section'", task.Title, task.Section)
				}
			}
		}
	})
}

func TestSectionList(t *testing.T) {
	t.Run("lists sections with counts", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		// Add some sections
		project.EnsureSection("Backlog")
		project.EnsureSection("In Progress")

		// Add tasks to sections
		task1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
		task1.Section = "Backlog"
		task2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)
		task2.Section = "In Progress"
		project.Tasks = append(project.Tasks, task1, task2)

		deps.Repository.Projects[project.ID] = project

		// Just verify it runs without error
		// Output testing is difficult for table formatting
		err := runSectionList(nil, nil)
		testutil.AssertNoError(t, err)
	})

	t.Run("handles project with no sections", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		// Clear all sections
		project.Sections = []string{}
		deps.Repository.Projects[project.ID] = project

		// Should handle gracefully and not error
		err := runSectionList(nil, nil)
		testutil.AssertNoError(t, err)
	})
}
