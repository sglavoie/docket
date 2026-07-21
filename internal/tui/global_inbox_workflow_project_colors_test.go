package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

func assertProjectColorOnlyChangesStyle(t *testing.T, colored, fallback string) {
	t.Helper()

	if got, want := ansi.Strip(colored), ansi.Strip(fallback); got != want {
		t.Fatalf("project color changed rendered text:\ncolored=%q\nfallback=%q", got, want)
	}
	coloredLines := strings.Split(colored, "\n")
	fallbackLines := strings.Split(fallback, "\n")
	if len(coloredLines) != len(fallbackLines) {
		t.Fatalf("project color changed rendered height: colored=%d fallback=%d", len(coloredLines), len(fallbackLines))
	}
	for i := range coloredLines {
		if got, want := lipgloss.Width(coloredLines[i]), lipgloss.Width(fallbackLines[i]); got != want {
			t.Fatalf("project color changed line %d width: colored=%d fallback=%d", i, got, want)
		}
	}
}

func TestGlobalInboxProjectColorAcrossThemesAndLocalParity(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	task := &models.Task{
		ID:            "inbox-task",
		ShortID:       42,
		Title:         "Triage project-aware task",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateInbox,
		Priority:      3,
	}
	project := &models.Project{
		ID:        "inbox-project",
		Name:      "Inbox Project",
		Slug:      "inbox-project",
		ColorSlot: 17,
		Tasks:     []*models.Task{task},
	}
	runtime := newTestConfig()
	runtime.Settings().SectionColors = map[string]string{}

	local := NewInboxProcessorModel(project, nil, runtime)
	local.SetSize(100, 24)
	global := &GlobalInboxProcessorModel{
		inboxTasks:   []GlobalInboxTask{{Task: task, Project: project}},
		projectCache: map[string]*models.Project{project.ID: project},
	}
	global.initCore(global, global, nil, runtime)
	global.SetSize(100, 24)

	for _, theme := range []ThemeConfig{{Name: ""}, {Name: "light"}} {
		themeName := theme.Name
		if themeName == "" {
			themeName = "dark"
		}
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(theme)

			colored := global.renderTaskDisplay()
			want := projectColorStyle(project).Render(project.Name)
			if !strings.Contains(colored, want) {
				t.Fatalf("Global Inbox did not render the %s project color %q:\n%s", themeName, want, colored)
			}

			localWithSlot := local.renderTaskDisplay()
			project.ColorSlot = 0
			fallback := global.renderTaskDisplay()
			localWithoutSlot := local.renderTaskDisplay()
			project.ColorSlot = 17

			assertProjectColorOnlyChangesStyle(t, colored, fallback)
			if localWithSlot != localWithoutSlot {
				t.Fatal("local Inbox rendering changed with the owning project's ColorSlot")
			}
			if strings.Contains(ansi.Strip(localWithSlot), "Project:") {
				t.Fatalf("local Inbox unexpectedly rendered project context:\n%s", ansi.Strip(localWithSlot))
			}
		})
	}
}

func TestWorkflowDryRunLoadingRetainsOwningProject(t *testing.T) {
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	project := &models.Project{
		ID:        "workflow-project",
		Name:      "Workflow Project",
		Slug:      "workflow-project",
		ColorSlot: 23,
	}
	catalog := config.NewWorkspaceCatalog()
	catalog.UpsertProject(project)
	if err := manager.CommitCatalog(catalog); err != nil {
		t.Fatalf("CommitCatalog: %v", err)
	}

	task := &models.Task{
		ID:            "workflow-task",
		ShortID:       7,
		Title:         "Promote priority",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateNext,
		Priority:      3,
	}
	persister := NewMockPersister()
	persister.Projects[project.ID] = &models.Project{Tasks: []*models.Task{task}}
	rule := workflow.WorkflowRule{
		ID:      "promote-priority",
		Name:    "Promote priority",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "3"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetPriority, Value: "1"}},
	}

	model := NewWorkflowDryRunModel(dataDir, manager, persister, &rule)
	rawMsg := model.runDryRun()
	msg, ok := rawMsg.(dryRunDoneMsg)
	if !ok {
		t.Fatalf("runDryRun returned %T, want dryRunDoneMsg", rawMsg)
	}
	if msg.err != nil {
		t.Fatalf("runDryRun: %v", msg.err)
	}
	if len(msg.results) != 1 {
		t.Fatalf("runDryRun results = %d, want 1", len(msg.results))
	}
	if len(persister.LoadCalls) != 1 {
		t.Fatalf("persister Load calls = %d, want 1", len(persister.LoadCalls))
	}
	result := msg.results[0]
	if result.Project != persister.LoadCalls[0] {
		t.Fatal("dry-run result did not retain the exact project object loaded for evaluation")
	}
	if result.Project.ColorSlot != project.ColorSlot || result.Project.Name != project.Name {
		t.Fatalf("dry-run project identity = %+v, want name %q slot %d", result.Project, project.Name, project.ColorSlot)
	}
	if result.Task != task {
		t.Fatal("dry-run result did not retain the original unmodified task")
	}
}

