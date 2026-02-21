package tui

import "github.com/sglavoie/tt/internal/config"

// PrimaryViewBundle groups fields shared by all three primary project views
// (ProjectTree, Kanban, Outliner). Embed this anonymously so that existing
// references like m.config, m.quickEdit, etc. continue to work unchanged.
type PrimaryViewBundle struct {
	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService
	quickEdit      *QuickEditComponent
	bulkOp         *BulkOperationComponent
}

// NewPrimaryViewBundle creates a bundle with a fresh ProjectService, QuickEdit,
// and BulkOp component.
func NewPrimaryViewBundle(cfg *config.Config, mgr *config.Manager) PrimaryViewBundle {
	return PrimaryViewBundle{
		config:         cfg,
		configManager:  mgr,
		projectService: NewProjectService(),
		quickEdit:      NewQuickEditComponent(),
		bulkOp:         NewBulkOperationComponent(),
	}
}
