package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

// Update implements tea.Model.
func (m *AskViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case AskStreamStartMsg:
		m.streamReader = msg.Reader
		m.streamCmd = msg.Cmd
		m.raw += msg.FirstText
		m.response = m.raw
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, m.readNextChunk()

	case AskStreamChunkMsg:
		m.raw += msg.Text
		m.response = m.raw
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoBottom()
		return m, m.readNextChunk()

	case AskStreamDoneMsg:
		m.state = askStateResult
		m.streamReader = nil
		m.streamCmd = nil
		// Release the context's child resources. Without this call the
		// goroutine context.WithCancel spawns leaks until the parent is
		// canceled — but the parent is context.Background(), which never is.
		if m.cancelFn != nil {
			m.cancelFn()
			m.cancelFn = nil
		}
		if msg.Err != nil {
			m.response = fmt.Sprintf("Error: %v", msg.Err)
		} else {
			plan, parseErr := external.ParseActionPlan(m.raw)
			if parseErr != nil {
				m.response = fmt.Sprintf("Structured response rejected: %v", parseErr)
			} else if m.service == nil {
				m.response = "Structured response received, but the application service is unavailable."
			} else {
				prepared, prepareErr := m.service.PrepareActionPlan(plan)
				if prepareErr != nil {
					m.response = fmt.Sprintf("Action plan rejected: %v", prepareErr)
				} else {
					m.prepared = prepared
					m.response = renderActionPlanMarkdown(prepared.Preview())
				}
			}
		}
		m.viewport.SetContent(m.renderResponse())
		m.viewport.GotoTop()
		return m, nil

	case AskConsentSavedMsg:
		if msg.Err != nil {
			m.state = askStateInput
			m.response = fmt.Sprintf("Could not save AI context consent: %v", msg.Err)
			m.textarea.Focus()
			return m, textarea.Blink
		}
		query := m.pendingQuery
		m.pendingQuery = ""
		return m, m.submitQuery(query)

	case AskActionPlanAppliedMsg:
		m.state = askStateResult
		if msg.Err != nil {
			switch {
			case isStalePreparedActionPlanError(msg.Err):
				m.prepared = nil
				m.acceptedSafe = false
				m.response += "\n\nApply failed; no action plan was committed. The workspace changed while you were reviewing it. Request a fresh AI response and review the new preview."
			case shouldDiscardPreparedActionPlan(msg.Err):
				m.prepared = nil
				m.acceptedSafe = false
				m.response += fmt.Sprintf("\n\nApply failed; no action plan was committed, and this preview can no longer be applied. Request a fresh AI response: %v", msg.Err)
			default:
				m.response += fmt.Sprintf("\n\nApply failed; no action plan was committed: %v", msg.Err)
			}
		} else {
			warning := repairAskActionPlanTaskRefs(m.runtime, msg.Result.DeletedTaskIDs, msg.PendingDomains)
			// A successful plan is single-use. Clear executable state before
			// returning to the result screen so the apply key and footer are disabled.
			m.prepared = nil
			m.acceptedSafe = false
			m.response += fmt.Sprintf("\n\nApplied %d action(s).\n\nTransaction ID: `%s`  \nUndo ID: `%s`", len(msg.Result.Actions), msg.Result.TransactionID, msg.Result.UndoID)
			if warning != nil {
				m.response += fmt.Sprintf("\n\nWarning: the plan committed, but local reference repair was incomplete: %v", warning)
			}
			if m.project != nil {
				_ = storage.LoadProject(m.project)
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
			if m.project != nil && m.contextMode != external.TaskContextNone && !m.contextConsentGranted() {
				m.pendingQuery = query
				m.state = askStateConsent
				m.textarea.Blur()
				return m, nil
			}
			return m, m.submitQuery(query)
		case "ctrl+t":
			m.cycleContextMode()
			return m, nil
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

	case askStateConsent:
		switch strings.ToLower(msg.String()) {
		case "y":
			return m, m.saveContextConsent()
		case "n":
			m.contextMode = external.TaskContextNone
			query := m.pendingQuery
			m.pendingQuery = ""
			return m, m.submitQuery(query)
		case "esc":
			m.state = askStateInput
			m.pendingQuery = ""
			m.textarea.Focus()
			return m, textarea.Blink
		}

	case askStateLoading:
		if msg.String() == "esc" {
			if m.cancelFn != nil {
				m.cancelFn()
			}
			// Close the reader pipe and reap the child process to prevent
			// zombie processes. The context cancellation kills the subprocess,
			// but Wait() is still needed to release OS resources.
			if m.streamReader != nil {
				_ = m.streamReader.Close()
			}
			if m.streamCmd != nil {
				_ = m.streamCmd.Wait()
			}
			m.state = askStateInput
			m.streamReader = nil
			m.streamCmd = nil
			m.textarea.Focus()
			return m, textarea.Blink
		}

	case askStateResult:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "a":
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
			m.textarea.Focus()
			m.response = ""
			m.raw = ""
			m.prepared = nil
			m.acceptedSafe = false
			return m, textarea.Blink
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
	runtime := m.runtime
	return func() tea.Msg {
		if runtime == nil {
			return AskConsentSavedMsg{Err: fmt.Errorf("runtime state is unavailable")}
		}
		domains, err := runtime.CloneDomains()
		if err != nil {
			return AskConsentSavedMsg{Err: err}
		}
		now := time.Now().UTC()
		domains.Settings.AIConsent = config.AIConsentSettings{Granted: true, GrantedAt: &now}
		if runtime.DataDir() == "" {
			runtime.SetSettingsInMemory(domains.Settings)
			return AskConsentSavedMsg{}
		}
		return AskConsentSavedMsg{Err: runtime.CommitPortableSettings(domains.Settings)}
	}
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
