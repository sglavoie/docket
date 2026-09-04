package projecthub

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// OpenProjectHubActions offers the cursor's project, or the whole selection
// when one exists. The selection wins because it is the deliberate, visible
// state: acting on the cursor while checkboxes are set would silently ignore
// them.
func (m *ProjectHubModel) OpenProjectHubActions() {
	if m.selectionCount() > 0 {
		m.openProjectHubBulkActions()
		return
	}
	entry, ok := m.selectedProjectHubEntry()
	if !ok || entry.Project == nil || m.surface == nil {
		return
	}
	target := projectHubActionTarget{ProjectID: entry.Project.ID}
	sheet, err := listsurface.NewContextActionSheetModel(
		m.registry,
		m.executor,
		m.projectHubCommandContext(target.ProjectID),
		m.surface.ActionState().OperationIDs,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostProjectHub,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Actions",
			Subtitle:     entry.Project.Name,
			Categories:   projectHubActionCategories,
			CategoryFor:  projectHubActionCategory,
			SecondaryFor: func(descriptor operation.OperationDescriptor) string { return string(descriptor.Risk) },
			Value:        target,
			HasValue:     true,
			EmptyState:   listsurface.ListEmptyState{Title: "No actions available."},
			NoMatchState: listsurface.ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		m.notice = err.Error()
		return
	}
	if len(sheet.OperationIDs()) == 0 {
		m.notice = "No administration actions are available for this project"
		return
	}
	m.actions = sheet
}

// openProjectHubBulkActions freezes the selected project identities into the
// command context instead of a single host value, so the executor resolves the
// same target set when the tag prompt resumes the request.
func (m *ProjectHubModel) openProjectHubBulkActions() {
	selected := m.surface.SelectedItems()
	identities := make([]operation.EntityIdentity, 0, len(selected))
	for _, item := range selected {
		entry, ok := item.Data.(projectHubEntry)
		if !ok || entry.Project == nil {
			continue
		}
		identities = append(identities, operation.ProjectEntity(entry.Project.ID))
	}
	if len(identities) == 0 {
		return
	}
	ctx := m.projectHubCommandContext("")
	ctx.Selection.EntityIdentities = identities

	sheet, err := listsurface.NewContextActionSheetModel(
		m.registry,
		m.executor,
		ctx,
		projectHubBulkOperationIDs(m.surface.ActionState().OperationIDs),
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostProjectHub,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Bulk Actions",
			Categories:   projectHubActionCategories,
			CategoryFor:  projectHubActionCategory,
			SecondaryFor: func(descriptor operation.OperationDescriptor) string { return string(descriptor.Risk) },
			Bulk:         true,
			Targets:      len(identities),
			EmptyState:   listsurface.ListEmptyState{Title: "No bulk actions available."},
			NoMatchState: listsurface.ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		m.notice = err.Error()
		return
	}
	if len(sheet.OperationIDs()) == 0 {
		m.notice = "Bulk tag actions apply only to active projects"
		return
	}
	m.actions = sheet
}

// projectHubBulkOperationIDs keeps the bulk sheet closed over the four tag
// actions, in their declared order, and drops any the selection does not share.
// An archived row in the selection carries none of them, so a mixed selection
// offers nothing rather than an action half of it would refuse.
func projectHubBulkOperationIDs(available []operation.OperationID) []operation.OperationID {
	ids := make([]operation.OperationID, 0, len(projectBulkOperations))
	for _, id := range projectBulkOperations {
		if slices.Contains(available, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func projectHubActionCategory(descriptor operation.OperationDescriptor) listsurface.ListCategoryID {
	for _, entry := range ProjectOperationManifest() {
		if entry.ID == descriptor.ID {
			return entry.Category
		}
	}
	return projectHubActionCategoryProject
}

func (m *ProjectHubModel) updateProjectHubActions(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.actions.Searching && (key.String() == " " || key.String() == "space") {
		key = tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	cmd, dismissed, message := m.actions.HandleKey(key)
	if dismissed {
		m.actions = nil
	}
	switch typed := message.(type) {
	case listsurface.ContextActionSheetClosedMsg:
		m.actions = nil
		return m, cmd
	case listsurface.ContextActionSheetResultMsg:
		return m, m.applyProjectHubExecutionResult(typed.Result)
	case status.StatusMsg:
		m.notice = string(typed)
	}
	return m, cmd
}

func (m *ProjectHubModel) applyProjectHubExecutionResult(result operation.OperationExecutionResult) tea.Cmd {
	if result.Err != nil {
		m.notice = result.Err.Error()
		return nil
	}
	// A value step is owned by the shell overlay above the Hub and resumes from
	// the frozen request, so the sheet has nothing left to do and closing it keeps
	// one modal on screen instead of two.
	if result.ValueRequired != nil {
		m.actions = nil
	}
	message := result.AsMessage()
	if message == nil {
		return nil
	}
	return func() tea.Msg { return message }
}
