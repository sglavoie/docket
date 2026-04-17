package tui

import (
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// ThemedTextInputStyles is updated by ApplyTheme to reflect the active palette.
var ThemedTextInputStyles textinput.Styles

// ThemedTextareaStyles is updated by ApplyTheme to reflect the active palette.
var ThemedTextareaStyles textarea.Styles

// themedTextInputStyles builds a textinput.Styles using the active palette so
// input text, prompt, placeholder, and cursor remain readable under both dark
// and light themes. Without this, bubbles' defaults render text in terminal
// default fg ("7"/none), which becomes near-invisible on a painted light bg.
func themedTextInputStyles() textinput.Styles {
	textFg := ColorMuted
	if colorNormalText != "" {
		textFg = lipgloss.Color(colorNormalText)
	}
	s := textinput.DefaultStyles(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(textFg)
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(textFg)
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Cursor.Color = textFg
	return s
}

// themedTextareaStyles builds a textarea.Styles using the active palette so
// textarea content, placeholders, and cursor remain readable on both themes.
// In particular, it clears the dark backgrounds that bubbles v2 defaults inject
// (CursorLine, Base), which appear as dark strips on a painted light bg.
func themedTextareaStyles() textarea.Styles {
	textFg := ColorMuted
	if colorNormalText != "" {
		textFg = lipgloss.Color(colorNormalText)
	}
	s := textarea.DefaultStyles(true)
	// Base: fg only — no background so the app bg shows through.
	fgBase := lipgloss.NewStyle().Foreground(textFg)
	s.Focused.Base = fgBase
	s.Blurred.Base = fgBase
	// Text
	s.Focused.Text = lipgloss.NewStyle().Foreground(textFg)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(ColorMuted)
	// Placeholder
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.Placeholder = lipgloss.NewStyle().Foreground(ColorMuted)
	// Prompt
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(ColorMuted)
	// CursorLine: no background (primary cause of dark strip on light theme)
	s.Focused.CursorLine = lipgloss.NewStyle()
	// Line numbers
	s.Focused.LineNumber = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.LineNumber = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.CursorLineNumber = lipgloss.NewStyle().Foreground(ColorMuted)
	// EndOfBuffer: kill tilde artifacts on light bg
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(ColorMuted)
	s.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(ColorMuted)
	// Cursor color
	s.Cursor.Color = textFg
	return s
}

// applyThemedTextInput sets the current themed styles on a textinput.Model.
func applyThemedTextInput(ti *textinput.Model) {
	ti.SetStyles(ThemedTextInputStyles)
}

// applyThemedTextarea sets the current themed styles on a textarea.Model.
func applyThemedTextarea(ta *textarea.Model) {
	ta.SetStyles(ThemedTextareaStyles)
}