func TestWorkflowDryRunProjectColorWideNarrowDetailFilterAndFallback(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })
	ApplyTheme(ThemeConfig{Name: "light"})

	project := &models.Project{ID: "dry-run", Name: "Dry Run", Slug: "dry-run", ColorSlot: 31}
	task := &models.Task{
		ID:            "dry-run-task",
		ShortID:       11,
		Title:         "Render workflow preview",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      2,
	}
	result := DryRunTaskResult{
		Task:         task,
		Project:      project,
		MatchedRules: []string{"Preview rule"},
		Changes:      []DryRunChange{{Field: "priority", OldValue: "2", NewValue: "1"}},
	}
	model := &WorkflowDryRunModel{results: []DryRunTaskResult{result}, filtered: []DryRunTaskResult{result}}
	model.SetSize(100, 24)

	wide := model.renderTaskList(6)
	if want := projectColorStyle(project).Render("[" + project.Name + "]"); !strings.Contains(wide, want) {
		t.Fatalf("wide workflow dry-run list did not color project badge %q:\n%s", want, wide)
	}
	detail := model.renderDetailPane()
	if want := projectColorStyle(project).Render(project.Name); !strings.Contains(detail, want) {
		t.Fatalf("workflow dry-run detail did not color project value %q:\n%s", want, detail)
	}

	model.SetSize(60, 24)
	narrow := model.renderTaskListWithDetails(6)
	if want := projectColorStyle(project).Render("[" + project.Name + "]"); !strings.Contains(narrow, want) {
		t.Fatalf("narrow workflow dry-run list did not color selected project badge %q:\n%s", want, narrow)
	}

	model.searchQuery = strings.ToLower(project.Name)
	model.filterResults()
	if len(model.filtered) != 1 || model.filtered[0].Project != project {
		t.Fatalf("project-name filter lost the owning project: %+v", model.filtered)
	}

	fallbackProject := *project
	fallbackProject.ColorSlot = 0
	fallbackResult := result
	fallbackResult.Project = &fallbackProject
	fallbackModel := &WorkflowDryRunModel{filtered: []DryRunTaskResult{fallbackResult}}
	fallbackModel.SetSize(100, 24)
	assertProjectColorOnlyChangesStyle(t, wide, fallbackModel.renderTaskList(6))
	assertProjectColorOnlyChangesStyle(t, detail, fallbackModel.renderDetailPane())
	fallbackModel.SetSize(60, 24)
	assertProjectColorOnlyChangesStyle(t, narrow, fallbackModel.renderTaskListWithDetails(6))

	// Partial or synthetic results may not have an owning project. All render
	// variants and project-name filtering must stay nil-safe and muted.
	nilResult := result
	nilResult.Project = nil
	nilModel := &WorkflowDryRunModel{results: []DryRunTaskResult{nilResult}, filtered: []DryRunTaskResult{nilResult}}
	nilModel.SetSize(100, 24)
	if got := nilModel.renderTaskList(6); !strings.Contains(got, projectColorStyle(nil).Render("[]")) {
		t.Fatalf("nil-project wide fallback was not muted: %q", got)
	}
	_ = nilModel.renderDetailPane()
	nilModel.SetSize(60, 24)
	_ = nilModel.renderTaskListWithDetails(6)
	nilModel.searchQuery = "missing"
	nilModel.filterResults()
}

func TestProjectColorIdentityMatchesAcrossFinalMixedProjectSurfaces(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })
	ApplyTheme(ThemeConfig{Name: ""})

	project := &models.Project{ID: "identity", Name: "Identity", Slug: "identity", ColorSlot: 37}
	task := &models.Task{
		ID:            "identity-task",
		ShortID:       37,
		Title:         "Keep one durable hue",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	project.Tasks = []*models.Task{task}
	runtime := testConfigWithProjects(project)

	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = []*models.Project{project}
	outlinerLine := outliner.renderOutlinerProjectHeaderRow(
		outlinerRow{IsProjectHeader: true, ProjectID: project.ID}, false, 80, 0,
	)
	if want := projectColorStyle(project).Bold(true).Render("[" + project.Slug + "]"); !strings.Contains(outlinerLine, want) {
		t.Fatalf("outliner did not use durable project hue %v: %q", projectColor(project), outlinerLine)
	}

	kanban := newRenderKanban(80, 24, []KanbanColumn{{
		ID: "next", Label: "next", Icon: "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}})
	kanban.crossProject = true
	kanbanRendered := kanbanRender(kanban)
	kanbanLayout := computeKanbanColumnLayout(kanban.width, len(kanban.columns), kanban.colScrollOff)
	kanbanProject := projectColorStyle(project).Background(ColorDimmedBg).Bold(true).Width(kanbanLayout.colWidth).Render(project.Name)
	if !strings.Contains(kanbanRendered, kanbanProject) {
		t.Fatalf("kanban did not use durable project hue %v:\n%s", projectColor(project), kanbanRendered)
	}

	shared := RenderCrossProjectTaskLine(task, project, runtime, false, 100, 40, CrossProjectTaskLineOpts{})
	if want := projectColorStyle(project).Render("[" + project.Name + "]"); !strings.Contains(shared, want) {
		t.Fatalf("shared task row did not use durable project hue %v: %q", projectColor(project), shared)
	}

	inbox := renderInboxTaskDisplay(task, 100, false, project, runtime.Settings())
	if want := projectColorStyle(project).Render(project.Name); !strings.Contains(inbox, want) {
		t.Fatalf("Global Inbox did not use durable project hue %v: %q", projectColor(project), inbox)
	}

	dryRunResult := DryRunTaskResult{
		Task: project.Tasks[0], Project: project,
		Changes: []DryRunChange{{Field: "priority", OldValue: "3", NewValue: "1"}},
	}
	dryRun := &WorkflowDryRunModel{filtered: []DryRunTaskResult{dryRunResult}}
	dryRun.SetSize(100, 24)
	dryRunList := dryRun.renderTaskList(5)
	if want := projectColorStyle(project).Render(fmt.Sprintf("[%s]", project.Name)); !strings.Contains(dryRunList, want) {
		t.Fatalf("workflow dry run did not use durable project hue %v: %q", projectColor(project), dryRunList)
	}
}
