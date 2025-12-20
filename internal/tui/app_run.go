package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
)

// RunApp starts the TUI application.
func RunApp(cfg *config.Config, manager *config.Manager) error {
	model := NewAppModel(cfg, manager)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
