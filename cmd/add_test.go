package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestParseInlineMetadata(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTitle    string
		wantPriority int
		wantTags     []string
	}{
		{
			name:         "plain title",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "title with priority",
			input:        "Fix login bug !1",
			wantTitle:    "Fix login bug",
			wantPriority: 1,
			wantTags:     nil,
		},
		{
			name:         "priority in middle",
			input:        "Fix !2 login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 2,
			wantTags:     nil,
		},
		{
			name:         "title with tag",
			input:        "Fix login bug #backend",
			wantTitle:    "Fix login bug",
			wantPriority: 0,
			wantTags:     []string{"backend"},
		},
		{
			name:         "title with multiple tags",
			input:        "Add feature #backend #urgent",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"backend", "urgent"},
		},
		{
			name:         "title with priority and tags",
			input:        "Critical fix !1 #hotfix #backend",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"hotfix", "backend"},
		},
		{
			name:         "mixed order",
			input:        "#frontend Fix the button !3 #urgent",
			wantTitle:    "Fix the button",
			wantPriority: 3,
			wantTags:     []string{"frontend", "urgent"},
		},
		{
			name:         "priority at start",
			input:        "!5 Low priority task",
			wantTitle:    "Low priority task",
			wantPriority: 5,
			wantTags:     nil,
		},
		{
			name:         "invalid priority stripped with warning",
			input:        "Fix bug !9",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "tag with underscore",
			input:        "Add feature #code_review",
			wantTitle:    "Add feature",
			wantPriority: 0,
			wantTags:     []string{"code_review"},
		},
		{
			name:         "priority 0 stripped with warning",
			input:        "!0 not matched",
			wantTitle:    "not matched",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "priority 6 stripped with warning",
			input:        "!6 out of range",
			wantTitle:    "out of range",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "multiple spaces normalized",
			input:        "Multiple   spaces   here",
			wantTitle:    "Multiple spaces here",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "empty input",
			input:        "",
			wantTitle:    "",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "only metadata leaves empty title",
			input:        "!1 #tag",
			wantTitle:    "",
			wantPriority: 1,
			wantTags:     []string{"tag"},
		},
		{
			name:         "hash without word not a tag",
			input:        "Fix bug # comment",
			wantTitle:    "Fix bug # comment",
			wantPriority: 0,
			wantTags:     nil,
		},
		{
			name:         "tag with hyphen",
			input:        "Fix bug #my-tag",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantTags:     []string{"my-tag"},
		},
		{
			name:         "tag with dot",
			input:        "Release #v2.0",
			wantTitle:    "Release",
			wantPriority: 0,
			wantTags:     []string{"v2.0"},
		},
		{
			name:         "tag with hyphen and dot",
			input:        "Deploy #my-app.v2",
			wantTitle:    "Deploy",
			wantPriority: 0,
			wantTags:     []string{"my-app.v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotPriority, gotTags := parseInlineMetadata(tt.input)

			if gotTitle != tt.wantTitle {
				t.Errorf("parseInlineMetadata() title = %q, want %q", gotTitle, tt.wantTitle)
			}

			if gotPriority != tt.wantPriority {
				t.Errorf("parseInlineMetadata() priority = %d, want %d", gotPriority, tt.wantPriority)
			}

			if len(gotTags) != len(tt.wantTags) {
				t.Errorf("parseInlineMetadata() tags = %v, want %v", gotTags, tt.wantTags)
				return
			}

			for i, tag := range gotTags {
				if tag != tt.wantTags[i] {
					t.Errorf("parseInlineMetadata() tag[%d] = %q, want %q", i, tag, tt.wantTags[i])
				}
			}
		})
	}
}

func resetQuickAddVars() {
	addTypeFlag = ""
	addParentFlag = ""
	addPriority = 0
	addTags = nil
	addIsEpic = false
	addIsStory = false
	addIsBug = false
	addIsIdea = false
}

func TestRunQuickAdd(t *testing.T) {
	t.Run("creates task successfully", func(t *testing.T) {
		resetQuickAddVars()

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

		initialCount := len(project.Tasks)

		err = runQuickAdd(addCmd, []string{"Fix login bug"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Errorf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}

		newTask := savedProject.Tasks[len(savedProject.Tasks)-1]
		if newTask.Title != "Fix login bug" {
			t.Errorf("task title = %q, want %q", newTask.Title, "Fix login bug")
		}
		if newTask.Type != models.TaskTypeTask {
			t.Errorf("task type = %v, want %v", newTask.Type, models.TaskTypeTask)
		}
	})

	t.Run("rejects missing title", func(t *testing.T) {
		resetQuickAddVars()

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

		err = runQuickAdd(addCmd, []string{})
		if err == nil {
			t.Error("runQuickAdd() should error for missing title")
		}
	})

	t.Run("rejects empty title after metadata extraction", func(t *testing.T) {
		resetQuickAddVars()

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

		err = runQuickAdd(addCmd, []string{"!1 #tag"})
		if err == nil {
			t.Error("runQuickAdd() should error when title is empty after metadata extraction")
		}
	})

	t.Run("creates task with inline priority and tags", func(t *testing.T) {
		resetQuickAddVars()

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

		err = runQuickAdd(addCmd, []string{"Critical fix !1 #hotfix #backend"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		newTask := savedProject.Tasks[len(savedProject.Tasks)-1]

		if newTask.Title != "Critical fix" {
			t.Errorf("task title = %q, want %q", newTask.Title, "Critical fix")
		}
		if newTask.Priority != 1 {
			t.Errorf("task priority = %d, want 1", newTask.Priority)
		}
		if len(newTask.Tags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(newTask.Tags))
		}
		if newTask.Tags[0] != "hotfix" || newTask.Tags[1] != "backend" {
			t.Errorf("task tags = %v, want [hotfix backend]", newTask.Tags)
		}
	})

	t.Run("creates epic with flag", func(t *testing.T) {
		resetQuickAddVars()
		addIsEpic = true

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

		err = runQuickAdd(addCmd, []string{"User Authentication"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		newTask := savedProject.Tasks[len(savedProject.Tasks)-1]

		if newTask.Type != models.TaskTypeEpic {
			t.Errorf("task type = %v, want %v", newTask.Type, models.TaskTypeEpic)
		}
	})

	t.Run("explicit priority flag overrides inline", func(t *testing.T) {
		resetQuickAddVars()
		addPriority = 2

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

		err = runQuickAdd(addCmd, []string{"Task !1"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		newTask := savedProject.Tasks[len(savedProject.Tasks)-1]

		if newTask.Priority != 2 {
			t.Errorf("task priority = %d, want 2 (explicit flag should override inline !1)", newTask.Priority)
		}
	})

	t.Run("creates task with explicit tags", func(t *testing.T) {
		resetQuickAddVars()
		addTags = []string{"manual"}

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

		err = runQuickAdd(addCmd, []string{"Task #inline"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		newTask := savedProject.Tasks[len(savedProject.Tasks)-1]

		if len(newTask.Tags) != 2 {
			t.Fatalf("expected 2 tags, got %d: %v", len(newTask.Tags), newTask.Tags)
		}

		hasManual := false
		hasInline := false
		for _, tag := range newTask.Tags {
			if tag == "manual" {
				hasManual = true
			}
			if tag == "inline" {
				hasInline = true
			}
		}
		if !hasManual {
			t.Error("expected tag 'manual' from explicit flag")
		}
		if !hasInline {
			t.Error("expected tag 'inline' from inline parsing")
		}
	})
}
