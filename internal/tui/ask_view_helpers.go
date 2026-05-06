package tui

import (
	"charm.land/bubbles/v2/viewport"
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
	return m.state == askStateInput || m.state == askStateLoading
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

	aWidth := max(width-4, 30)
	m.textarea.SetWidth(aWidth)

	vpHeight := max(height-5, 5)
	vpWidth := max(width-2, 30)

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

func (m *AskViewModel) StatusBarLabel() string { return "AI Assistant" }
