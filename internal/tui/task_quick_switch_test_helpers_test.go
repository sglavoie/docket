package tui

import (
	"github.com/sglavoie/tt/internal/config"
)

func newTestAppForQuickSwitch() *AppModel {
	cfg := &config.Config{}
	manager := &config.Manager{}
	return &AppModel{
		config:        cfg,
		configManager: manager,
	}
}

// initQuickSwitchFilter initializes the filter index for tests that
// call methods depending on quickSwitchFiltered (pin, remove, etc.).
func initQuickSwitchFilter(app *AppModel) {
	app.rebuildQuickSwitchFilter()
}

func newTestAppWithPersisterForQuickSwitch() (*AppModel, *MockPersister) {
	cfg := &config.Config{}
	manager := &config.Manager{}
	mp := NewMockPersister()
	return &AppModel{
		config:        cfg,
		configManager: manager,
		persister:     mp,
	}, mp
}
