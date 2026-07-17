package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

func TestAskViewRunnerUsesRememberedHarnessConfigAndConsent(t *testing.T) {
	domains := config.NewDomains()
	if err := domains.Machine.SetAIHarness(aiharness.Codex); err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	if err := domains.Machine.SetAIConfigDir(aiharness.Codex, configDir); err != nil {
		t.Fatal(err)
	}
	if err := domains.Settings.AIConsent.GrantFor(aiharness.Codex, time.Now()); err != nil {
		t.Fatal(err)
	}
	runtime := config.NewRuntimeState("", domains)
	stream := newScriptedAskStream([]byte(`{"version":1,"summary":"ok","actions":[]}`), 5, nil)
	runner := &fakeAskRunner{streams: []external.Stream{stream}}
	project := &models.Project{ID: "p1", Slug: "demo", Name: "Demo"}
	m := NewAskViewModelWithRunner(project, "", runtime, nil, runner)

	if m.harness != aiharness.Codex || m.configDir != configDir {
		t.Fatalf("remembered selection = %s %q, want codex %q", m.harness, m.configDir, configDir)
	}
	driveAskCommand(t, m, m.submitQuery("help me"))

	requests := runner.Requests()
	if len(requests) != 1 {
		t.Fatalf("runner requests = %d, want 1", len(requests))
	}
	request := requests[0]
	if request.Harness != aiharness.Codex || request.ConfigDir != configDir || request.Query != "help me" {
		t.Fatalf("request = %+v", request)
	}
	if !strings.Contains(request.SystemPrompt, `"name":"Demo"`) {
		t.Fatalf("consented task context missing from prompt: %q", request.SystemPrompt)
	}
	if stream.ReadCalls() < 2 || stream.WaitCalls() != 1 {
		t.Fatalf("stream lifecycle = %d reads %d waits, want chunked reads and one wait", stream.ReadCalls(), stream.WaitCalls())
	}
}

func TestAskViewConsentIsSelectedHarnessSpecific(t *testing.T) {
	legacyTime := time.Now().UTC()
	domains := config.NewDomains()
	domains.Settings.AIConsent = config.AIConsentSettings{Granted: true, GrantedAt: &legacyTime}
	if err := domains.Machine.SetAIHarness(aiharness.Codex); err != nil {
		t.Fatal(err)
	}
	runtime := config.NewRuntimeState("", domains)
	runner := &fakeAskRunner{}
	project := &models.Project{ID: "p1", Slug: "demo", Name: "Demo"}
	m := NewAskViewModelWithRunner(project, "", runtime, nil, runner)
	m.textarea.SetValue("private tasks")

	_, preferenceCmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if preferenceCmd == nil {
		t.Fatal("selected harness preferences were not scheduled for persistence")
	}
	if next := driveAskPreferenceSave(t, m, preferenceCmd); next != nil || m.state != askStateConsent {
		t.Fatalf("legacy consent authorized codex: state=%v cmd=%v", m.state, next)
	}

	msg := m.saveContextConsent()()
	if saved, ok := msg.(AskConsentSavedMsg); !ok || saved.Err != nil {
		t.Fatalf("save consent message = %#v", msg)
	}
	_, resumeCmd := m.Update(msg)
	if resumeCmd == nil {
		t.Fatal("saved provider consent did not resume the pending query")
	}
	if !runtime.Settings().AIConsent.GrantedFor(aiharness.Codex) {
		t.Fatal("codex consent was not remembered")
	}
	if !runtime.Settings().AIConsent.GrantedFor(aiharness.Claude) {
		t.Fatal("pre-existing legacy Claude consent was lost")
	}
	if runtime.Settings().AIConsent.GrantedFor(aiharness.Copilot) {
		t.Fatal("codex consent leaked to copilot")
	}
	if len(runner.Requests()) != 0 {
		t.Fatal("runner was invoked before the resumed command executed")
	}
	if m.cancelFn != nil {
		m.cancelFn()
	}
}

