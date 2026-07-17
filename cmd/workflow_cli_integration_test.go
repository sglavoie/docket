package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	tttemplate "github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/workflow"
)

func setupFileBackedWorkflowCLI(
	t *testing.T,
	root string,
	current *models.Project,
	projects []*models.Project,
	rules []workflow.WorkflowRule,
	templates []tttemplate.TaskTemplate,
) string {
	t.Helper()
	depth := 10
	domains := config.NewDomains()
	for _, project := range projects {
		saveProjectForGroupedCLITest(t, project)
		domains.Catalog.UpsertProject(project)
	}
	domains.Machine.CurrentProject = current.ID
	domains.Settings.UndoHistoryDepth = &depth
	domains.Automation.WorkflowRules = append([]workflow.WorkflowRule(nil), rules...)
	domains.Automation.TaskTemplates = append([]tttemplate.TaskTemplate(nil), templates...)
	configureFileBackedGroupedCLI(t, config.NewRuntimeState(root, domains))
	return filepath.Join(root, "config.json")
}

func reloadWorkflowRuntime(t *testing.T, configPath string) *config.RuntimeState {
	t.Helper()
	runtime, err := config.NewManager(configPath).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	return runtime
}

func reloadWorkflowProject(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload project %s: %v", project.Name, err)
	}
}

func enabledTaskRule(id string, trigger workflow.RuleTrigger, actions ...workflow.RuleAction) workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      id,
		Name:    id,
		Enabled: true,
		Trigger: trigger,
		Actions: actions,
	}
}

func TestCLIWorkflowStatelessAddCreatesFollowup(t *testing.T) {
	root := t.TempDir()
	project := newFileBackedProject(t, root, "project-add", "Add Project")
	rule := enabledTaskRule(
		"followup-on-add",
		workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
		workflow.RuleAction{ActionType: workflow.ActionCreateFollowup, Value: "Follow up from CLI"},
	)
	setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)
	resetQuickAddVars()
	t.Cleanup(resetQuickAddVars)
	_ = addCmd.Flags().Set("project", "")

	if err := runQuickAdd(addCmd, []string{"Created from CLI"}); err != nil {
		t.Fatalf("runQuickAdd: %v", err)
	}
	reloadWorkflowProject(t, project)
	if len(project.Tasks) != 2 {
		t.Fatalf("tasks = %d, want created task plus follow-up", len(project.Tasks))
	}
	if project.Tasks[1].Title != "Follow up from CLI" {
		t.Fatalf("follow-up title = %q", project.Tasks[1].Title)
	}
}

func TestCLIWorkflowContextualStateAndEditRules(t *testing.T) {
	t.Run("state transition", func(t *testing.T) {
		root := t.TempDir()
		project := newFileBackedProject(t, root, "project-state", "State Project")
		task := addTestTask(t, project, "state-task", "Complete me", models.StatusTodo)
		rule := enabledTaskRule(
			"tag-on-complete",
			workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
			workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "completed-by-rule"},
		)
		setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)

		if err := runTaskComplete(newStateTestCmd(), []string{fmt.Sprintf("%d", task.ShortID)}); err != nil {
			t.Fatalf("runTaskComplete: %v", err)
		}
		reloadWorkflowProject(t, project)
		if got := project.GetTaskByID(task.ID); got == nil || !got.HasTag("completed-by-rule") {
			t.Fatalf("completed task = %+v, want workflow tag", got)
		}
	})

	t.Run("edited content", func(t *testing.T) {
		root := t.TempDir()
		project := newFileBackedProject(t, root, "project-edit", "Edit Project")
		task := addTestTask(t, project, "edit-task", "Edit me", models.StatusTodo)
		rule := enabledTaskRule(
			"must-not-fire-on-content-only-edit",
			workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
			workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "unexpected"},
		)
		setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)

		editor := filepath.Join(root, "edit-task.sh")
		script := "#!/bin/sh\nprintf '%s\\n' '# Edited title' '' '<!-- Edit the title above and description below. -->' '' 'Edited description' > \"$1\"\n"
		if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
			t.Fatalf("write editor: %v", err)
		}
		oldEditor := editEditor
		editEditor = editor
		t.Cleanup(func() { editEditor = oldEditor })

		if err := runTaskEdit(newStateTestCmd(), []string{fmt.Sprintf("%d", task.ShortID)}); err != nil {
			t.Fatalf("runTaskEdit: %v", err)
		}
		reloadWorkflowProject(t, project)
		got := project.GetTaskByID(task.ID)
		if got == nil || got.Title != "Edited title" || got.HasTag("unexpected") {
			t.Fatalf("edited task = %+v, want contextual content edit without a false state event", got)
		}
	})
}

