package cmd

import (
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
)

// resetTemplateUseVars resets all package-level flag variables used by runTemplateUse.
func resetTemplateUseVars() {
	templateUseSkipOptional = false
	templateUseParent = ""
	templateUseVars = nil
}

// setupTemplateUseTest creates common test dependencies and returns them.
func setupTemplateUseTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetTemplateUseVars()

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	t.Cleanup(resetTemplateUseVars)

	return deps, project
}

func TestParseVarFlags(t *testing.T) {
	t.Run("valid name=value flag", func(t *testing.T) {
		result, err := parseVarFlags([]string{"name=value"})
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, result["name"], "value")
	})

	t.Run("value containing equals sign", func(t *testing.T) {
		result, err := parseVarFlags([]string{"a=b=c"})
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, result["a"], "b=c")
	})

	t.Run("invalid format missing equals", func(t *testing.T) {
		_, err := parseVarFlags([]string{"noequals"})
		if err == nil {
			t.Error("expected error for flag missing '='")
		}
	})

	t.Run("empty slice returns empty map", func(t *testing.T) {
		result, err := parseVarFlags([]string{})
		testutil.AssertNoError(t, err)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("multiple valid flags", func(t *testing.T) {
		result, err := parseVarFlags([]string{"client=Acme", "env=staging"})
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, result["client"], "Acme")
		testutil.AssertEqual(t, result["env"], "staging")
	})
}

func TestRunTemplateUse(t *testing.T) {
	t.Run("template not found returns error", func(t *testing.T) {
		deps, _ := setupTemplateUseTest(t)
		defer deps.Cleanup()

		err := runTemplateUse(templateUseCmd, []string{"nonexistent-tmpl"})
		if err == nil {
			t.Error("expected error for template not found")
		}
	})

	t.Run("creates task from template with correct type, title, priority, tags", func(t *testing.T) {
		deps, project := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:       "test-tmpl",
			Name:     "Test Template",
			Enabled:  true,
			Type:     models.TaskTypeTask,
			Priority: 2,
			Tags:     []string{"backend"},
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		initialCount := len(project.Tasks)

		err := runTemplateUse(templateUseCmd, []string{"test-tmpl", "My Task Title"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Title, "My Task Title")
		testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
		testutil.AssertEqual(t, task.Priority, 2)
		if !task.HasTag("backend") {
			t.Error("task should have 'backend' tag from template")
		}
	})

	t.Run("uses template name as title when no title provided", func(t *testing.T) {
		deps, project := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "notitle-tmpl",
			Name:    "No Title Template",
			Enabled: true,
			Type:    models.TaskTypeTask,
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		initialCount := len(project.Tasks)

		err := runTemplateUse(templateUseCmd, []string{"notitle-tmpl"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Title, "No Title Template")
	})

	t.Run("creates checklist items from template.Checklist", func(t *testing.T) {
		deps, project := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:        "checklist-tmpl",
			Name:      "Checklist Template",
			Enabled:   true,
			Type:      models.TaskTypeTask,
			Checklist: []string{"Step 1", "Step 2"},
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		initialCount := len(project.Tasks)

		err := runTemplateUse(templateUseCmd, []string{"checklist-tmpl", "Checklist Task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		if len(task.Checklist) != 2 {
			t.Fatalf("expected 2 checklist items, got %d", len(task.Checklist))
		}
		testutil.AssertEqual(t, task.Checklist[0].Text, "Step 1")
		testutil.AssertEqual(t, task.Checklist[1].Text, "Step 2")
	})

	t.Run("creates subtasks from template.Children for Epic type", func(t *testing.T) {
		deps, project := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "epic-tmpl",
			Name:    "Epic Template",
			Enabled: true,
			Type:    models.TaskTypeEpic,
			Children: []template.TemplateChild{
				{Title: "Child Task 1"},
				{Title: "Optional Child", Optional: true},
			},
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		initialCount := len(project.Tasks)

		err := runTemplateUse(templateUseCmd, []string{"epic-tmpl", "My Epic"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		// 1 epic + 2 children
		if len(savedProject.Tasks) != initialCount+3 {
			t.Fatalf("expected %d tasks, got %d", initialCount+3, len(savedProject.Tasks))
		}
	})

	t.Run("skips optional children with templateUseSkipOptional=true", func(t *testing.T) {
		deps, project := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "epic-skip-tmpl",
			Name:    "Epic Skip Template",
			Enabled: true,
			Type:    models.TaskTypeEpic,
			Children: []template.TemplateChild{
				{Title: "Child Task 1"},
				{Title: "Optional Child", Optional: true},
			},
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		templateUseSkipOptional = true
		initialCount := len(project.Tasks)

		err := runTemplateUse(templateUseCmd, []string{"epic-skip-tmpl", "My Skipped Epic"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		// 1 epic + 1 non-optional child (optional skipped)
		if len(savedProject.Tasks) != initialCount+2 {
			t.Fatalf("expected %d tasks, got %d", initialCount+2, len(savedProject.Tasks))
		}
	})

	t.Run("rejects parent not found", func(t *testing.T) {
		deps, _ := setupTemplateUseTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "parent-tmpl",
			Name:    "Parent Template",
			Enabled: true,
			Type:    models.TaskTypeTask,
		}
		deps.Config.TaskTemplates = append(deps.Config.TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		templateUseParent = "99999"

		err := runTemplateUse(templateUseCmd, []string{"parent-tmpl", "Orphan Task"})
		if err == nil {
			t.Error("expected error for parent not found")
		}
	})
}

// TestResolveTemplateVars_NonInteractiveRequiredVar verifies that when stdin
// is not a terminal and a template variable has no default and was not
// provided via --var, the call fails loudly instead of silently substituting
// an empty string (which would produce a degenerate task).
func TestResolveTemplateVars_NonInteractiveRequiredVar(t *testing.T) {
	// Replace stdin with a closed pipe so isatty returns false.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close writer: %v", closeErr)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("close reader: %v", closeErr)
		}
	})

	vars := []template.TemplateVariable{
		{Name: "client", Default: ""}, // required, no default
	}

	_, err = resolveTemplateVars(vars, nil)
	if err == nil {
		t.Fatal("expected error when required variable lacks default in non-interactive mode")
	}
}

// TestResolveTemplateVars_NonInteractiveUsesDefault verifies that variables
// with defaults silently substitute when stdin is non-tty, matching prior
// behavior for the default-providing path.
func TestResolveTemplateVars_NonInteractiveUsesDefault(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close writer: %v", closeErr)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("close reader: %v", closeErr)
		}
	})

	vars := []template.TemplateVariable{
		{Name: "env", Default: "staging"},
	}

	values, err := resolveTemplateVars(vars, nil)
	testutil.AssertNoError(t, err)
	if values["env"] != "staging" {
		t.Errorf("expected env=staging from default, got %q", values["env"])
	}
}

// TestResolveTemplateVars_NonInteractiveFlagOverride verifies that --var
// supplied values satisfy variables without defaults under non-tty stdin.
func TestResolveTemplateVars_NonInteractiveFlagOverride(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close writer: %v", closeErr)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("close reader: %v", closeErr)
		}
	})

	vars := []template.TemplateVariable{
		{Name: "client", Default: ""},
	}

	values, err := resolveTemplateVars(vars, []string{"client=Acme"})
	testutil.AssertNoError(t, err)
	if values["client"] != "Acme" {
		t.Errorf("expected client=Acme from --var, got %q", values["client"])
	}
}