func TestAskViewConsentPersistenceFailureDoesNotInvokeRunner(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataDir, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := config.NewRuntimeState(dataDir, config.NewDomains())
	runner := &fakeAskRunner{streams: []external.Stream{
		newScriptedAskStream([]byte(`{"version":1,"summary":"must not run","actions":[]}`), 16, nil),
	}}
	m := NewAskViewModelWithRunner(&models.Project{ID: "p1"}, "", runtime, nil, runner)
	m.state = askStateConsent
	m.pendingQuery = "private tasks"

	msg := m.saveContextConsent()()
	saved, ok := msg.(AskConsentSavedMsg)
	if !ok || saved.Err == nil {
		t.Fatalf("save consent message = %#v, want persistence error", msg)
	}
	_, _ = m.Update(msg)
	if m.state != askStateInput || len(runner.Requests()) != 0 {
		t.Fatalf("failed consent persistence = state %v requests %d", m.state, len(runner.Requests()))
	}
	if runtime.Settings().AIConsent.GrantedFor(aiharness.Claude) {
		t.Fatal("failed persistence mutated remembered consent in memory")
	}
}

func TestAskViewStaleConsentSaveAfterEscapeDoesNotInvokeRunner(t *testing.T) {
	runtime := config.EmptyRuntimeState("")
	runner := &fakeAskRunner{streams: []external.Stream{
		newScriptedAskStream([]byte(`{"version":1,"summary":"must not run","actions":[]}`), 16, nil),
	}}
	m := NewAskViewModelWithRunner(&models.Project{ID: "p1"}, "", runtime, nil, runner)
	m.state = askStateConsent
	m.pendingQuery = "private tasks"

	_, saveCmd := m.handleKey(tea.KeyPressMsg{Code: 'y'})
	if saveCmd == nil {
		t.Fatal("consent key did not schedule persistence")
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.state != askStateInput || m.pendingQuery != "" || m.consentSaveID != 0 {
		t.Fatalf("escape result = state %v pending query %q consent ID %d", m.state, m.pendingQuery, m.consentSaveID)
	}

	msg := saveCmd()
	if saved, ok := msg.(AskConsentSavedMsg); !ok || saved.Err != nil {
		t.Fatalf("save consent message = %#v", msg)
	}
	_, resumeCmd := m.Update(msg)
	if resumeCmd != nil {
		_ = askStartMessage(t, resumeCmd)
	}
	if requests := runner.Requests(); len(requests) != 0 {
		t.Fatalf("stale consent save invoked runner with requests %#v", requests)
	}
}

//nolint:gocyclo // Identity regressions intentionally cover zero, cross-instance, same-instance, and matching completions together.
func TestAskViewConsentSaveOperationIdentity(t *testing.T) {
	t.Run("zero is unscheduled", func(t *testing.T) {
		m := NewAskViewModelWithRunner(&models.Project{ID: "current"}, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
		m.state = askStateConsent
		m.pendingQuery = "current query"

		_, next := m.Update(AskConsentSavedMsg{})
		if next != nil || m.state != askStateConsent || m.pendingQuery != "current query" {
			t.Fatalf("zero consent operation = state %v pending %q cmd %v", m.state, m.pendingQuery, next)
		}
	})

	t.Run("previous instance", func(t *testing.T) {
		previousRuntime := config.EmptyRuntimeState("")
		previous := NewAskViewModelWithRunner(&models.Project{ID: "previous"}, "", previousRuntime, nil, &fakeAskRunner{})
		previous.state = askStateConsent
		previous.pendingQuery = "previous query"
		previousCmd := previous.saveContextConsent()

		currentRuntime := config.EmptyRuntimeState("")
		current := NewAskViewModelWithRunner(&models.Project{ID: "current"}, "", currentRuntime, nil, &fakeAskRunner{})
		current.state = askStateConsent
		current.pendingQuery = "current query"
		currentCmd := current.saveContextConsent()
		currentID := current.consentSaveID

		previousMsg, ok := previousCmd().(AskConsentSavedMsg)
		if !ok || previousMsg.Err != nil || previousMsg.OperationID == 0 || previousMsg.OperationID == currentID {
			t.Fatalf("previous consent save = %#v, current operation %d", previousMsg, currentID)
		}
		_, next := current.Update(previousMsg)
		if next != nil || current.state != askStateConsent || current.pendingQuery != "current query" || current.consentSaveID != currentID {
			t.Fatalf("stale consent completion = state %v pending %q id %d cmd %v", current.state, current.pendingQuery, current.consentSaveID, next)
		}
		if currentRuntime.Settings().AIConsent.GrantedFor(current.harness) {
			t.Fatal("stale consent completion mutated the replacement instance's consent")
		}

		currentMsg, ok := currentCmd().(AskConsentSavedMsg)
		if !ok || currentMsg.Err != nil || currentMsg.OperationID != currentID {
			t.Fatalf("current consent save = %#v, want operation %d", currentMsg, currentID)
		}
		_, providerCmd := current.Update(currentMsg)
		if providerCmd == nil || current.state != askStateLoading || current.pendingQuery != "" || current.requestQuery != "current query" || current.consentSaveID != 0 {
			t.Fatalf("matching consent completion = state %v pending %q request %q id %d cmd %v", current.state, current.pendingQuery, current.requestQuery, current.consentSaveID, providerCmd)
		}
		current.OnExit()
	})

	t.Run("earlier operation on same instance", func(t *testing.T) {
		previousRuntime := config.EmptyRuntimeState("")
		m := NewAskViewModelWithRunner(&models.Project{ID: "same"}, "", previousRuntime, nil, &fakeAskRunner{})
		m.state = askStateConsent
		m.pendingQuery = "previous query"
		previousCmd := m.saveContextConsent()

		currentRuntime := config.EmptyRuntimeState("")
		m.runtime = currentRuntime
		m.pendingQuery = "current query"
		currentCmd := m.saveContextConsent()
		currentID := m.consentSaveID

		previousMsg, ok := previousCmd().(AskConsentSavedMsg)
		if !ok || previousMsg.Err != nil || previousMsg.OperationID == 0 || previousMsg.OperationID == currentID {
			t.Fatalf("previous consent save = %#v, current operation %d", previousMsg, currentID)
		}
		_, next := m.Update(previousMsg)
		if next != nil || m.state != askStateConsent || m.pendingQuery != "current query" || m.consentSaveID != currentID {
			t.Fatalf("stale same-instance consent completion = state %v pending %q id %d cmd %v", m.state, m.pendingQuery, m.consentSaveID, next)
		}
		if currentRuntime.Settings().AIConsent.GrantedFor(m.harness) {
			t.Fatal("stale same-instance consent completion mutated current consent")
		}

		currentMsg, ok := currentCmd().(AskConsentSavedMsg)
		if !ok || currentMsg.Err != nil || currentMsg.OperationID != currentID {
			t.Fatalf("current consent save = %#v, want operation %d", currentMsg, currentID)
		}
		_, providerCmd := m.Update(currentMsg)
		if providerCmd == nil || m.state != askStateLoading || m.requestQuery != "current query" {
			t.Fatalf("matching same-instance consent completion = state %v request %q cmd %v", m.state, m.requestQuery, providerCmd)
		}
		m.OnExit()
	})
}

func TestAskViewEmptyQueryDoesNotInvokeRunner(t *testing.T) {
	runner := &fakeAskRunner{streams: []external.Stream{
		newScriptedAskStream([]byte(`{"version":1,"summary":"must not run","actions":[]}`), 16, nil),
	}}
	m := NewAskViewModelWithRunner(nil, "", nil, nil, runner)
	m.state = askStateConsent
	m.pendingQuery = "\t "

	requestID := m.requestID
	_ = m.submitQuery(m.pendingQuery)
	if m.state != askStateInput || m.pendingQuery != "" {
		t.Fatalf("empty submission result = state %v pending query %q", m.state, m.pendingQuery)
	}
	if m.requestID != requestID {
		t.Fatalf("empty submission advanced request ID from %d to %d", requestID, m.requestID)
	}
	if requests := runner.Requests(); len(requests) != 0 {
		t.Fatalf("empty submission invoked runner with requests %#v", requests)
	}
}

func TestAskViewStreamingSuccessPreviewsAndAppliesOnce(t *testing.T) {
	project, err := models.NewProject("ask-stream-success", "Ask Stream", "")
	if err != nil {
		t.Fatal(err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	service := app.NewService(newTestAppRepo(project), nil, nil)
	response := `{"version":1,"summary":"create it","actions":[{"type":"create_task","project_id":"` + project.ID + `","title":"From stream"}]}`
	stream := newScriptedAskStream([]byte(response), 9, nil)
	runner := &fakeAskRunner{streams: []external.Stream{stream}}
	m := NewAskViewModelWithRunner(project, "", config.EmptyRuntimeState(""), service, runner)
	m.SetSize(90, 24)
	m.contextMode = external.TaskContextNone

	driveAskCommand(t, m, m.submitQuery("create a task"))
	if m.state != askStateResult || m.prepared == nil || len(m.prepared.Preview().Actions) != 1 {
		t.Fatalf("stream result = state %v prepared %#v response %q", m.state, m.prepared, m.response)
	}
	if strings.Contains(m.View().Content, "reuse request") {
		t.Fatal("successful preview advertised failed-request reuse")
	}
	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: 'a'}); cmd != nil || m.state != askStateConfirmSafe {
		t.Fatalf("review key = state %v cmd %v", m.state, cmd)
	}
	_, applyCmd := m.handleKey(tea.KeyPressMsg{Code: 'y'})
	if applyCmd == nil {
		t.Fatal("safe confirmation did not produce apply command")
	}
	appModel := newTestAppModel()
	setTestAppProjects(appModel, project)
	appModel.views.Register(ViewAsk, m)
	appModel.pushView(ViewAsk)
	_, _ = appModel.Update(askActionPlanAppliedMessage(t, applyCmd))
	if m.prepared != nil || len(project.Tasks) != 1 || project.Tasks[0].Title != "From stream" {
		t.Fatalf("applied stream plan = prepared %#v tasks %#v", m.prepared, project.Tasks)
	}
	if _, second := m.handleKey(tea.KeyPressMsg{Code: 'a'}); second != nil {
		t.Fatal("successful streamed plan remained reusable")
	}
}

func TestAskViewEOFWaitSurfacesExitErrorSafely(t *testing.T) {
	waitErr := errors.New("codex exited with code 7: \x1b[2Jbad\rstderr")
	stream := newScriptedAskStream([]byte(`{"version":1,"summary":"ignored","actions":[]}`), 32, waitErr)
	runner := &fakeAskRunner{streams: []external.Stream{stream}}
	m := NewAskViewModelWithRunner(nil, "", nil, nil, runner)

	driveAskCommand(t, m, m.submitQuery("query"))
	if m.state != askStateResult || !strings.Contains(m.response, "codex exited with code 7") {
		t.Fatalf("nonzero exit response = state %v %q", m.state, m.response)
	}
	if strings.ContainsAny(m.response, "\x1b\r") || !strings.Contains(m.response, `\x1b[2Jbad\rstderr`) {
		t.Fatalf("nonzero exit was not terminal-safe: %q", m.response)
	}
	if stream.WaitCalls() != 1 {
		t.Fatalf("Wait calls = %d, want 1", stream.WaitCalls())
	}
}

func TestAskViewCancellationIsNonBlockingAndOldCompletionIsStale(t *testing.T) {
	oldStream := newBlockingAskStream()
	newStream := newScriptedAskStream([]byte(`{"version":1,"summary":"new result","actions":[]}`), 11, nil)
	runner := &fakeAskRunner{streams: []external.Stream{oldStream, newStream}}
	m := NewAskViewModelWithRunner(nil, "", nil, nil, runner)

	startMsg := askStartMessage(t, m.submitQuery("old request"))
	_, readCmd := m.Update(startMsg)
	if readCmd == nil {
		t.Fatal("stream start did not produce read command")
	}
	oldDone := make(chan tea.Msg, 1)
	go func() { oldDone <- readCmd() }()
	select {
	case <-oldStream.readStarted:
	case <-time.After(time.Second):
		t.Fatal("old stream read did not start")
	}

	started := time.Now()
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("Escape blocked for %s", elapsed)
	}
	if m.state != askStateInput || !oldStream.Canceled() {
		t.Fatalf("cancellation state = %v canceled=%v", m.state, oldStream.Canceled())
	}

	driveAskCommand(t, m, m.submitQuery("new request"))
	if m.state != askStateResult || !strings.Contains(m.raw, "new result") {
		t.Fatalf("new request result = state %v raw=%q response=%q", m.state, m.raw, m.response)
	}
	newResponse := m.response
	newRaw := m.raw
	select {
	case msg := <-oldDone:
		_, _ = m.Update(msg)
	case <-time.After(time.Second):
		t.Fatal("canceled read did not finish")
	}
	if m.state != askStateResult || m.response != newResponse || m.raw != newRaw {
		t.Fatalf("stale completion corrupted new result: state=%v raw=%q response=%q", m.state, m.raw, m.response)
	}
}

func TestAskViewPreviousInstanceStreamIsNotAdopted(t *testing.T) {
	previousStream := newScriptedAskStream([]byte("stale"), 5, nil)
	previous := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
		streams: []external.Stream{previousStream},
	})
	previousStart := askStartMessage(t, previous.submitQuery("previous request"))

	currentStream := newScriptedAskStream([]byte(`{"version":1,"summary":"current","actions":[]}`), 16, nil)
	current := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
		streams: []external.Stream{currentStream},
	})
	currentStart := askStartMessage(t, current.submitQuery("current request"))

	previousMsg, ok := previousStart.(AskStreamStartMsg)
	if !ok {
		t.Fatalf("previous start = %T, want AskStreamStartMsg", previousStart)
	}
	currentMsg, ok := currentStart.(AskStreamStartMsg)
	if !ok {
		t.Fatalf("current start = %T, want AskStreamStartMsg", currentStart)
	}
	if previousMsg.RequestID == currentMsg.RequestID {
		t.Fatalf("request IDs collided across model instances: %d", currentMsg.RequestID)
	}

	_, cleanup := current.Update(previousMsg)
	if cleanup == nil || !previousStream.Canceled() {
		t.Fatal("previous instance stream was adopted instead of canceled")
	}
	_, _ = current.Update(cleanup())
	if current.stream != nil || current.state != askStateLoading || current.raw != "" {
		t.Fatalf("stale start mutated current model: state=%v stream=%v raw=%q", current.state, current.stream, current.raw)
	}

	_, readCmd := current.Update(currentMsg)
	if readCmd == nil || current.stream != currentStream {
		t.Fatal("current instance stream was not adopted")
	}
	current.OnExit()
	if !currentStream.Canceled() {
		t.Fatal("current stream was not canceled during test cleanup")
	}
}

