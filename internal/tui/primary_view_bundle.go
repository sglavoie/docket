package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// PrimaryViewBundle groups domain-native dependencies and coordinators shared
// by primary task views. It deliberately does not embed ViewBundle: the bundles
// serve different view families while sharing the same facade-free runtime and
// application-layer coordinator pattern.
type PrimaryViewBundle struct {
	configManager  *config.Manager
	runtime        *config.RuntimeState
	bulk           BulkCoordinator
	taskItem       TaskItemCoordinator
	link           LinkCoordinator
	projectService *ProjectService
	quickEdit      *QuickEditComponent
	bulkOp         *BulkOperationComponent
}

// NewPrimaryViewBundle creates a bundle with a fresh ProjectService, QuickEdit,
// and BulkOp component.
func NewPrimaryViewBundle(mgr *config.Manager, runtime *config.RuntimeState) PrimaryViewBundle {
	return PrimaryViewBundle{
		configManager:  mgr,
		runtime:        runtime,
		projectService: NewProjectService(),
		quickEdit:      NewQuickEditComponent(),
		bulkOp:         NewBulkOperationComponent(),
	}
}

func (b *PrimaryViewBundle) SetBulkCoordinator(c BulkCoordinator)         { b.bulk = c }
func (b *PrimaryViewBundle) SetTaskItemCoordinator(c TaskItemCoordinator) { b.taskItem = c }
func (b *PrimaryViewBundle) SetLinkCoordinator(c LinkCoordinator)         { b.link = c }
func (b *PrimaryViewBundle) taskItemCoordinator() TaskItemCoordinator     { return b.taskItem }
func (b *PrimaryViewBundle) enabledWorkflowRules() []workflow.WorkflowRule {
	return b.runtime.Automation().GetEnabledWorkflowRules()
}
func (b *PrimaryViewBundle) workflowRules() []workflow.WorkflowRule {
	return b.enabledWorkflowRules()
}
