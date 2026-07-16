package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

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

func TestAskActionPlanMarkdownDisplaySafety(t *testing.T) {
	got := renderActionPlanMarkdown(app.PlanPreview{
		Summary: "summary\x1b[2J\r\u202e\nspoof",
		Actions: []app.ActionPreview{{
			Index:       1,
			Risk:        app.ActionRiskSafe,
			Description: "description\x1b]0;x\x07\u009b\u2066\n\t- **Markdown**",
		}},
	})

	for _, r := range got {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			t.Fatalf("rendered Markdown contains unsafe U+%04X: %q", r, got)
		}
	}
	for _, want := range []string{`\x1b[2J`, `\r`, `\u202e`, `\nspoof`, `\x1b]0;x\a`, `\u009b`, `\u2066`} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered Markdown = %q, want visible escape %q", got, want)
		}
	}
	if !strings.Contains(got, "\\u2066\n\t- **Markdown**") {
		t.Fatalf("rendered Markdown = %q, want multiline description preserved", got)
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
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	preview := prepared.Preview()
	m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
	m.SetSize(90, 24)
	m.state = askStateResult
	m.prepared = prepared
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

	if m.prepared != nil {
		t.Fatal("successful apply retained executable prepared state")
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
	project, err := models.NewProject("ask-retry-project", "Ask Retry", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	service := app.NewService(newTestAppRepo(project), nil, nil)
	title := "Retry me"
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &title}},
	}
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
	m.state = askStateApplying
	m.prepared = prepared

	_, _ = m.Update(AskActionPlanAppliedMsg{Err: errors.New("commit failed")})
	if m.prepared != prepared {
		t.Fatal("retryable apply failure discarded prepared state")
	}
	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateConfirmSafe {
		t.Fatalf("retry apply key = state %v cmd %v, want safe confirmation", m.state, cmd)
	}
}

func TestAskActionPlanRiskSelectionsPreserveOriginalIndexes(t *testing.T) {
	for _, tc := range []struct {
		name               string
		acceptedSafe       bool
		includeDestructive bool
		wantIndexes        []int
	}{
		{name: "safe-only", acceptedSafe: true, wantIndexes: []int{2}},
		{name: "destructive-only", includeDestructive: true, wantIndexes: []int{1}},
		{name: "mixed", acceptedSafe: true, includeDestructive: true, wantIndexes: []int{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, service, prepared := newAskMixedPreparedPlan(t, "ask-selection-"+tc.name)
			m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
			m.prepared = prepared
			m.acceptedSafe = tc.acceptedSafe

			cmd := m.applyAcceptedPlan(tc.includeDestructive)
			if cmd == nil {
				t.Fatal("accepted risk selection did not produce an apply command")
			}
			msg, ok := cmd().(AskActionPlanAppliedMsg)
			if !ok || msg.Err != nil {
				t.Fatalf("apply message = %#v", msg)
			}
			if len(msg.Result.Actions) != len(tc.wantIndexes) {
				t.Fatalf("applied actions = %#v, want indexes %v", msg.Result.Actions, tc.wantIndexes)
			}
			for i, want := range tc.wantIndexes {
				if msg.Result.Actions[i].Index != want {
					t.Fatalf("applied action %d index = %d, want original index %d", i, msg.Result.Actions[i].Index, want)
				}
			}
		})
	}
}

func TestAskActionPlanDriftWhileConfirmationOpenClearsPreparedState(t *testing.T) {
	project, err := models.NewProject("ask-confirmation-drift", "Ask Confirmation Drift", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	baseRepo := newTestAppRepo(project)
	repo := &askActionPlanFailRepo{base: baseRepo}
	service := app.NewService(repo, nil, nil)
	title := "Created only without drift"
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &title}},
	})
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
	m.state = askStateResult
	m.prepared = prepared
	m.response = renderActionPlanMarkdown(prepared.Preview())

	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateConfirmSafe {
		t.Fatalf("review key = state %v cmd %v, want safe confirmation", m.state, cmd)
	}
	repo.commitErr = &storage.RevisionConflictError{
		ProjectID: project.ID,
		Expected:  "reviewed-revision",
		Actual:    "concurrent-revision",
	}
	_, applyCmd := m.handleKey(tea.KeyPressMsg{Code: 'y'})
	if applyCmd == nil {
		t.Fatal("safe confirmation did not produce an apply command")
	}
	msg := applyCmd()
	if applied, ok := msg.(AskActionPlanAppliedMsg); !ok || !errors.Is(applied.Err, storage.ErrStaleRevision) {
		t.Fatalf("drift apply message = %#v, want stale revision", msg)
	}
	_, _ = m.Update(msg)

	if m.prepared != nil {
		t.Fatal("confirmation-time drift retained executable prepared state")
	}
	if !strings.Contains(m.response, "workspace changed") || !strings.Contains(m.response, "fresh AI response") || !strings.Contains(m.response, "new preview") {
		t.Fatalf("stale response = %q, want fresh-response and preview guidance", m.response)
	}
	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateResult {
		t.Fatalf("stale plan remained applicable: state=%v cmd=%v", m.state, cmd)
	}
}