func TestAskViewOnResumeRestoresVisibleWork(t *testing.T) {
	t.Run("input", func(t *testing.T) {
		m := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
		m.textarea.Blur()
		m.configInput.Focus()
		m.focusIndex = askFocusConfigDir

		cmd := m.OnResume()
		if cmd == nil || !m.textarea.Focused() || m.configInput.Focused() || m.focusIndex != askFocusQuery {
			t.Fatalf("input resume = query focused %v config focused %v index %d cmd %v", m.textarea.Focused(), m.configInput.Focused(), m.focusIndex, cmd)
		}
	})

	for _, state := range []askState{
		askStateSavingPreferences,
		askStateConsent,
		askStateResult,
		askStateConfirmSafe,
		askStateConfirmDestructive,
	} {
		t.Run(fmt.Sprintf("state-%d", state), func(t *testing.T) {
			m := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
			m.state = state
			m.textarea.Blur()
			m.configInput.Blur()

			if cmd := m.OnResume(); cmd != nil || m.textarea.Focused() || m.configInput.Focused() {
				t.Fatalf("state %v resume = query focused %v config focused %v cmd %v", state, m.textarea.Focused(), m.configInput.Focused(), cmd)
			}
		})
	}

	for _, state := range []askState{askStateLoading, askStateApplying} {
		t.Run(fmt.Sprintf("spinner-state-%d", state), func(t *testing.T) {
			m := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
			m.state = state
			m.textarea.Blur()
			m.configInput.Blur()

			cmd := m.OnResume()
			if cmd == nil || m.textarea.Focused() || m.configInput.Focused() {
				t.Fatalf("state %v resume = query focused %v config focused %v cmd %v", state, m.textarea.Focused(), m.configInput.Focused(), cmd)
			}
			msg := cmd()
			if _, ok := msg.(spinner.TickMsg); !ok {
				t.Fatalf("state %v resume message = %T, want spinner.TickMsg", state, msg)
			}
		})
	}
}

