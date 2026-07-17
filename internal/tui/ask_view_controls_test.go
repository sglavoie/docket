package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

func TestAskViewFocusNavigationAndHarnessSelection(t *testing.T) {
	m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
	if m.focusIndex != askFocusQuery {
		t.Fatalf("initial focus = %d, want query", m.focusIndex)
	}

	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusIndex != askFocusHarness {
		t.Fatalf("focus after Tab = %d, want harness", m.focusIndex)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.contextMode != external.TaskContextRedacted {
		t.Fatalf("context mode from harness field = %s, want redacted", m.contextMode)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.harness != aiharness.Codex {
		t.Fatalf("harness after Right = %s, want codex", m.harness)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusIndex != askFocusConfigDir {
		t.Fatalf("focus after second Tab = %d, want config directory", m.focusIndex)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focusIndex != askFocusQuery {
		t.Fatalf("focus after two Shift+Tabs = %d, want query", m.focusIndex)
	}
}

func TestAskViewKeepsSeparateConfigDirectoryDrafts(t *testing.T) {
	m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
	claudeDir := t.TempDir()
	codexDir := t.TempDir()

	m.configInput.SetValue(claudeDir)
	m.syncConfigDraft()
	m.changeHarness(1)
	m.configInput.SetValue(codexDir)
	m.syncConfigDraft()
	m.changeHarness(-1)
	if got := m.configInput.Value(); got != claudeDir {
		t.Fatalf("Claude draft = %q, want %q", got, claudeDir)
	}
	m.changeHarness(1)
	if got := m.configInput.Value(); got != codexDir {
		t.Fatalf("Codex draft = %q, want %q", got, codexDir)
	}
}

func TestAskViewPersistsHarnessValidDraftsAndClearedOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := saveTestRuntime(manager, newTestConfigAt(dataDir)); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	claudeDir := t.TempDir()
	codexDir := t.TempDir()
	copilotDir := t.TempDir()
	machine := runtime.Machine()
	if setErr := machine.SetAIConfigDir(aiharness.Claude, claudeDir); setErr != nil {
		t.Fatal(setErr)
	}
	if setErr := machine.SetAIConfigDir(aiharness.Codex, codexDir); setErr != nil {
		t.Fatal(setErr)
	}
	if commitErr := runtime.CommitMachine(machine); commitErr != nil {
		t.Fatalf("seed machine preferences: %v", commitErr)
	}

	runner := &fakeAskRunner{}
	m := NewAskViewModelWithRunner(nil, "", runtime, nil, runner)
	m.changeHarness(1)
	m.configInput.SetValue("")
	m.syncConfigDraft()
	m.configDrafts[aiharness.Copilot] = copilotDir
	m.contextMode = external.TaskContextNone

	cmd := m.beginAskSubmission("remember these choices")
	if cmd == nil {
		t.Fatal("preference persistence was not scheduled")
	}
	msg, ok := cmd().(AskPreferencesSavedMsg)
	if !ok || msg.Err != nil {
		t.Fatalf("preference persistence = %#v", msg)
	}
	_, providerCmd := m.Update(msg)
	if providerCmd == nil || len(runner.Requests()) != 0 {
		t.Fatalf("preference result = provider cmd %v requests %d", providerCmd, len(runner.Requests()))
	}
	if m.cancelFn != nil {
		m.cancelFn()
	}

	reloaded, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	if got := reloaded.Machine().GetAIHarness(); got != aiharness.Codex {
		t.Fatalf("reloaded harness = %s, want codex", got)
	}
	if got := reloaded.Machine().GetAIConfigDir(aiharness.Codex); got != "" {
		t.Fatalf("cleared Codex override reloaded as %q", got)
	}
	if got := reloaded.Machine().GetAIConfigDir(aiharness.Claude); got != claudeDir {
		t.Fatalf("Claude override = %q, want %q", got, claudeDir)
	}
	if got := reloaded.Machine().GetAIConfigDir(aiharness.Copilot); got != copilotDir {
		t.Fatalf("Copilot override = %q, want %q", got, copilotDir)
	}
}

//nolint:gocyclo // Identity regressions intentionally cover zero, cross-instance, same-instance, and matching completions together.
func TestAskViewPreferenceSaveOperationIdentity(t *testing.T) {
	t.Run("zero is unscheduled", func(t *testing.T) {
		m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
		m.state = askStateSavingPreferences
		m.pendingQuery = "current query"

		_, next := m.Update(AskPreferencesSavedMsg{Harness: aiharness.Codex})
		if next != nil || m.state != askStateSavingPreferences || m.pendingQuery != "current query" || m.harness != aiharness.Claude {
			t.Fatalf("zero preference operation = state %v pending %q harness %s cmd %v", m.state, m.pendingQuery, m.harness, next)
		}
	})

	t.Run("previous instance", func(t *testing.T) {
		previous := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
		previous.contextMode = external.TaskContextNone
		previousDir := t.TempDir()
		previous.configInput.SetValue(previousDir)
		previousCmd := previous.beginAskSubmission("previous query")

		current := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
		current.contextMode = external.TaskContextNone
		current.changeHarness(1)
		currentDir := t.TempDir()
		current.configInput.SetValue(currentDir)
		currentCmd := current.beginAskSubmission("current query")
		currentID := current.preferenceSaveID

		previousMsg, ok := previousCmd().(AskPreferencesSavedMsg)
		if !ok || previousMsg.Err != nil {
			t.Fatalf("previous preference save = %#v", previousMsg)
		}
		if previousMsg.OperationID == 0 || currentID == 0 || previousMsg.OperationID == currentID {
			t.Fatalf("preference operation IDs = previous %d current %d", previousMsg.OperationID, currentID)
		}

		_, next := current.Update(previousMsg)
		if next != nil || current.state != askStateSavingPreferences || current.preferenceSaveID != currentID {
			t.Fatalf("stale preference completion = state %v id %d cmd %v", current.state, current.preferenceSaveID, next)
		}
		if current.pendingQuery != "current query" || current.harness != aiharness.Codex || current.configDir != currentDir {
			t.Fatalf("stale preference completion mutated current submission: query=%q harness=%s dir=%q", current.pendingQuery, current.harness, current.configDir)
		}
		if current.configDrafts[aiharness.Codex] != currentDir || current.configDrafts[aiharness.Claude] != "" {
			t.Fatalf("stale preference completion mutated drafts: %#v", current.configDrafts)
		}

		currentMsg, ok := currentCmd().(AskPreferencesSavedMsg)
		if !ok || currentMsg.Err != nil || currentMsg.OperationID != currentID {
			t.Fatalf("current preference save = %#v, want operation %d", currentMsg, currentID)
		}
		_, providerCmd := current.Update(currentMsg)
		if providerCmd == nil || current.state != askStateLoading || current.pendingQuery != "" || current.requestQuery != "current query" || current.preferenceSaveID != 0 {
			t.Fatalf("matching preference completion = state %v pending %q request %q id %d cmd %v", current.state, current.pendingQuery, current.requestQuery, current.preferenceSaveID, providerCmd)
		}
		current.OnExit()
	})

	t.Run("earlier operation on same instance", func(t *testing.T) {
		m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
		m.contextMode = external.TaskContextNone
		previousDir := t.TempDir()
		m.configInput.SetValue(previousDir)
		previousCmd := m.beginAskSubmission("previous query")

		m.changeHarness(1)
		currentDir := t.TempDir()
		m.configInput.SetValue(currentDir)
		currentCmd := m.beginAskSubmission("current query")
		currentID := m.preferenceSaveID

		previousMsg, ok := previousCmd().(AskPreferencesSavedMsg)
		if !ok || previousMsg.Err != nil || previousMsg.OperationID == 0 || previousMsg.OperationID == currentID {
			t.Fatalf("previous preference save = %#v, current operation %d", previousMsg, currentID)
		}
		_, next := m.Update(previousMsg)
		if next != nil || m.state != askStateSavingPreferences || m.preferenceSaveID != currentID {
			t.Fatalf("stale same-instance preference completion = state %v id %d cmd %v", m.state, m.preferenceSaveID, next)
		}
		if m.pendingQuery != "current query" || m.harness != aiharness.Codex || m.configDir != currentDir {
			t.Fatalf("stale same-instance preference completion mutated submission: query=%q harness=%s dir=%q", m.pendingQuery, m.harness, m.configDir)
		}

		currentMsg, ok := currentCmd().(AskPreferencesSavedMsg)
		if !ok || currentMsg.Err != nil || currentMsg.OperationID != currentID {
			t.Fatalf("current preference save = %#v, want operation %d", currentMsg, currentID)
		}
		_, providerCmd := m.Update(currentMsg)
		if providerCmd == nil || m.state != askStateLoading || m.requestQuery != "current query" {
			t.Fatalf("matching same-instance preference completion = state %v request %q cmd %v", m.state, m.requestQuery, providerCmd)
		}
		m.OnExit()
	})
}

func TestAskViewInvalidDirectoryAndPersistenceFailureDoNotInvokeRunner(t *testing.T) {
	t.Run("invalid selected directory", func(t *testing.T) {
		runner := &fakeAskRunner{}
		m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, runner)
		m.contextMode = external.TaskContextNone
		m.textarea.SetValue("do not send")
		m.configInput.SetValue(filepath.Join(t.TempDir(), "missing"))
		m.syncConfigDraft()
		_ = m.focusAskField(askFocusConfigDir)

		_, cmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		if cmd == nil || m.state != askStateInput || m.focusIndex != askFocusConfigDir {
			t.Fatalf("invalid directory = state %v focus %d cmd %v", m.state, m.focusIndex, cmd)
		}
		if len(runner.Requests()) != 0 || !strings.Contains(m.inputError, "does not exist") {
			t.Fatalf("invalid directory = requests %d error %q", len(runner.Requests()), m.inputError)
		}
	})

	t.Run("machine persistence failure", func(t *testing.T) {
		dataDir := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(dataDir, []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
		runner := &fakeAskRunner{}
		m := NewAskViewModelWithRunner(nil, "", config.NewRuntimeState(dataDir, config.NewDomains()), nil, runner)
		m.contextMode = external.TaskContextNone
		m.textarea.SetValue("do not send")

		_, cmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		if cmd == nil {
			t.Fatal("machine persistence was not scheduled")
		}
		msg := cmd()
		if saved, ok := msg.(AskPreferencesSavedMsg); !ok || saved.Err == nil {
			t.Fatalf("persistence message = %#v", msg)
		}
		_, next := m.Update(msg)
		if next == nil || m.state != askStateInput || m.focusIndex != askFocusQuery {
			t.Fatalf("persistence failure = state %v focus %d cmd %v", m.state, m.focusIndex, next)
		}
		if len(runner.Requests()) != 0 || !strings.Contains(m.inputError, "Could not save") {
			t.Fatalf("persistence failure = requests %d error %q", len(runner.Requests()), m.inputError)
		}
	})
}

func TestAskViewFailureReuseRestoresRequestWithoutSending(t *testing.T) {
	runner := &fakeAskRunner{startErr: errors.New("codex unavailable")}
	runtime := config.EmptyRuntimeState("")
	if err := runtime.Settings().AIConsent.GrantFor(aiharness.Claude, time.Now()); err != nil {
		t.Fatalf("grant current Claude consent: %v", err)
	}
	m := NewAskViewModelWithRunner(nil, "", runtime, nil, runner)
	m.SetSize(80, 24)
	m.contextMode = external.TaskContextRedacted
	m.textarea.SetValue("keep this exact request")

	_, preferenceCmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	providerCmd := driveAskPreferenceSave(t, m, preferenceCmd)
	driveAskCommand(t, m, providerCmd)
	if m.state != askStateResult || !m.reuseAvailable {
		t.Fatalf("failed request = state %v reusable %v", m.state, m.reuseAvailable)
	}
	if !strings.Contains(m.View().Content, "r: reuse request") {
		t.Fatalf("failed-request footer = %q", m.View().Content)
	}
	m.textarea.SetValue("changed while viewing the failure")

	_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'r'})
	if cmd == nil || m.state != askStateInput {
		t.Fatalf("reuse key = state %v cmd %v", m.state, cmd)
	}
	if got := m.textarea.Value(); got != "keep this exact request" {
		t.Fatalf("reused query = %q", got)
	}
	if m.contextMode != external.TaskContextRedacted {
		t.Fatalf("reused context mode = %s", m.contextMode)
	}
	if got := len(runner.Requests()); got != 1 {
		t.Fatalf("reuse sent %d requests, want the original request only", got)
	}
}

func TestAskViewDisplaysAvailabilityConsentAndConfigSources(t *testing.T) {
	t.Setenv("CODEX_HOME", "/inherited/codex")
	domains := config.NewDomains()
	if err := domains.Machine.SetAIHarness(aiharness.Codex); err != nil {
		t.Fatal(err)
	}
	runtime := config.NewRuntimeState("", domains)
	m := NewAskViewModelWithRunner(&models.Project{ID: "p1"}, "", runtime, nil, &fakeAskRunner{})
	m.available = func(aiharness.Name) bool { return false }
	m.SetSize(80, 24)

	view := m.View().Content
	for _, want := range []string{"Codex unavailable", "Codex CLI (codex): not found", "environment CODEX_HOME"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Ask input missing %q:\n%s", want, view)
		}
	}
	m.state = askStateConsent
	if consent := m.View().Content; !strings.Contains(consent, "configured Codex CLI provider") ||
		!strings.Contains(consent, "all active projects") ||
		!strings.Contains(consent, "read-only") ||
		!strings.Contains(consent, "strict JSON action-plan") {
		t.Fatalf("workspace-aware consent disclosure = %q", consent)
	}
}