func TestCLIWorkflowCrossProjectMove(t *testing.T) {
	root := t.TempDir()
	source := newFileBackedProject(t, root, "project-source", "Source")
	dest := newFileBackedProject(t, root, "project-dest", "Destination")
	rule := enabledTaskRule(
		"move-new-tasks",
		workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
		workflow.RuleAction{ActionType: workflow.ActionMoveToProject, Value: dest.Name},
	)
	setupFileBackedWorkflowCLI(t, root, source, []*models.Project{source, dest}, []workflow.WorkflowRule{rule}, nil)
	resetQuickAddVars()
	t.Cleanup(resetQuickAddVars)
	_ = addCmd.Flags().Set("project", "")

	if err := runQuickAdd(addCmd, []string{"Move on creation"}); err != nil {
		t.Fatalf("runQuickAdd: %v", err)
	}
	reloadWorkflowProject(t, source)
	reloadWorkflowProject(t, dest)
	if len(source.Tasks) != 0 || len(dest.Tasks) != 1 || dest.Tasks[0].Title != "Move on creation" {
		t.Fatalf("source tasks = %d, destination tasks = %+v", len(source.Tasks), dest.Tasks)
	}
}

func TestCLIWorkflowProjectStatusRule(t *testing.T) {
	root := t.TempDir()
	project := newFileBackedProject(t, root, "project-status", "Status Project")
	rule := workflow.WorkflowRule{
		ID:      "status-active-project",
		Name:    "status-active-project",
		Level:   workflow.RuleLevelProject,
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldActiveTaskCount, Operation: workflow.OpGreaterThan, Value: "0"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetProjectStatus, Value: "active"}},
	}
	configPath := setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)
	resetQuickAddVars()
	t.Cleanup(resetQuickAddVars)
	_ = addCmd.Flags().Set("project", "")

	if err := runQuickAdd(addCmd, []string{"Activate project"}); err != nil {
		t.Fatalf("runQuickAdd: %v", err)
	}
	got := reloadWorkflowRuntime(t, configPath).GetProject(project.ID)
	if got == nil || got.StatusIcon != "active" || got.StatusIconSource != "auto" {
		t.Fatalf("catalog project = %+v, want auto active status", got)
	}
}

func TestCLITemplateWorkflowPersistsUseAndCounts(t *testing.T) {
	root := t.TempDir()
	project := newFileBackedProject(t, root, "project-template", "Template Project")
	rule := enabledTaskRule(
		"template-workflow",
		workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
		workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "from-workflow"},
	)
	tmpl := tttemplate.TaskTemplate{ID: "workflow-template", Name: "Workflow Template", Enabled: true, Type: models.TaskTypeTask}
	configPath := setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, []tttemplate.TaskTemplate{tmpl})
	resetTemplateUseVars()
	t.Cleanup(resetTemplateUseVars)

	if err := runTemplateUse(templateUseCmd, []string{tmpl.ID, "Template task"}); err != nil {
		t.Fatalf("runTemplateUse: %v", err)
	}
	reloadWorkflowProject(t, project)
	runtime := reloadWorkflowRuntime(t, configPath)
	storedTemplate := runtime.Automation().GetTaskTemplate(tmpl.ID)
	storedRule := runtime.Automation().GetWorkflowRule(rule.ID)
	if len(project.Tasks) != 1 || !project.Tasks[0].HasTag("from-workflow") {
		t.Fatalf("template-created task = %+v", project.Tasks)
	}
	if storedTemplate == nil || storedTemplate.UseCount != 1 {
		t.Fatalf("stored template = %+v, want use count 1", storedTemplate)
	}
	if storedRule == nil || storedRule.TriggerCount != 1 {
		t.Fatalf("stored rule = %+v, want trigger count 1", storedRule)
	}
}