func TestAskViewLifecycleCancelsStreamOnExitAndReplacement(t *testing.T) {
	ApplyTheme(ThemeConfig{})

	t.Run("covered loading resumes after palette closes", func(t *testing.T) {
		stream := newBlockingAskStream()
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
			streams: []external.Stream{stream},
		})
		start := askStartMessage(t, ask.submitQuery("covered request"))
		_, _ = ask.Update(start)

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)
		palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), ViewAsk, nil)
		app.showView(ViewCommandPalette, palette)

		if !stream.Canceled() || ask.cancelFn != nil || ask.state != askStateInput || ask.pendingQuery != "" {
			t.Fatalf("cover cleanup = canceled %v cancelFn %v state %v pending %q", stream.Canceled(), ask.cancelFn != nil, ask.state, ask.pendingQuery)
		}
		if ask.textarea.Focused() || ask.configInput.Focused() || ask.focusIndex != askFocusQuery {
			t.Fatalf("hidden reset focus = query %v config %v index %d", ask.textarea.Focused(), ask.configInput.Focused(), ask.focusIndex)
		}

		resumeCmd := app.popView()
		if got := app.views.Get(ViewAsk); got != ask {
			t.Fatalf("revealed ask = %p, want original %p", got, ask)
		}
		assertAskResumeBlink(t, app, ask, resumeCmd)
	})

	t.Run("covered consent resumes after palette closes", func(t *testing.T) {
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
		ask.state = askStateConsent
		ask.pendingQuery = "private query"
		ask.textarea.Blur()

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)
		palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), ViewAsk, nil)
		app.showView(ViewCommandPalette, palette)

		if ask.state != askStateInput || ask.pendingQuery != "" || ask.textarea.Focused() {
			t.Fatalf("covered consent = state %v pending %q focused %v", ask.state, ask.pendingQuery, ask.textarea.Focused())
		}
		assertAskResumeBlink(t, app, ask, app.popView())
	})

	t.Run("covered applying restarts spinner after palette closes", func(t *testing.T) {
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
		ask.state = askStateApplying
		ask.applyID = nextAskOperationID()
		ask.textarea.Blur()

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)
		palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), ViewAsk, nil)
		app.showView(ViewCommandPalette, palette)

		if ask.state != askStateApplying || ask.applyID == 0 {
			t.Fatalf("covered apply = state %v id %d", ask.state, ask.applyID)
		}
		resumeCmd := app.popView()
		if resumeCmd == nil {
			t.Fatal("revealed applying Ask did not restart its spinner")
		}
		msg := resumeCmd()
		if _, ok := msg.(spinner.TickMsg); !ok {
			t.Fatalf("applying resume message = %T, want spinner.TickMsg", msg)
		}
	})

	t.Run("exit", func(t *testing.T) {
		stream := newBlockingAskStream()
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
			streams: []external.Stream{stream},
		})
		start := askStartMessage(t, ask.submitQuery("exit request"))
		_, _ = ask.Update(start)

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)
		app.popView()

		if !stream.Canceled() || ask.cancelFn != nil || ask.state != askStateInput {
			t.Fatalf("exit cleanup = canceled %v cancelFn %v state %v", stream.Canceled(), ask.cancelFn != nil, ask.state)
		}
	})

	t.Run("popping ask does not resume it", func(t *testing.T) {
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
		ask.state = askStateConsent
		ask.pendingQuery = "private query"
		_ = ask.focusAskField(askFocusConfigDir)

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)

		if cmd := app.popView(); cmd != nil {
			t.Fatalf("popping Ask returned hidden resume command %v", cmd)
		}
		if app.currentView() != ViewDashboard || ask.state != askStateInput || ask.pendingQuery != "" {
			t.Fatalf("popped Ask = current %v state %v pending %q", app.currentView(), ask.state, ask.pendingQuery)
		}
		if ask.textarea.Focused() || ask.configInput.Focused() {
			t.Fatalf("popped Ask retained focus: query %v config %v", ask.textarea.Focused(), ask.configInput.Focused())
		}
	})

	t.Run("replacement", func(t *testing.T) {
		stream := newBlockingAskStream()
		ask := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{
			streams: []external.Stream{stream},
		})
		start := askStartMessage(t, ask.submitQuery("replacement request"))
		_, _ = ask.Update(start)

		app := newTestAppModel()
		app.views.Register(ViewAsk, ask)
		app.pushView(ViewAsk)
		replacement := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
		app.showView(ViewAsk, replacement)

		if !stream.Canceled() || ask.cancelFn != nil {
			t.Fatalf("replacement cleanup = canceled %v cancelFn %v", stream.Canceled(), ask.cancelFn != nil)
		}
		if got := app.views.Get(ViewAsk); got != replacement {
			t.Fatalf("registered ask = %p, want replacement %p", got, replacement)
		}
	})
}

