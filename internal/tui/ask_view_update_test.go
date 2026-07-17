package tui

import (
	"errors"
	"os"
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

func askActionPlanAppliedMessage(t *testing.T, cmd tea.Cmd) AskActionPlanAppliedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil apply command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("apply command = %#v, want completion plus spinner batch", batch)
	}
	raw := batch[0]()
	msg, ok := raw.(AskActionPlanAppliedMsg)
	if !ok {
		t.Fatalf("apply completion = %T, want AskActionPlanAppliedMsg", raw)
	}
	return msg
}

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

	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}); cmd == nil {
		t.Fatal("first contextual submit should persist harness preferences before consent")
	} else if next := driveAskPreferenceSave(t, m, cmd); next != nil {
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
	msg := askActionPlanAppliedMessage(t, applyCmd)
	if msg.Err != nil {
		t.Fatalf("apply message = %#v", msg)
	}
	appModel := newTestAppModel()
	setTestAppProjects(appModel, project)
	appModel.views.Register(ViewAsk, m)
	appModel.pushView(ViewAsk)
	_, _ = appModel.Update(msg)

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

	m.applyID = nextAskOperationID()
	_, _ = m.Update(AskActionPlanAppliedMsg{OperationID: m.applyID, Err: errors.New("commit failed")})
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
			msg := askActionPlanAppliedMessage(t, cmd)
			if msg.Err != nil {
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
	msg := askActionPlanAppliedMessage(t, applyCmd)
	if !errors.Is(msg.Err, storage.ErrStaleRevision) {
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
			m.applyID = nextAskOperationID()

			_, _ = m.Update(AskActionPlanAppliedMsg{OperationID: m.applyID, Err: tc.err})
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
	m.applyID = nextAskOperationID()

	_, _ = m.Update(AskActionPlanAppliedMsg{OperationID: m.applyID, Err: app.ErrPreparedActionPlanConsumed})
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
	msg := askActionPlanAppliedMessage(t, applyCmd)
	if msg.Err != nil {
		t.Fatalf("delete apply message = %#v", msg)
	}
	appModel := newTestAppModel()
	appModel.runtime = runtime
	setTestAppProjects(appModel, project)
	appModel.views.Register(ViewAsk, m)
	appModel.pushView(ViewAsk)
	_, _ = appModel.Update(msg)

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

func TestAskActionPlanCompletionRequiresMatchingApplyIdentityAndState(t *testing.T) {
	result := app.ActionPlanResult{
		TransactionID: "tx-stale",
		UndoID:        "undo-stale",
		Actions:       []app.ActionPreview{{Index: 1}},
	}

	t.Run("wrong ID", func(t *testing.T) {
		m := NewAskViewModel(nil, "")
		m.state = askStateApplying
		m.applyID = nextAskOperationID()
		m.response = "reviewed preview"

		_, cmd := m.Update(AskActionPlanAppliedMsg{OperationID: nextAskOperationID(), Result: result})
		if cmd != nil || m.state != askStateApplying || m.applyID == 0 || m.response != "reviewed preview" {
			t.Fatalf("wrong-ID completion mutated Ask: state=%v id=%d response=%q cmd=%v", m.state, m.applyID, m.response, cmd)
		}
	})

	t.Run("wrong state", func(t *testing.T) {
		m := NewAskViewModel(nil, "")
		m.state = askStateInput
		m.applyID = nextAskOperationID()
		m.response = "new instance input"

		_, cmd := m.Update(AskActionPlanAppliedMsg{OperationID: m.applyID, Result: result})
		if cmd != nil || m.state != askStateInput || m.applyID == 0 || m.response != "new instance input" {
			t.Fatalf("wrong-state completion mutated Ask: state=%v id=%d response=%q cmd=%v", m.state, m.applyID, m.response, cmd)
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		m := NewAskViewModel(nil, "")
		m.state = askStateApplying
		m.applyID = nextAskOperationID()
		m.response = "active"

		_, _ = m.Update(AskActionPlanAppliedMsg{Result: result})
		if m.state != askStateApplying || m.applyID == 0 || m.response != "active" {
			t.Fatalf("zero-ID completion mutated Ask: state=%v id=%d response=%q", m.state, m.applyID, m.response)
		}
	})
}

func TestAskActionPlanApplyingSpinnerAndEscape(t *testing.T) {
	m := NewAskViewModelWithDeps(nil, "", config.EmptyRuntimeState(""), nil)
	cmd := m.applyPlan(nil, false)
	if cmd == nil || m.state != askStateApplying || m.applyID == 0 {
		t.Fatalf("apply start = state %v id %d cmd %v", m.state, m.applyID, cmd)
	}
	operationID := m.applyID
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("apply command = %#v, want completion plus fresh spinner tick", batch)
	}
	completion, ok := batch[0]().(AskActionPlanAppliedMsg)
	if !ok || completion.OperationID != operationID || completion.Err == nil {
		t.Fatalf("apply completion = %#v, want matching unavailable-service failure", completion)
	}
	spinnerTick := batch[1]()
	if _, next := m.Update(spinnerTick); next == nil {
		t.Fatal("fresh applying spinner tick did not continue the animation chain")
	}

	_, escapeCmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if escapeCmd != nil || m.state != askStateApplying || m.applyID != operationID {
		t.Fatalf("applying escape = state %v id %d cmd %v", m.state, m.applyID, escapeCmd)
	}
	m.OnExit()
	if m.state != askStateApplying || m.applyID != operationID {
		t.Fatalf("applying exit invalidated transaction tracking: state=%v id=%d", m.state, m.applyID)
	}
}

func TestAskActionPlanShellCompletionIsExactlyOnce(t *testing.T) {
	appModel := newTestAppModel()
	ask := NewAskViewModelWithDeps(nil, "", appModel.runtime, nil)
	ask.state = askStateApplying
	ask.applyID = nextAskOperationID()
	ask.response = "reviewed preview"
	appModel.views.Register(ViewAsk, ask)
	appModel.pushView(ViewAsk)
	msg := AskActionPlanAppliedMsg{
		OperationID: ask.applyID,
		Result: app.ActionPlanResult{
			TransactionID: "tx-once",
			UndoID:        "undo-once",
			Actions:       []app.ActionPreview{{Index: 1}},
		},
	}

	_, _ = appModel.Update(msg)
	if ask.state != askStateResult || ask.applyID != 0 || strings.Count(ask.response, "Applied 1 action(s).") != 1 {
		t.Fatalf("first completion = state %v id %d response %q", ask.state, ask.applyID, ask.response)
	}
	if appModel.statusMessage != "" || len(appModel.messageLog.Entries()) != 0 {
		t.Fatalf("visible completion unexpectedly reported globally: status=%q log=%#v", appModel.statusMessage, appModel.messageLog.Entries())
	}

	_, _ = appModel.Update(msg)
	if ask.state != askStateResult || strings.Count(ask.response, "Applied 1 action(s).") != 1 {
		t.Fatalf("duplicate completion caused a second transition: state=%v response=%q", ask.state, ask.response)
	}
	if appModel.statusMessage != "" || len(appModel.messageLog.Entries()) != 0 || len(appModel.handledAskApplyIDs) != 1 {
		t.Fatalf("duplicate completion escaped exactly-once gate: status=%q log=%#v handled=%d", appModel.statusMessage, appModel.messageLog.Entries(), len(appModel.handledAskApplyIDs))
	}
}

func TestAskActionPlanShellReloadsEveryTouchedProjectAndInvalidatesCaches(t *testing.T) {
	root := t.TempDir()
	projects := make([]*models.Project, 0, 2)
	for i, id := range []string{"ask-touched-alpha", "ask-touched-beta"} {
		project, err := models.NewProject(id, id, "")
		if err != nil {
			t.Fatalf("NewProject(%s): %v", id, err)
		}
		project.Path = filepath.Join(root, "projects", project.Slug)
		task, err := models.NewTask(id+"-task", i+1, models.TaskTypeTask, "Persisted "+id)
		if err != nil {
			t.Fatalf("NewTask(%s): %v", id, err)
		}
		project.Tasks = []*models.Task{task}
		project.NextShortID = i + 2
		project.BuildTaskTree()
		if err := storage.SaveProject(project); err != nil {
			t.Fatalf("SaveProject(%s): %v", id, err)
		}
		projects = append(projects, project)
	}

	appModel := newTestAppModel()
	setTestAppProjects(appModel, projects...)
	for _, project := range projects {
		project.Tasks = nil
	}
	appModel.priorityViewFilterCache = map[string]*PriorityViewCache{"stale": {}}
	appModel.allTasksFilterCache = &AllTasksCache{}
	ask := NewAskViewModelWithDeps(nil, "", appModel.runtime, nil)
	ask.state = askStateApplying
	ask.applyID = nextAskOperationID()
	appModel.views.Register(ViewAsk, ask)
	appModel.pushView(ViewAsk)

	_, _ = appModel.Update(AskActionPlanAppliedMsg{
		OperationID: ask.applyID,
		Result: app.ActionPlanResult{
			TransactionID:   "tx-projects",
			UndoID:          "undo-projects",
			TouchedProjects: []string{projects[0].ID, projects[1].ID},
			Actions:         []app.ActionPreview{{Index: 1}},
		},
	})

	for _, project := range projects {
		if len(project.Tasks) != 1 || !strings.Contains(project.Tasks[0].Title, project.ID) {
			t.Fatalf("project %s was not reloaded from disk: %#v", project.ID, project.Tasks)
		}
	}
	if len(appModel.priorityViewFilterCache) != 0 || appModel.allTasksFilterCache != nil {
		t.Fatalf("task/filter caches were not invalidated: priority=%#v all=%#v", appModel.priorityViewFilterCache, appModel.allTasksFilterCache)
	}
}

//nolint:gocyclo // The outcome matrix keeps covered and replaced completion behavior comparable in one regression.
func TestAskActionPlanCoveredAndReplacedOutcomes(t *testing.T) {
	outcomes := []struct {
		name      string
		err       error
		warning   bool
		retryable bool
		discarded bool
	}{
		{name: "success"},
		{name: "retryable failure", err: errors.New("commit unavailable"), retryable: true},
		{name: "non-retryable failure", err: app.ErrPreparedActionPlanConsumed, discarded: true},
		{name: "repair warning", warning: true},
	}

	for _, placement := range []string{"covered", "replaced"} {
		for _, outcome := range outcomes {
			t.Run(placement+"/"+outcome.name, func(t *testing.T) {
				_, _, prepared := newAskMixedPreparedPlan(t, "ask-background-"+strings.ReplaceAll(placement+"-"+outcome.name, " ", "-"))
				appModel := newTestAppModel()
				unrelatedErr := errors.New("unrelated operation error")
				appModel.isLoading = true
				appModel.loadingMessage = "Unrelated operation in progress..."
				appModel.err = unrelatedErr
				if outcome.warning {
					blockedPath := filepath.Join(t.TempDir(), "not-a-directory")
					if err := os.WriteFile(blockedPath, []byte("block domain reload"), 0o600); err != nil {
						t.Fatalf("WriteFile: %v", err)
					}
					appModel.runtime = config.NewRuntimeState(blockedPath, config.NewDomains())
				}
				origin := NewAskViewModelWithDeps(nil, "", appModel.runtime, nil)
				origin.state = askStateApplying
				origin.applyID = nextAskOperationID()
				origin.prepared = prepared
				origin.response = "reviewed preview"
				appModel.views.Register(ViewAsk, origin)
				appModel.pushView(ViewAsk)

				var replacement *AskViewModel
				switch placement {
				case "covered":
					appModel.showView(ViewMessageLog, NewMessageLogViewModel(nil))
				case "replaced":
					replacement = NewAskViewModelWithDeps(nil, "", appModel.runtime, nil)
					replacement.response = "replacement input"
					appModel.showView(ViewAsk, replacement)
				}

				result := app.ActionPlanResult{
					TransactionID: "tx-background",
					UndoID:        "undo-background",
					Actions:       []app.ActionPreview{{Index: 1}},
				}
				if outcome.warning {
					result.DeletedTaskIDs = []string{"deleted-task"}
				}
				_, _ = appModel.Update(AskActionPlanAppliedMsg{
					OperationID:    origin.applyID,
					Result:         result,
					PendingDomains: config.NewDomains(),
					Err:            outcome.err,
				})

				if placement == "covered" {
					if appModel.currentView() != ViewMessageLog {
						t.Fatalf("covered completion changed current view to %v", appModel.currentView())
					}
					if origin.state != askStateResult || origin.applyID != 0 {
						t.Fatalf("covered Ask did not receive completion: state=%v id=%d", origin.state, origin.applyID)
					}
					if outcome.retryable && origin.prepared != prepared {
						t.Fatal("covered retryable failure discarded prepared plan")
					}
					if (outcome.discarded || outcome.err == nil) && origin.prepared != nil {
						t.Fatal("covered terminal outcome retained prepared plan")
					}
					if outcome.warning && !strings.Contains(origin.response, "Warning:") {
						t.Fatalf("covered warning was not rendered in Ask result: %q", origin.response)
					}
				} else {
					if replacement.state != askStateInput || replacement.applyID != 0 || replacement.response != "replacement input" {
						t.Fatalf("replacement Ask was mutated: state=%v id=%d response=%q", replacement.state, replacement.applyID, replacement.response)
					}
					if origin.state != askStateApplying || origin.applyID == 0 {
						t.Fatalf("unregistered origin was unexpectedly mutated: state=%v id=%d", origin.state, origin.applyID)
					}
				}

				entries := appModel.messageLog.Entries()
				if !appModel.isLoading || appModel.loadingMessage != "Unrelated operation in progress..." || !errors.Is(appModel.err, unrelatedErr) {
					t.Fatalf("hidden outcome clobbered unrelated app state: loading=%v message=%q err=%v", appModel.isLoading, appModel.loadingMessage, appModel.err)
				}
				if outcome.err != nil {
					if !strings.Contains(appModel.statusMessage, "AI action plan apply failed") || !messageLogHasLevel(entries, MessageLogLevelError) {
						t.Fatalf("hidden failure was not reported globally: status=%q log=%#v", appModel.statusMessage, entries)
					}
				} else {
					if !strings.Contains(appModel.statusMessage, "Applied 1 AI action(s)") ||
						!strings.Contains(appModel.statusMessage, "undo undo-background") || !messageLogHasLevel(entries, MessageLogLevelSuccess) {
						t.Fatalf("hidden success was not reported globally: status=%q log=%#v", appModel.statusMessage, entries)
					}
					if outcome.warning && (!strings.Contains(appModel.statusMessage, "warning:") || !messageLogHasLevel(entries, MessageLogLevelWarning)) {
						t.Fatalf("repair warning was not reported as a warning: status=%q log=%#v", appModel.statusMessage, entries)
					}
				}
			})
		}
	}
}

func TestAskActionPlanHiddenDeleteRepairsTaskReferences(t *testing.T) {
	const (
		deletedID  = "ask-hidden-deleted"
		survivorID = "ask-hidden-survivor"
	)
	domains := config.NewDomains()
	for _, taskID := range []string{deletedID, survivorID} {
		domains.Planning.FocusQueue = append(domains.Planning.FocusQueue, config.FocusQueueItem{TaskID: taskID, ProjectID: "p1"})
		domains.Planning.DailyPlan = append(domains.Planning.DailyPlan, config.DailyPlanItem{TaskID: taskID, ProjectID: "p1"})
		domains.Planning.FocusDismissedIDs = append(domains.Planning.FocusDismissedIDs, taskID)
		domains.Machine.RecentTaskVisits = append(domains.Machine.RecentTaskVisits, config.RecentTaskVisit{TaskID: taskID, ProjectID: "p1"})
	}
	runtime := config.NewRuntimeState("", domains)
	pending, err := runtime.CloneDomains()
	if err != nil {
		t.Fatalf("CloneDomains: %v", err)
	}
	pending.Planning.RemoveDurableTaskRefs(map[string]bool{deletedID: true})

	appModel := newTestAppModel()
	appModel.runtime = runtime
	replacement := NewAskViewModelWithDeps(nil, "", runtime, nil)
	replacement.response = "replacement input"
	appModel.views.Register(ViewAsk, replacement)
	appModel.pushView(ViewAsk)
	_, _ = appModel.Update(AskActionPlanAppliedMsg{
		OperationID: nextAskOperationID(),
		Result: app.ActionPlanResult{
			TransactionID:  "tx-hidden-delete",
			UndoID:         "undo-hidden-delete",
			DeletedTaskIDs: []string{deletedID},
			Actions:        []app.ActionPreview{{Index: 1}},
		},
		PendingDomains: pending,
	})

	if replacement.state != askStateInput || replacement.response != "replacement input" {
		t.Fatalf("hidden delete mutated replacement Ask: state=%v response=%q", replacement.state, replacement.response)
	}
	planning := runtime.Planning()
	if len(planning.FocusQueue) != 1 || planning.FocusQueue[0].TaskID != survivorID ||
		len(planning.DailyPlan) != 1 || planning.DailyPlan[0].TaskID != survivorID ||
		len(planning.FocusDismissedIDs) != 1 || planning.FocusDismissedIDs[0] != survivorID {
		t.Fatalf("hidden delete left durable refs: focus=%#v daily=%#v dismissed=%#v", planning.FocusQueue, planning.DailyPlan, planning.FocusDismissedIDs)
	}
	visits := runtime.Machine().RecentTaskVisits
	if len(visits) != 1 || visits[0].TaskID != survivorID {
		t.Fatalf("hidden delete left machine refs: %#v", visits)
	}
}

func messageLogHasLevel(entries []MessageLogEntry, level MessageLogLevel) bool {
	for _, entry := range entries {
		if entry.Level == level {
			return true
		}
	}
	return false
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
