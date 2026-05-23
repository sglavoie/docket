package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// ScoreFilterInput encapsulates the inline text input for entering a custom
// score filter threshold. Both the project tree and outliner views embed this
// component instead of duplicating the input setup and key handling.
type ScoreFilterInput struct {
	Input  textinput.Model
	active bool
}

// IsActive returns true when the text input is visible and capturing keys.
func (s *ScoreFilterInput) IsActive() bool {
	return s.active
}

// Enter initializes and focuses the text input. If customValue is non-nil,
// the input is pre-filled with its formatted value.
func (s *ScoreFilterInput) Enter(customValue *float64) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "enter value"
	ti.CharLimit = 10
	ti.SetWidth(12)
	if customValue != nil {
		ti.SetValue(fmt.Sprintf("%.0f", *customValue))
	}
	ti.Focus()
	applyThemedTextInput(&ti)
	s.Input = ti
	s.active = true
}

// HandleKey processes a key event while the input is active.
// Returns the parsed value (nil if canceled or invalid) and whether the input
// session is done. The caller is responsible for applying the value to filters
// and persisting to config.
func (s *ScoreFilterInput) HandleKey(msg tea.KeyPressMsg) (value *float64, done bool, cmd tea.Cmd) {
	switch msg.String() {
	case "enter":
		val, err := strconv.ParseFloat(strings.TrimSpace(s.Input.Value()), 64)
		if err == nil && val > 0 {
			s.active = false
			return &val, true, nil
		}
		s.active = false
		return nil, true, nil
	case "esc":
		s.active = false
		return nil, true, nil
	}
	s.Input, cmd = s.Input.Update(msg)
	return nil, false, cmd
}

// ProcessKey handles a key event, persists the result to config, and updates
// the filter ScoreMin. The onApply callback is called when a valid value is
// accepted so the caller can refresh its view.
func (s *ScoreFilterInput) ProcessKey(msg tea.KeyPressMsg, cfg *config.Config, mgr *config.Manager, scoreMin *float64, onApply func()) tea.Cmd {
	val, done, cmd := s.HandleKey(msg)
	if done && val != nil {
		cfg.ScoreFilterCustom = val
		if mgr != nil {
			saveConfigWarn(mgr, cfg, "score filter preference")
		}
		*scoreMin = *val
		onApply()
	}
	return cmd
}

// View returns the rendered text input.
func (s *ScoreFilterInput) View() string {
	return s.Input.View()
}