func assertAskResumeBlink(t *testing.T, app *AppModel, ask *AskViewModel, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil || !ask.textarea.Focused() || ask.configInput.Focused() || ask.focusIndex != askFocusQuery {
		t.Fatalf("Ask resume = query focused %v config focused %v index %d cmd %v", ask.textarea.Focused(), ask.configInput.Focused(), ask.focusIndex, cmd)
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("Ask resume blink command returned nil message")
	}
	_, next := app.Update(msg)
	if next == nil {
		t.Fatal("Ask resume blink message did not continue the cursor blink chain")
	}
}

func TestAskViewIgnoresAndCleansUpStaleStart(t *testing.T) {
	m := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
	m.state = askStateLoading
	m.requestID = 2
	m.response = "current"
	stale := newScriptedAskStream([]byte("stale"), 5, nil)

	_, cleanup := m.Update(AskStreamStartMsg{RequestID: 1, Stream: stale})
	if cleanup == nil || !stale.Canceled() {
		t.Fatal("stale start was not canceled and scheduled for cleanup")
	}
	_, _ = m.Update(cleanup())
	if m.response != "current" || m.stream != nil || m.requestID != 2 {
		t.Fatalf("stale start mutated current state: response=%q stream=%v id=%d", m.response, m.stream, m.requestID)
	}
}