func TestCLIPartialSuccessFlushesTriggerCountsOnce(t *testing.T) {
	root := t.TempDir()
	project := newFileBackedProject(t, root, "project-partial", "Partial Project")
	first := addTestTask(t, project, "partial-first", "First", models.StatusTodo)
	second := addTestTask(t, project, "partial-second", "Second", models.StatusTodo)
	rule := enabledTaskRule(
		"priority-one",
		workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "1"},
		workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "priority-one"},
	)
	configPath := setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)

	args := []string{fmt.Sprintf("%d", first.ShortID), "999", fmt.Sprintf("%d", second.ShortID), "1"}
	if err := runTaskPriority(newStateTestCmd(), args); err != nil {
		t.Fatalf("runTaskPriority partial success: %v", err)
	}
	runtime := reloadWorkflowRuntime(t, configPath)
	storedRule := runtime.Automation().GetWorkflowRule(rule.ID)
	if storedRule == nil || storedRule.TriggerCount != 2 {
		t.Fatalf("stored rule = %+v, want combined trigger count 2", storedRule)
	}

	backupData, err := os.ReadFile(config.AutomationPath(root) + ".bak")
	if err != nil {
		t.Fatalf("read automation backup: %v", err)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(backupData, &envelope); err != nil {
		t.Fatalf("decode automation backup: %v", err)
	}
	var backup config.AutomationConfig
	if err := json.Unmarshal(envelope.Data, &backup); err != nil {
		t.Fatalf("decode automation backup payload: %v", err)
	}
	backupRule := backup.GetWorkflowRule(rule.ID)
	if backupRule == nil || backupRule.TriggerCount != 0 {
		t.Fatalf("automation backup rule = %+v; counter persistence was not one final flush", backupRule)
	}
}

func TestCLIWorkflowDisabledRulesAndNoOpCommands(t *testing.T) {
	t.Run("disabled rule", func(t *testing.T) {
		root := t.TempDir()
		project := newFileBackedProject(t, root, "project-disabled", "Disabled Project")
		rule := enabledTaskRule(
			"disabled-followup",
			workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "todo"},
			workflow.RuleAction{ActionType: workflow.ActionCreateFollowup, Value: "must not exist"},
		)
		rule.Enabled = false
		setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)
		resetQuickAddVars()
		t.Cleanup(resetQuickAddVars)
		_ = addCmd.Flags().Set("project", "")

		if err := runQuickAdd(addCmd, []string{"Only task"}); err != nil {
			t.Fatalf("runQuickAdd: %v", err)
		}
		reloadWorkflowProject(t, project)
		if len(project.Tasks) != 1 {
			t.Fatalf("disabled rule created side effects: %+v", project.Tasks)
		}
	})

	t.Run("priority no-op", func(t *testing.T) {
		root := t.TempDir()
		project := newFileBackedProject(t, root, "project-noop", "No-op Project")
		task := addTestTask(t, project, "noop-task", "No-op", models.StatusTodo)
		rule := enabledTaskRule(
			"match-default-priority",
			workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: fmt.Sprintf("%d", task.Priority)},
			workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "must-not-fire"},
		)
		configPath := setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, []workflow.WorkflowRule{rule}, nil)

		if err := runTaskPriority(newStateTestCmd(), []string{fmt.Sprintf("%d", task.ShortID), fmt.Sprintf("%d", task.Priority)}); err != nil {
			t.Fatalf("runTaskPriority no-op: %v", err)
		}
		reloadWorkflowProject(t, project)
		storedRule := reloadWorkflowRuntime(t, configPath).Automation().GetWorkflowRule(rule.ID)
		if project.GetTaskByID(task.ID).HasTag("must-not-fire") || storedRule == nil || storedRule.TriggerCount != 0 {
			t.Fatalf("no-op fired workflow: task=%+v rule=%+v", project.GetTaskByID(task.ID), storedRule)
		}
	})
}

func TestCLIWorkflowConflictWarningsAreNormalized(t *testing.T) {
	_, stderr, err := captureTUITestOutput(t, func() error {
		emitCLIWorkflowConflicts([]app.WorkflowConflict{
			{TaskID: "task-b", TaskTitle: "Beta", Field: "priority", Rules: []string{"z-rule", "a-rule"}},
			{TaskID: "task-a", TaskTitle: "Alpha", Field: "section", Rules: []string{"c-rule", "b-rule"}},
		})
		return nil
	})
	if err != nil {
		t.Fatalf("emit conflict warnings: %v", err)
	}
	alpha := "Alpha (task-a): section <- b-rule, c-rule"
	beta := "Beta (task-b): priority <- a-rule, z-rule"
	if !strings.Contains(stderr, "warning: workflow rule conflicts detected:") ||
		!strings.Contains(stderr, alpha) || !strings.Contains(stderr, beta) ||
		strings.Index(stderr, alpha) > strings.Index(stderr, beta) {
		t.Fatalf("stderr is not normalized:\n%s", stderr)
	}
}
