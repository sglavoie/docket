package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSearchFindsTasks(t *testing.T) {
	t.Run("finds matching tasks in title", func(t *testing.T) {
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

		task := testutil.NewTestTask("Unique searchable title", models.TaskTypeTask)
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		searchIncludeArchived = false
		searchLimit = 0
		defer func() {
			searchIncludeArchived = false
			searchLimit = 0
		}()

		err = runSearch(nil, []string{"Unique searchable"})
		testutil.AssertNoError(t, err)
	})

	t.Run("finds matching tasks in description", func(t *testing.T) {
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

		task := testutil.NewTestTask("Generic title", models.TaskTypeTask)
		task.Description = "Contains uniquedescriptionterm for testing"
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"uniquedescriptionterm"})
		testutil.AssertNoError(t, err)
	})

	t.Run("search is case-insensitive", func(t *testing.T) {
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

		task := testutil.NewTestTask("CamelCaseTitle", models.TaskTypeTask)
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"camelcasetitle"})
		testutil.AssertNoError(t, err)
	})
}

func TestSearchAcrossProjects(t *testing.T) {
	t.Run("searches multiple projects with --all flag", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		project1 := testutil.NewTestProject("Project One")
		project1.Path = deps.Config.DataDir + "/project-one"
		task1 := testutil.NewTestTask("Searchterm in project one", models.TaskTypeTask)
		project1.Tasks = append(project1.Tasks, task1)

		project2 := testutil.NewTestProject("Project Two")
		project2.Path = deps.Config.DataDir + "/project-two"
		project2.Archived = true
		task2 := testutil.NewTestTask("Searchterm in archived project", models.TaskTypeTask)
		project2.Tasks = append(project2.Tasks, task2)

		deps.Config.Projects = []*models.Project{project1, project2}
		deps.Config.CurrentProject = project1.ID
		deps.Repository.Projects[project1.ID] = project1
		deps.Repository.Projects[project2.ID] = project2

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		searchIncludeArchived = true
		searchLimit = 0
		defer func() {
			searchIncludeArchived = false
			searchLimit = 0
		}()

		err = runSearch(nil, []string{"Searchterm"})
		testutil.AssertNoError(t, err)
	})

	t.Run("excludes archived projects without --all flag", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		project1 := testutil.NewTestProject("Active Project")
		project1.Path = deps.Config.DataDir + "/active-project"
		task1 := testutil.NewTestTask("Activeonlyterm in active", models.TaskTypeTask)
		project1.Tasks = append(project1.Tasks, task1)

		project2 := testutil.NewTestProject("Archived Project")
		project2.Path = deps.Config.DataDir + "/archived-project"
		project2.Archived = true
		task2 := testutil.NewTestTask("Activeonlyterm in archived", models.TaskTypeTask)
		project2.Tasks = append(project2.Tasks, task2)

		deps.Config.Projects = []*models.Project{project1, project2}
		deps.Config.CurrentProject = project1.ID
		deps.Repository.Projects[project1.ID] = project1
		deps.Repository.Projects[project2.ID] = project2

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"Activeonlyterm"})
		testutil.AssertNoError(t, err)
	})
}

func TestSearchLimit(t *testing.T) {
	t.Run("respects --limit flag", func(t *testing.T) {
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

		for range 10 {
			task := testutil.NewTestTask("Limittest matching task", models.TaskTypeTask)
			project.Tasks = append(project.Tasks, task)
		}
		deps.Repository.Projects[project.ID] = project

		searchIncludeArchived = false
		searchLimit = 3
		defer func() {
			searchIncludeArchived = false
			searchLimit = 0
		}()

		err = runSearch(nil, []string{"Limittest"})
		testutil.AssertNoError(t, err)
	})
}

func TestSearchEdgeCases(t *testing.T) {
	t.Run("empty query returns early", func(t *testing.T) {
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

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"   "})
		if err == nil {
			t.Fatal("expected error for empty query, got nil")
		}
		if err.Error() != "search query cannot be empty" {
			t.Fatalf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("no matches returns appropriate message", func(t *testing.T) {
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

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"xyznonexistenttermxyz"})
		testutil.AssertNoError(t, err)
	})

	t.Run("no projects returns appropriate message", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		deps.Config.Projects = nil

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		searchIncludeArchived = false
		searchLimit = 0

		err = runSearch(nil, []string{"anything"})
		testutil.AssertNoError(t, err)
	})
}
