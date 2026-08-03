package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// Persist the canonical mode DTO for the active Outliner scope. Filter facets
// are run-local and typed navigation continues to own scope and route hierarchy.
func (m *AppModel) saveOutlinerFilterState() {
	outliner := m.crossOutliner
	if outliner == nil {
		return
	}
	if err := outliner.persistActiveRouteState(); err != nil {
		logging.Warn("failed to persist Outliner route state: %v", err)
	}
}

// applyTagsToTask applies selected tags to a single task as one asynchronous
// transaction through the BulkCoordinator, mirroring the multi-task
// applyTagsToTaskRefs sibling. Setting tags is an attribute mutation (no workflow
// rules run); the shell owns reload and toast via the batch result, and the
// durable write records through the workspace undo ledger.
func (m *AppModel) applyTagsToTask(task *models.Task, selectedTags []string) tea.Cmd {
	if task == nil {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTags([]string{task.ID}, selectedTags)
	return finalizeBatchResult(cmd, fmt.Sprintf("Tagged #%d", task.ShortID))
}

// applyTagsToTaskRefs applies selected tags to multiple task refs as one
// asynchronous cross-project transaction through the BulkCoordinator. Setting
// tags is an attribute mutation; the shell owns reload while preserving visual
// selection.
func (m *AppModel) applyTagsToTaskRefs(refs []TaskProjectRef, selectedTags []string) tea.Cmd {
	refs = validTaskProjectRefs(refs)
	locators := taskLocatorsFromTaskRefs(refs)
	if len(locators) == 0 {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTagsLocators(locators, selectedTags)
	return finalizeBatchResult(cmd, fmt.Sprintf("Tagged %d tasks", len(locators)))
}
