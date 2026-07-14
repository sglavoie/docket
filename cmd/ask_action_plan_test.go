package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestPrintActionPlanPreviewEscapesTerminalControls(t *testing.T) {
	var out bytes.Buffer
	printActionPlanPreview(&out, app.PlanPreview{
		Summary: "clear\x1b[2J\u009bspoof",
		Actions: []app.ActionPreview{{
			Index:       1,
			Risk:        app.ActionRiskSafe,
			Description: "title\x1b]0;spoof\x07\u009dhidden\u009c",
		}},
	})

	got := out.String()
	for _, r := range got {
		if r == '\x1b' || r == '\x07' || r == '\u009b' || r == '\u009c' || r == '\u009d' {
			t.Fatalf("preview contains raw terminal control U+%04X: %q", r, got)
		}
	}
	for _, want := range []string{`\x1b[2J`, `\u009bspoof`, `\x1b]0;spoof\a`, `\u009dhidden\u009c`} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview = %q, want visible escape %q", got, want)
		}
	}
}

func TestApplyCLIActionPlanCleansDurableAndMachineDeleteRefs(t *testing.T) {
	deps, project := setupStateTest(t)
	project.BuildTaskTree()

	var root *models.Task
	for _, task := range project.Tasks {
		if len(task.Children) > 0 {
			root = task
			break
		}
	}
	if root == nil {
		t.Fatal("test project has no task subtree")
	}
	deleted := collectTaskDescendantIDs(project.Tasks, map[string]bool{root.ID: true})
	for id := range deleted {
		deps.Config.Planning().FocusQueue = append(deps.Config.Planning().FocusQueue, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
		deps.Config.Planning().DailyPlan = append(deps.Config.Planning().DailyPlan, config.DailyPlanItem{TaskID: id, ProjectID: project.ID})
		deps.Config.Planning().FocusDismissedIDs = append(deps.Config.Planning().FocusDismissedIDs, id)
		deps.Config.Machine().RecentTaskVisits = append(deps.Config.Machine().RecentTaskVisits, config.RecentTaskVisit{TaskID: id, ProjectID: project.ID})
	}
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	workspace, err := LoadCLIWorkspace()
	testutil.AssertNoError(t, err)
	result, warning, applyErr := applyCLIActionPlan(
		context.Background(),
		workspace.Runtime,
		newWorkspaceAppService(workspace),
		app.ActionPlan{
			Version: app.ActionPlanVersion,
			Summary: "Delete subtree",
			Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
		},
		app.ApplyActionPlanOptions{ConfirmSafe: true, ConfirmDestructive: true},
	)
	testutil.AssertNoError(t, applyErr)
	if warning != nil {
		t.Fatalf("post-commit reference warning: %v", warning)
	}
	if len(result.DeletedTaskIDs) != len(deleted) {
		t.Fatalf("deleted IDs = %v, want %v", result.DeletedTaskIDs, deleted)
	}
	for _, id := range result.DeletedTaskIDs {
		if !deleted[id] {
			t.Fatalf("unexpected deleted task ID %q", id)
		}
	}

	reloaded, loadErr := manager.LoadRuntimeState()
	testutil.AssertNoError(t, loadErr)
	if got := reloaded.Planning(); len(got.FocusQueue) != 0 || len(got.DailyPlan) != 0 || len(got.FocusDismissedIDs) != 0 {
		t.Fatalf("durable planning refs survived AI delete: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}
	if got := reloaded.Machine().RecentTaskVisits; len(got) != 0 {
		t.Fatalf("machine visits survived AI delete: %v", got)
	}
	stored := deps.Repository.Projects[project.ID]
	for id := range deleted {
		if stored.GetTaskByID(id) != nil {
			t.Fatalf("deleted subtree task %q survived in project", id)
		}
	}
}
