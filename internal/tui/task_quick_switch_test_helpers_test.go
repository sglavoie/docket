package tui

import "github.com/sglavoie/tt/internal/config"

func newTestAppForQuickSwitch() *AppModel {
	cfg := newTestConfig()
	manager := &config.Manager{}
	return &AppModel{
		configManager: manager,
		runtime:       testRuntimeAt("", cfg),
	}
}

func initQuickSwitchFilter(app *AppModel) {
	app.rebuildQuickSwitchFilter()
}
