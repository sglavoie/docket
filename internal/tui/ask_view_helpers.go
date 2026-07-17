package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

// HasTextInput returns true when the textarea is active.
func (m *AskViewModel) HasTextInput() bool {
	return m.state == askStateInput
}

// IsFullHeight returns true because the ask view renders its own footer.
func (m *AskViewModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *AskViewModel) ShouldCaptureEsc() bool {
	return m.state != askStateResult
}

// OnExit stops provider work before the shell hides or replaces this model.
// It is intentionally idempotent because some navigation paths can converge
// after an overlay has already caused the request to be canceled.
func (m *AskViewModel) OnExit() {
	m.cancelActiveRequest()
	m.preferenceSaveID = 0
	m.consentSaveID = 0
	// Applying work is intentionally neither canceled nor invalidated here. The
	// transaction may already be committing, and the shell uses applyID to route
	// its eventual completion even while this model is covered.
	if m.state == askStateSavingPreferences || m.state == askStateConsent || m.state == askStateLoading {
		m.state = askStateInput
		m.pendingQuery = ""
		m.textarea.Blur()
		m.configInput.Blur()
		m.focusIndex = askFocusQuery
	}
}

// OnResume restores visible-only work after Ask is revealed. Input resumes its
// cursor chain, while spinner states restart animation without taking focus.
func (m *AskViewModel) OnResume() tea.Cmd {
	switch m.state {
	case askStateInput:
		return m.focusAskField(askFocusQuery)
	case askStateLoading, askStateApplying:
		return m.spinner.Tick()
	default:
		return nil
	}
}

func (m *AskViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *AskViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewAsk)
}

// SetSize implements View.
func (m *AskViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	aWidth := max(width-4, 1)
	m.textarea.SetWidth(aWidth)
	m.configInput.SetWidth(max(width-18, 1))

	vpHeight := max(height-5, 1)
	vpWidth := max(width-2, 1)

	// Recreate renderer with updated word wrap width
	if r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(vpWidth),
	); err == nil {
		m.mdRenderer = r
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
		m.ready = true
	} else {
		m.viewport.SetWidth(vpWidth)
		m.viewport.SetHeight(vpHeight)
	}
}

func (m *AskViewModel) StatusBarLabel() string { return "Ask AI" }