func TestAskActionPlanStaleProjectAndArtifactFailuresClearState(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "project", err: &storage.RevisionConflictError{ProjectID: "p1", Expected: "old", Actual: "new"}},
		{name: "artifact", err: &storage.ArtifactConflictError{Path: "planning.json", Expected: "old", Actual: "new"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, service, prepared := newAskMixedPreparedPlan(t, "ask-stale-"+tc.name)
			m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
			m.state = askStateApplying
			m.prepared = prepared
			m.acceptedSafe = true
			m.response = renderActionPlanMarkdown(prepared.Preview())

			_, _ = m.Update(AskActionPlanAppliedMsg{Err: tc.err})
			if m.prepared != nil || m.acceptedSafe {
				t.Fatalf("stale %s failure retained executable state", tc.name)
			}
			if !strings.Contains(m.response, "workspace changed") || !strings.Contains(m.response, "fresh AI response") || !strings.Contains(m.response, "new preview") {
				t.Fatalf("stale %s response = %q", tc.name, m.response)
			}
		})
	}
}

func TestAskActionPlanNonRetryableFailureClearsPreparedState(t *testing.T) {
	project, service, prepared := newAskMixedPreparedPlan(t, "ask-consumed")
	m := NewAskViewModelWithDeps(project, "", config.EmptyRuntimeState(""), service)
	m.state = askStateApplying
	m.prepared = prepared

	_, _ = m.Update(AskActionPlanAppliedMsg{Err: app.ErrPreparedActionPlanConsumed})
	if m.prepared != nil {
		t.Fatal("non-retryable prepared-plan failure retained executable state")
	}
	if !strings.Contains(m.response, "fresh AI response") {
		t.Fatalf("non-retryable response = %q, want fresh-response guidance", m.response)
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
	survivor, err := models.NewTask("ask-delete-survivor", 3, models.TaskTypeTask, "Keep survivor")
	if err != nil {
		t.Fatalf("NewTask survivor: %v", err)
	}
	project.Tasks = []*models.Task{root, child, survivor}
	project.NextShortID = 4
	project.BuildTaskTree()

	domains := config.NewDomains()
	for _, id := range []string{root.ID, child.ID, survivor.ID} {
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
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	m := NewAskViewModelWithDeps(project, "", runtime, service)
	m.prepared = prepared
	applyCmd := m.applyPlan(prepared, true)
	if applyCmd == nil {
		t.Fatal("delete plan did not produce an apply command")
	}
	msg := applyCmd()
	if applied, ok := msg.(AskActionPlanAppliedMsg); !ok || applied.Err != nil {
		t.Fatalf("delete apply message = %#v", msg)
	}
	_, _ = m.Update(msg)

	if got := runtime.Planning(); len(got.FocusQueue) != 1 || got.FocusQueue[0].TaskID != survivor.ID ||
		len(got.DailyPlan) != 1 || got.DailyPlan[0].TaskID != survivor.ID ||
		len(got.FocusDismissedIDs) != 1 || got.FocusDismissedIDs[0] != survivor.ID {
		t.Fatalf("TUI AI delete cleaned the wrong planning refs: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}
	if got := runtime.Machine().RecentTaskVisits; len(got) != 1 || got[0].TaskID != survivor.ID {
		t.Fatalf("TUI AI delete cleaned the wrong machine visits: %v", got)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after TUI AI delete: %v", err)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != survivor.ID {
		t.Fatalf("TUI AI delete removed outside the reviewed subtree: %#v", project.Tasks)
	}
}

func newAskMixedPreparedPlan(t *testing.T, projectID string) (*models.Project, *app.Service, *app.PreparedActionPlan) {
	t.Helper()
	project, err := models.NewProject(projectID, "Ask Mixed", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	root, err := models.NewTask(projectID+"-root", 1, models.TaskTypeEpic, "Delete me")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{root}
	project.NextShortID = 2
	project.BuildTaskTree()
	service := app.NewService(newTestAppRepo(project), nil, nil)
	title := "Keep original provider index"
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{
			{Type: app.ActionDeleteTask, TaskID: root.ID},
			{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &title},
		},
	})
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	return project, service, prepared
}

type askActionPlanFailRepo struct {
	base      *testAppRepo
	commitErr error
}

func (r *askActionPlanFailRepo) Snapshot() (app.Snapshot, error) {
	return r.base.Snapshot()
}

func (r *askActionPlanFailRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.commitErr != nil {
		return nil, r.commitErr
	}
	return r.base.Commit(changesets...)
}