func TestAskViewStreamingOutputLimit(t *testing.T) {
	oversized := []byte(strings.Repeat("x", external.MaxHarnessOutputBytes+1))
	stream := newScriptedAskStream(oversized, 8192, nil)
	runner := &fakeAskRunner{streams: []external.Stream{stream}}
	m := NewAskViewModelWithRunner(nil, "", nil, nil, runner)

	driveAskCommand(t, m, m.submitQuery("too much"))
	if !strings.Contains(m.response, external.ErrHarnessOutputLimit.Error()) {
		t.Fatalf("output limit response = %q", m.response)
	}
	if len(m.raw) != external.MaxHarnessOutputBytes || !stream.Canceled() {
		t.Fatalf("retained=%d canceled=%v", len(m.raw), stream.Canceled())
	}
}

func TestAskViewRejectsMalformedStreamedPlan(t *testing.T) {
	stream := newScriptedAskStream([]byte("not-json"), 3, nil)
	runner := &fakeAskRunner{streams: []external.Stream{stream}}
	m := NewAskViewModelWithRunner(nil, "", nil, nil, runner)

	driveAskCommand(t, m, m.submitQuery("query"))
	if m.state != askStateResult || !strings.Contains(m.response, "Structured response rejected") || m.prepared != nil {
		t.Fatalf("malformed plan result = state %v prepared %#v response %q", m.state, m.prepared, m.response)
	}
}

