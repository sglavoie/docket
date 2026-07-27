package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

func effectiveUndoHistoryDepth(settings *config.Settings) int {
	if settings != nil && settings.UndoHistoryDepth != nil {
		return *settings.UndoHistoryDepth
	}
	return config.DefaultUndoHistoryDepth
}

func workspaceLedgerFromConfig(runtime *config.RuntimeState) *storage.WorkspaceLedger {
	dataDir := ""
	var settings *config.Settings
	if runtime != nil {
		dataDir = runtime.DataDir()
		settings = runtime.Settings()
	}
	depth := effectiveUndoHistoryDepth(settings)
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
}
