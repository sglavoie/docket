package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

// Update implements tea.Model.
//
//nolint:gocyclo // Message-specific transitions form one explicit asynchronous state machine.
func (m *AskViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case AskPreferencesSavedMsg:
		if m.state != askStateSavingPreferences || msg.OperationID == 0 || msg.OperationID != m.preferenceSaveID {
			return m, nil
		}
		m.preferenceSaveID = 0
		if msg.Err != nil {
			m.state = askStateInput
			m.pendingQuery = ""
			m.inputError = renderAskError("Could not save AI harness preferences", msg.Err)
			return m, m.focusAskField(askFocusQuery)
		}
		m.harness = msg.Harness
		m.configDir = msg.ConfigDir
		m.configDrafts = cloneAskConfigDrafts(msg.Drafts)
		m.configInput.SetValue(msg.ConfigDir)
		if m.project != nil && m.contextMode != external.TaskContextNone && !m.contextConsentGranted() {
			m.consentSaveID = 0
			m.state = askStateConsent
			return m, nil
		}
		query := m.pendingQuery
		return m, m.submitQuery(query)

	case AskStreamStartMsg:
		if msg.RequestID != m.requestID {
			if msg.Stream == nil {
				return m, nil
			}
			msg.Stream.Cancel()
			return m, waitForAskStreamCmd(msg.RequestID, msg.Stream, nil)
		}
		m.stream = msg.Stream
		return m, m.readNextChunk(msg.RequestID, msg.Stream)

	case AskStreamChunkMsg:
		if msg.RequestID != m.requestID {
			if msg.Stream == nil {
				return m, nil
			}
			msg.Stream.Cancel()
			return m, waitForAskStreamCmd(msg.RequestID, msg.Stream, msg.ReadErr)
		}
		if msg.Stream == nil {
			return m, func() tea.Msg {
				return AskStreamDoneMsg{RequestID: msg.RequestID, Err: errors.New("AI runner returned an empty stream")}
			}
		}
		remaining := external.MaxHarnessOutputBytes - len(m.raw)
		if remaining < 0 {
			remaining = 0
		}
		if len(msg.Text) > remaining {
			m.raw += msg.Text[:remaining]
			m.response = actionplan.TerminalSafeDisplayText(m.raw, actionplan.DisplayTextMultiline)
			m.viewport.SetContent(m.renderResponse())
			m.viewport.GotoBottom()
			msg.Stream.Cancel()
			return m, waitForAskStreamCmd(msg.RequestID, msg.Stream, external.ErrHarnessOutputLimit)
		}
		m.raw += msg.Text
		m.response = actionplan.TerminalSafeDisplayText(m.raw, actionplan.DisplayTextMultiline)
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		if msg.ReadErr != nil {
			return m, waitForAskStreamCmd(msg.RequestID, msg.Stream, msg.ReadErr)
		}
		return m, m.readNextChunk(msg.RequestID, msg.Stream)

	case AskStreamDoneMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		m.state = askStateResult
		m.stream = nil
		// Release the context's child resources. Without this call the
		// goroutine context.WithCancel spawns leaks until the parent is
		// canceled — but the parent is context.Background(), which never is.
		if m.cancelFn != nil {
			m.cancelFn()
			m.cancelFn = nil
		}
		if msg.Err != nil {
			m.response = renderAskError("Error", msg.Err)
		} else {
			plan, parseErr := external.ParseActionPlan(m.raw)
			if parseErr != nil {
				m.response = renderAskError("Structured response rejected", parseErr)
			} else if m.service == nil {
				m.response = "Structured response received, but the application service is unavailable."
			} else {
				prepared, prepareErr := m.service.PrepareActionPlan(plan)
				if prepareErr != nil {
					m.response = renderAskError("Action plan rejected", prepareErr)
				} else {
					m.prepared = prepared
					m.response = renderActionPlanMarkdown(prepared.Preview())
				}
			}
		}
		if m.prepared == nil && m.requestQuery != "" {
			m.failureQuery = m.requestQuery
			m.failureContextMode = m.requestContextMode
			m.reuseAvailable = true
		}
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoTop()
		return m, nil

	case AskConsentSavedMsg:
		if m.state != askStateConsent || msg.OperationID == 0 || msg.OperationID != m.consentSaveID {
			return m, nil
		}
		m.consentSaveID = 0
		if msg.Err != nil {
			m.state = askStateInput
			m.pendingQuery = ""
			m.inputError = renderAskError("Could not save AI context consent", msg.Err)
			return m, m.focusAskField(askFocusQuery)
		}
		query := m.pendingQuery
		m.pendingQuery = ""
		return m, m.submitQuery(query)

	case AskActionPlanAppliedMsg:
		if m.state != askStateApplying || msg.OperationID == 0 || msg.OperationID != m.applyID {
			return m, nil
		}
		m.applyID = 0
		m.state = askStateResult
		if msg.Err != nil {
			safeErr := askErrorText(msg.Err)
			switch {
			case isStalePreparedActionPlanError(msg.Err):
				m.prepared = nil
				m.acceptedSafe = false
				m.response += "\n\nApply failed; no action plan was committed. The workspace changed while you were reviewing it. Request a fresh AI response and review the new preview."
			case shouldDiscardPreparedActionPlan(msg.Err):
				m.prepared = nil
				m.acceptedSafe = false
				m.response += fmt.Sprintf("\n\nApply failed; no action plan was committed, and this preview can no longer be applied. Request a fresh AI response: %s", safeErr)
			default:
				m.response += fmt.Sprintf("\n\nApply failed; no action plan was committed: %s", safeErr)
			}
		} else {
			// A successful plan is single-use. Clear executable state before
			// returning to the result screen so the apply key and footer are disabled.
			m.prepared = nil
			m.acceptedSafe = false
			m.response += fmt.Sprintf("\n\nApplied %d action(s).\n\nTransaction ID: `%s`  \nUndo ID: `%s`", len(msg.Result.Actions), msg.Result.TransactionID, msg.Result.UndoID)
			if msg.RepairWarning != nil {
				m.response += fmt.Sprintf("\n\nWarning: the plan committed, but local reference repair was incomplete: %s", askErrorText(msg.RepairWarning))
			}
		}
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	if m.state == askStateLoading || m.state == askStateApplying {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	if m.state == askStateInput {
		var cmd tea.Cmd
		switch m.focusIndex {
		case askFocusQuery:
			m.textarea, cmd = m.textarea.Update(msg)
		case askFocusConfigDir:
			m.configInput, cmd = m.configInput.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

//nolint:gocyclo // State-specific key handling is kept together as an explicit state machine.
func (m *AskViewModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case askStateInput:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textarea.Value()) != "" {
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "ctrl+s":
			query := strings.TrimSpace(m.textarea.Value())
			if query == "" {
				return m, nil
			}
			return m, m.beginAskSubmission(query)
		case "ctrl+t":
			m.cycleContextMode()
			return m, nil
		case "tab":
			return m, m.cycleAskFocus(1)
		case "shift+tab":
			return m, m.cycleAskFocus(-1)
		case "left":
			if m.focusIndex == askFocusHarness {
				m.changeHarness(-1)
				return m, nil
			}
		case "right":
			if m.focusIndex == askFocusHarness {
				m.changeHarness(1)
				return m, nil
			}
		default:
			switch m.focusIndex {
			case askFocusQuery:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				return m, cmd
			case askFocusConfigDir:
				var cmd tea.Cmd
				m.configInput, cmd = m.configInput.Update(msg)
				m.syncConfigDraft()
				m.inputError = ""
				return m, cmd
			default:
				return m, nil
			}
		}
		if m.focusIndex == askFocusQuery {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
		if m.focusIndex == askFocusConfigDir {
			var cmd tea.Cmd
			m.configInput, cmd = m.configInput.Update(msg)
			m.syncConfigDraft()
			m.inputError = ""
			return m, cmd
		}
		return m, nil

	case askStateSavingPreferences:
		return m, nil

	case askStateConsent:
		switch strings.ToLower(msg.String()) {
		case "y":
			return m, m.saveContextConsent()
		case "n":
			m.consentSaveID = 0
			m.contextMode = external.TaskContextNone
			query := m.pendingQuery
			m.pendingQuery = ""
			return m, m.submitQuery(query)
		case "esc":
			m.consentSaveID = 0
			m.state = askStateInput
			m.pendingQuery = ""
			return m, m.focusAskField(askFocusQuery)
		}

	case askStateLoading:
		if msg.String() == "esc" {
			// Stream cancellation is non-blocking. The outstanding read command
			// observes closure, waits for process cleanup, and returns a stale
			// completion that the replacement request ID safely ignores.
			m.cancelActiveRequest()
			m.state = askStateInput
			return m, m.focusAskField(askFocusQuery)
		}

	case askStateResult:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "a":
			if m.prepared == nil {
				return m, nil
			}
			preview := m.prepared.Preview()
			if len(preview.Actions) > 0 {
				m.acceptedSafe = false
				if preview.SafeCount > 0 {
					m.state = askStateConfirmSafe
				} else {
					m.state = askStateConfirmDestructive
				}
				return m, nil
			}
		case "n":
			m.state = askStateInput
			m.textarea.Reset()
			m.inputError = ""
			m.response = ""
			m.raw = ""
			m.prepared = nil
			m.acceptedSafe = false
			m.failureQuery = ""
			m.reuseAvailable = false
			return m, m.focusAskField(askFocusQuery)
		case "r":
			if !m.reuseAvailable {
				return m, nil
			}
			m.state = askStateInput
			m.textarea.SetValue(m.failureQuery)
			m.contextMode = m.failureContextMode
			m.inputError = ""
			m.response = ""
			m.raw = ""
			m.prepared = nil
			m.acceptedSafe = false
			m.failureQuery = ""
			m.reuseAvailable = false
			return m, m.focusAskField(askFocusQuery)
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case askStateConfirmSafe:
		preview := m.prepared.Preview()
		switch strings.ToLower(msg.String()) {
		case "y":
			m.acceptedSafe = true
			if preview.DestructiveCount > 0 {
				m.state = askStateConfirmDestructive
				return m, nil
			}
			return m, m.applyAcceptedPlan(false)
		case "n", "esc":
			m.acceptedSafe = false
			if preview.DestructiveCount > 0 {
				m.state = askStateConfirmDestructive
			} else {
				m.state = askStateResult
			}
			return m, nil
		}

	case askStateConfirmDestructive:
		switch strings.ToLower(msg.String()) {
		case "y":
			return m, m.applyAcceptedPlan(true)
		case "n", "esc":
			if m.acceptedSafe {
				return m, m.applyAcceptedPlan(false)
			}
			m.state = askStateResult
			return m, nil
		}

	case askStateApplying:
		return m, nil
	}

	return m, nil
}

func (m *AskViewModel) cycleContextMode() {
	switch m.contextMode {
	case external.TaskContextFull:
		m.contextMode = external.TaskContextRedacted
	case external.TaskContextRedacted:
		m.contextMode = external.TaskContextNone
	default:
		m.contextMode = external.TaskContextFull
	}
}

func (m *AskViewModel) saveContextConsent() tea.Cmd {
	operationID := nextAskOperationID()
	m.consentSaveID = operationID
	runtime := m.runtime
	harness := m.harness
	return func() tea.Msg {
		if runtime == nil {
			return AskConsentSavedMsg{OperationID: operationID, Err: fmt.Errorf("runtime state is unavailable")}
		}
		domains, err := runtime.CloneDomains()
		if err != nil {
			return AskConsentSavedMsg{OperationID: operationID, Err: err}
		}
		now := time.Now().UTC()
		if err := domains.Settings.AIConsent.GrantFor(harness, now); err != nil {
			return AskConsentSavedMsg{OperationID: operationID, Err: err}
		}
		if runtime.DataDir() == "" {
			runtime.SetSettingsInMemory(domains.Settings)
			return AskConsentSavedMsg{OperationID: operationID}
		}
		return AskConsentSavedMsg{OperationID: operationID, Err: runtime.CommitPortableSettings(domains.Settings)}
	}
}

func renderAskError(prefix string, err error) string {
	return fmt.Sprintf("%s: %s", prefix, askErrorText(err))
}

func askErrorText(err error) string {
	return actionplan.TerminalSafeDisplayText(err.Error(), actionplan.DisplayTextMultiline)
}

func (m *AskViewModel) applyAcceptedPlan(includeDestructive bool) tea.Cmd {
	var risks []app.ActionRisk
	if m.acceptedSafe {
		risks = append(risks, app.ActionRiskSafe)
	}
	if includeDestructive {
		risks = append(risks, app.ActionRiskDestructive)
	}
	accepted := m.prepared.SelectRisks(risks...)
	if len(accepted.Preview().Actions) == 0 {
		m.state = askStateResult
		return nil
	}
	return m.applyPlan(accepted, includeDestructive)
}

func isStalePreparedActionPlanError(err error) bool {
	return errors.Is(err, storage.ErrStaleRevision) || errors.Is(err, storage.ErrStaleArtifact)
}

func shouldDiscardPreparedActionPlan(err error) bool {
	return errors.Is(err, app.ErrPreparedActionPlanConsumed) ||
		errors.Is(err, app.ErrPreparedActionPlanServiceMismatch) ||
		errors.Is(err, app.ErrInvalidInput) ||
		errors.Is(err, app.ErrProjectNotFound) ||
		errors.Is(err, app.ErrTaskNotFound)
}

func renderActionPlanMarkdown(preview app.PlanPreview) string {
	var b strings.Builder
	if preview.Summary != "" {
		b.WriteString(actionplan.TerminalSafeDisplayText(preview.Summary, actionplan.DisplayTextSingleLine))
		b.WriteString("\n\n")
	}
	if len(preview.Actions) == 0 {
		b.WriteString("No actions proposed.")
		return b.String()
	}
	b.WriteString("### Proposed actions\n\n")
	for _, action := range preview.Actions {
		fmt.Fprintf(&b, "%d. **%s** — %s\n", action.Index, action.Risk, actionplan.TerminalSafeDisplayText(action.Description, actionplan.DisplayTextMultiline))
	}
	return b.String()
}