func askStartMessage(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil submit command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("submit command = %#v, want non-empty tea.BatchMsg", batch)
	}
	return batch[0]()
}

func driveAskCommand(t *testing.T, m *AskViewModel, cmd tea.Cmd) {
	t.Helper()
	msg := askStartMessage(t, cmd)
	for step := 0; step < 10000; step++ {
		_, next := m.Update(msg)
		if next == nil {
			return
		}
		msg = next()
	}
	t.Fatal("ask stream did not finish within 10000 commands")
}

func driveAskPreferenceSave(t *testing.T, m *AskViewModel, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil preference-save command")
	}
	msg := cmd()
	if saved, ok := msg.(AskPreferencesSavedMsg); !ok || saved.Err != nil || saved.OperationID == 0 || saved.OperationID != m.preferenceSaveID {
		t.Fatalf("preference-save message = %#v", msg)
	}
	_, next := m.Update(msg)
	return next
}

type fakeAskRunner struct {
	mu       sync.Mutex
	streams  []external.Stream
	startErr error
	requests []external.Request
}

func (r *fakeAskRunner) Run(ctx context.Context, request external.Request) (string, error) {
	stream, err := r.Start(ctx, request)
	if err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(stream)
	waitErr := stream.Wait()
	if readErr != nil {
		return "", readErr
	}
	return string(data), waitErr
}