func TestAskViewEscapeExitsNonModalStates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state askState
		draft string
	}{
		{name: "input with draft", state: askStateInput, draft: "unfinished request"},
		{name: "saving preferences", state: askStateSavingPreferences},
		{name: "result", state: askStateResult},
		{name: "applying", state: askStateApplying},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewAskViewModelWithRunner(nil, "", nil, nil, &fakeAskRunner{})
			m.state = tc.state
			m.textarea.SetValue(tc.draft)

			_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
			if cmd == nil {
				t.Fatal("Escape did not schedule navigation back")
			}
			if _, ok := cmd().(BackMsg); !ok {
				t.Fatalf("Escape emitted %T, want BackMsg", cmd())
			}
		})
	}
}

func TestAskViewNarrowLayoutStaysWithinTerminalWidth(t *testing.T) {
	m := NewAskViewModelWithRunner(nil, "", config.EmptyRuntimeState(""), nil, &fakeAskRunner{})
	m.available = func(aiharness.Name) bool { return false }
	m.configInput.SetValue("/a/very/long/configuration/directory/that/must/not/widen/the-screen")
	m.syncConfigDraft()

	for _, width := range []int{22, 12} {
		m.SetSize(width, 16)
		for lineNumber, line := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d rendered at %d columns: %q", width, lineNumber+1, got, line)
			}
		}
	}
}
