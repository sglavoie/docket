package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestAskStreamDoneCallsCancelFn ensures that when streaming completes, the
// context cancel function is invoked rather than just dropped. Dropping it
// without calling cancel leaks the context's child goroutine until the parent
// is canceled — and the parent here is context.Background(), which never is.
func TestAskStreamDoneCallsCancelFn(t *testing.T) {
	m := NewAskViewModel(nil, "")

	called := false
	m.cancelFn = func() { called = true }

	if _, _ = m.Update(AskStreamDoneMsg{}); !called {
		t.Fatal("expected cancelFn to be called when AskStreamDoneMsg is handled (success path)")
	}

	// Same expectation on the error path.
	called = false
	m.cancelFn = func() { called = true }
	if _, _ = m.Update(AskStreamDoneMsg{Err: errors.New("boom")}); !called {
		t.Fatal("expected cancelFn to be called when AskStreamDoneMsg is handled (error path)")
	}

	if m.cancelFn != nil {
		t.Fatal("expected m.cancelFn to be cleared after handling AskStreamDoneMsg")
	}
}

func TestAskViewRequiresFirstUseConsentAndOffersContextControls(t *testing.T) {
	project := &models.Project{ID: "p1", Slug: "demo", Name: "Demo"}
	runtime := config.EmptyRuntimeState("")
	m := NewAskViewModelWithDeps(project, "", runtime, nil)
	m.textarea.SetValue("what should I do?")

	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}); cmd != nil {
		t.Fatal("first contextual submit should pause for consent without starting provider command")
	}
	if m.state != askStateConsent {
		t.Fatalf("state = %v, want consent", m.state)
	}

	m.state = askStateInput
	m.contextMode = external.TaskContextFull
	_, _ = m.handleKey(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.contextMode != external.TaskContextRedacted {
		t.Fatalf("first context cycle = %s, want redacted", m.contextMode)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.contextMode != external.TaskContextNone {
		t.Fatalf("second context cycle = %s, want none", m.contextMode)
	}
}

func TestAskActionPlanSuccessClearsPlanAndPreventsSecondCreate(t *testing.T) {
	project, err := models.NewProject("ask-project", "Ask Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	repo := newTestAppRepo(project)
	service := app.NewService(repo, nil, nil)
	title := "Created exactly once"
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "Create one task",
		Actions: []app.Action{{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &title}},
	}
	preview, err := service.PreviewActionPlan(plan)
	if err != nil {
		t.Fatalf("PreviewActionPlan: %v", err)
	}
	m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
	m.SetSize(90, 24)
	m.state = askStateResult
	m.plan = plan
	m.preview = preview
	m.response = renderActionPlanMarkdown(preview)

	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateConfirmSafe {
		t.Fatalf("review/apply key = state %v cmd %v, want safe confirmation", m.state, cmd)
	}
	_, applyCmd := m.handleKey(tea.KeyPressMsg{Code: 'y'})
	if applyCmd == nil {
		t.Fatal("safe confirmation did not produce an apply command")
	}
	msg := applyCmd()
	if applied, ok := msg.(AskActionPlanAppliedMsg); !ok || applied.Err != nil {
		t.Fatalf("apply message = %#v", msg)
	}
	_, _ = m.Update(msg)

	if len(m.plan.Actions) != 0 || len(m.preview.Actions) != 0 {
		t.Fatalf("successful apply retained executable state: plan=%#v preview=%#v", m.plan, m.preview)
	}
	if strings.Contains(m.View().Content, "review/apply plan") {
		t.Fatal("result footer still advertises the apply key after success")
	}
	if _, secondCmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); secondCmd != nil {
		t.Fatal("second apply key produced a command after successful apply")
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after second apply attempt: %v", err)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].Title != title {
		t.Fatalf("tasks after second apply attempt = %#v, want one created task", project.Tasks)
	}
}

func TestAskActionPlanFailurePreservesPlanForRetry(t *testing.T) {
	title := "Retry me"
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionCreateTask, ProjectID: "p1", Title: &title}},
	}
	preview := app.PlanPreview{SafeCount: 1, Actions: []app.ActionPreview{{Index: 1, Type: app.ActionCreateTask, Risk: app.ActionRiskSafe}}}
	m := NewAskViewModel(nil, "")
	m.state = askStateApplying
	m.plan = plan
	m.preview = preview

	_, _ = m.Update(AskActionPlanAppliedMsg{Err: errors.New("commit failed")})
	if len(m.plan.Actions) != 1 || len(m.preview.Actions) != 1 {
		t.Fatalf("failed apply discarded retry state: plan=%#v preview=%#v", m.plan, m.preview)
	}
	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateConfirmSafe {
		t.Fatalf("retry apply key = state %v cmd %v, want safe confirmation", m.state, cmd)
	}
}

func TestAskActionPlanDeleteCleansPlanningAndMachineSubtreeRefs(t *testing.T) {
	project, err := models.NewProject("ask-delete-project", "Ask Delete", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	root, err := models.NewTask("ask-delete-root", 1, models.TaskTypeEpic, "Delete root")
	if err != nil {
		t.Fatalf("NewTask root: %v", err)
	}
	child, err := models.NewTask("ask-delete-child", 2, models.TaskTypeTask, "Delete child")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	rootID := root.ID
	child.ParentID = &rootID
	project.Tasks = []*models.Task{root, child}
	project.NextShortID = 3
	project.BuildTaskTree()

	domains := config.NewDomains()
	for _, id := range []string{root.ID, child.ID} {
		domains.Planning.FocusQueue = append(domains.Planning.FocusQueue, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
		domains.Planning.DailyPlan = append(domains.Planning.DailyPlan, config.DailyPlanItem{TaskID: id, ProjectID: project.ID})
		domains.Planning.FocusDismissedIDs = append(domains.Planning.FocusDismissedIDs, id)
		domains.Machine.RecentTaskVisits = append(domains.Machine.RecentTaskVisits, config.RecentTaskVisit{TaskID: id, ProjectID: project.ID})
	}
	runtime := config.NewRuntimeState("", domains)
	service := app.NewService(newTestAppRepo(project), nil, nil)
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
	}
	m := NewAskViewModelWithDeps(project, "", runtime, service)
	m.plan = plan
	m.preview = app.PlanPreview{DestructiveCount: 1, Actions: []app.ActionPreview{{Index: 1, Type: app.ActionDeleteTask, Risk: app.ActionRiskDestructive}}}
	applyCmd := m.applyPlan(plan, true)
	if applyCmd == nil {
		t.Fatal("delete plan did not produce an apply command")
	}
	msg := applyCmd()
	if applied, ok := msg.(AskActionPlanAppliedMsg); !ok || applied.Err != nil {
		t.Fatalf("delete apply message = %#v", msg)
	}
	_, _ = m.Update(msg)

	if got := runtime.Planning(); len(got.FocusQueue) != 0 || len(got.DailyPlan) != 0 || len(got.FocusDismissedIDs) != 0 {
		t.Fatalf("planning refs survived TUI AI delete: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}
	if got := runtime.Machine().RecentTaskVisits; len(got) != 0 {
		t.Fatalf("machine visits survived TUI AI delete: %v", got)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after TUI AI delete: %v", err)
	}
	if len(project.Tasks) != 0 {
		t.Fatalf("deleted subtree survived TUI AI delete: %#v", project.Tasks)
	}
}
