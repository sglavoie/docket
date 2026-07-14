package tui

import (
	"github.com/sglavoie/tt/internal/config"
)

func newTestAppForQuickSwitch() *AppModel {
	cfg := newTestConfig()
	manager := &config.Manager{}
	return &AppModel{
		configManager: manager,
		runtime:       testRuntimeAt("", cfg),
	}
}

// initQuickSwitchFilter initializes the filter index for tests that
// call methods depending on quickSwitchFiltered (pin, remove, etc.).
func initQuickSwitchFilter(app *AppModel) {
	app.rebuildQuickSwitchFilter()
}

func newTestAppWithPersisterForQuickSwitch() (*AppModel, *MockPersister) {
	cfg := newTestConfig()
	manager := &config.Manager{}
	mp := NewMockPersister()
	return &AppModel{
		configManager: manager,
		persister:     mp,
		runtime:       testRuntimeAt("", cfg),
	}, mp
}