func (r *fakeAskRunner) Start(_ context.Context, request external.Request) (external.Stream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request)
	if r.startErr != nil {
		return nil, r.startErr
	}
	if len(r.streams) == 0 {
		return nil, errors.New("no fake stream configured")
	}
	stream := r.streams[0]
	r.streams = r.streams[1:]
	return stream, nil
}

func (r *fakeAskRunner) Requests() []external.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]external.Request(nil), r.requests...)
}

type scriptedAskStream struct {
	mu          sync.Mutex
	data        []byte
	offset      int
	chunkSize   int
	waitErr     error
	canceled    bool
	readCalls   int
	waitCalls   int
	cancelOnce  sync.Once
	cancelCh    chan struct{}
	blockRead   bool
	readStarted chan struct{}
	readOnce    sync.Once
}

func newScriptedAskStream(data []byte, chunkSize int, waitErr error) *scriptedAskStream {
	return &scriptedAskStream{
		data:      append([]byte(nil), data...),
		chunkSize: chunkSize,
		waitErr:   waitErr,
		cancelCh:  make(chan struct{}),
	}
}

func newBlockingAskStream() *scriptedAskStream {
	return &scriptedAskStream{
		chunkSize:   1,
		cancelCh:    make(chan struct{}),
		blockRead:   true,
		readStarted: make(chan struct{}),
	}
}

func (s *scriptedAskStream) Read(buffer []byte) (int, error) {
	s.mu.Lock()
	s.readCalls++
	block := s.blockRead
	canceled := s.canceled
	s.mu.Unlock()
	if canceled {
		return 0, context.Canceled
	}
	if block {
		s.readOnce.Do(func() { close(s.readStarted) })
		<-s.cancelCh
		return 0, context.Canceled
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.offset >= len(s.data) {
		return 0, io.EOF
	}
	chunkSize := s.chunkSize
	if chunkSize <= 0 || chunkSize > len(buffer) {
		chunkSize = len(buffer)
	}
	n := min(chunkSize, len(s.data)-s.offset)
	copy(buffer, s.data[s.offset:s.offset+n])
	s.offset += n
	return n, nil
}

func (s *scriptedAskStream) Close() error {
	s.Cancel()
	return s.Wait()
}

func (s *scriptedAskStream) Cancel() {
	s.mu.Lock()
	s.canceled = true
	s.mu.Unlock()
	s.cancelOnce.Do(func() { close(s.cancelCh) })
}

func (s *scriptedAskStream) Wait() error {
	s.mu.Lock()
	s.waitCalls++
	block := s.blockRead
	s.mu.Unlock()
	if block {
		<-s.cancelCh
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.canceled {
		return context.Canceled
	}
	return s.waitErr
}

func (s *scriptedAskStream) Stderr() string        { return "" }
func (s *scriptedAskStream) StderrTruncated() bool { return false }

func (s *scriptedAskStream) Canceled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.canceled
}

func (s *scriptedAskStream) ReadCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readCalls
}

func (s *scriptedAskStream) WaitCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.waitCalls
}
