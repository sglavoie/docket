package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) activeOutliner() *outliner.OutlinerModel {
	if m == nil || m.viewNav.navigation.Primary.Current().ID() != route.TaskWorkspace {
		return nil
	}
	return m.crossOutliner
}

// cursorProjectHeaderProject returns the project the Outliner cursor names when
// it rests on a project sub-header.
func (m *AppModel) cursorProjectHeaderProject() *models.Project {
	if out := m.activeOutliner(); out != nil {
		return out.CursorProjectHeaderProject()
	}
	return nil
}

// cursorNamesSectionHeader reports whether the Outliner cursor rests on a
// grouping section header.
func (m *AppModel) cursorNamesSectionHeader() bool {
	if out := m.activeOutliner(); out != nil {
		return out.CursorNamesSectionHeader()
	}
	return false
}

// cursorNoteIdentity returns the note the Outliner cursor names, so the shell's
// command context reads the same row kind the Outliner's own does.
func (m *AppModel) cursorNoteIdentity() (notes.NoteRowIdentity, bool) {
	if out := m.activeOutliner(); out != nil {
		return out.CursorNoteIdentity()
	}
	return notes.NoteRowIdentity{}, false
}

// cursorChecklistItem returns the checklist item the Outliner cursor names.
func (m *AppModel) cursorChecklistItem() (outliner.ChecklistCursor, bool) {
	if out := m.activeOutliner(); out != nil {
		return out.CursorChecklistItem()
	}
	return outliner.ChecklistCursor{}, false
}

// anchorOutlinerCursorAfterProjectPark points the cursor at the project header
// that will sit where the parked projects' rows were. It self-guards on the
// active Outliner, so a park committed from the Project Hub — where parked rows
// stay visible — leaves the cursor alone.
func (m *AppModel) anchorOutlinerCursorAfterProjectPark(projectIDs ...string) {
	out := m.activeOutliner()
	if out == nil {
		return
	}
	anchor, onScreen := out.ProjectHeaderAnchorAfterRemoval(projectIDs...)
	if !onScreen {
		return
	}
	out.Population.AnchorReloadHeader(anchor)
}

func (m *AppModel) outlinerTarget() (*models.Task, *models.Project) {
	if out := m.activeOutliner(); out != nil {
		return out.GetTaskRef()
	}
	return nil, nil
}

func (m *AppModel) applyProjectScope(msg outliner.ApplyProjectScopeMsg) (bool, tea.Cmd) {
	if m.baseProjectView() != presentation.ViewCrossProjectOutliner || m.crossOutliner == nil {
		m.setInfoStatus("Project scope is available from the global Outliner")
		return true, nil
	}
	returnToOutliner := m.currentView() == presentation.ViewProjectHub
	if msg.Clear {
		// A lens applied under this scope recorded project facets it could not act
		// on, because the scope had already fixed the population. Leaving the scope
		// is when they decide it, so the exit rides that load instead of dropping
		// the scope onto the population it replaced.
		activation, deferred := m.crossOutliner.BeginLensScopeExit()
		if !deferred {
			m.crossOutliner.ClearProjectScope()
		}
		if returnToOutliner {
			return true, tea.Batch(activation, m.returnToOutlinerAfterProjectScope())
		}
		return true, activation
	}
	if !m.crossOutliner.ProjectIsLoaded(msg.ProjectID) {
		// A parked project keeps its tasks on disk but contributes nothing to the
		// base workspace, so opening it is an explicit lazy load rather than an
		// error. The scope is pushed by the load result, not here.
		if cmd, started := m.startParkedProjectFocus(msg.ProjectID); started {
			return true, cmd
		}
		m.setInfoStatus("That project isn't loaded — restore it before scoping")
		return true, nil
	}
	m.crossOutliner.PushProjectScope(msg.ProjectID)
	if returnToOutliner {
		return true, m.returnToOutlinerAfterProjectScope()
	}
	return true, nil
}

// startParkedProjectFocus begins the lazy load that admits one parked project
// into the snapshot. The requesting surface stays open while the load runs, so a
// failure can report against the row the user chose instead of stranding them in
// an Outliner that never gained the project.
func (m *AppModel) startParkedProjectFocus(projectID string) (tea.Cmd, bool) {
	if m.domain.runtime == nil || projectID == "" {
		return nil, false
	}
	project := m.domain.runtime.AvailableProject(projectID)
	if project == nil {
		return nil, false
	}
	if m.domain.runtime.TaskWorkspaceProject(projectID) != nil {
		// In the base workspace but absent from the snapshot: a failed or stale
		// load, not a parked project. Leave the existing guidance in place.
		return nil, false
	}
	return m.crossOutliner.BeginParkedProjectFocus(projectID), true
}

// finishParkedProjectFocus closes the surface that requested a parked focus once
// the scope is actually in place. A load that failed, was superseded, or was
// abandoned leaves the requesting surface open, which is where its error belongs.
func (m *AppModel) finishParkedProjectFocus(focusProjectID string) tea.Cmd {
	if focusProjectID == "" || m.crossOutliner == nil {
		return nil
	}
	if m.crossOutliner.Scope.ProjectID != focusProjectID {
		return nil
	}
	if m.currentView() != presentation.ViewProjectHub {
		return nil
	}
	return m.returnToOutlinerAfterProjectScope()
}

// returnToOutlinerAfterProjectScope dismisses every primary child above the sole
// Outliner root without resetting the freshly applied project scope. Cancel uses
// the ordinary one-pop path instead so it can restore the prior view.
func (m *AppModel) returnToOutlinerAfterProjectScope() tea.Cmd {
	m.viewNav.navigation.ResetRoot()
	if view := m.viewNav.views.Get(m.currentView()); view != nil {
		m.setAccentOnView(view)
	}
	return func() tea.Msg { return status.ViewDidAppearMsg{} }
}

// The shell half of the relocation capture. outliner_relocation.go owns the
// capture itself and every cursor decision made against it; this file owns the
// two seams the shell needs — resolving a capture from an operation's targets,
// and carrying one alongside a transaction result until the committed rows are
// visible.

func (m *AppModel) captureOutlinerRelocationForTargets(targets operation.ResolvedOperationTargets) *outliner.OutlinerRelocationCapture {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	identities := make([]taskref.TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			identities = append(identities, target.Task)
		}
	}
	return out.CaptureOutlinerRelocation(identities)
}

// captureOutlinerRelocationForLocators is the same capture taken from addresses
// rather than from resolved operation targets. A queued deadline gesture is
// dispatched long after its invocation is gone and holds only the locators it
// froze, so it needs this door; nothing else does.
func (m *AppModel) captureOutlinerRelocationForLocators(targets []app.TaskLocator) *outliner.OutlinerRelocationCapture {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	identities := make([]taskref.TaskIdentity, 0, len(targets))
	for _, target := range targets {
		identity := taskref.FromLocator(target)
		if identity.Valid() {
			identities = append(identities, identity)
		}
	}
	return out.CaptureOutlinerRelocation(identities)
}

// captureOutlinerReviewAdvance is the acknowledgement's capture: the ordinary
// relocation snapshot plus the sequence of candidates the cursor may advance
// onto. It is taken before the commit because the rows it reads are the ones the
// user is looking at; a list rebuilt afterwards no longer knows who was next.
func (m *AppModel) captureOutlinerReviewAdvance(targets operation.ResolvedOperationTargets) *outliner.OutlinerRelocationCapture {
	capture := m.captureOutlinerRelocationForTargets(targets)
	if capture == nil {
		return nil
	}
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	out.PlanReviewAdvance(capture)
	return capture
}

// withOutlinerRelocationCapture decorates only successful batch results. Error
// messages pass through unchanged, so failed mutations cannot pollute history.
func withOutlinerRelocationCapture(cmd tea.Cmd, capture *outliner.OutlinerRelocationCapture) tea.Cmd {
	if cmd == nil || capture == nil {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		switch result := msg.(type) {
		case coordinator.BatchTransactionResultMsg:
			result.Relocation = capture.StampTransaction(result.TransactionID)
			return result
		case coordinator.TransactionResultMsg:
			result.Relocation = capture.StampTransaction(result.Result.TransactionID)
			return result
		default:
			return msg
		}
	}
}

func (m *AppModel) finishSynchronousOutlinerRelocation(cmd tea.Cmd, capture *outliner.OutlinerRelocationCapture, successMessage, transactionID string) tea.Cmd {
	recorded := false
	var jumpCmd tea.Cmd
	if m != nil && m.crossOutliner != nil && capture != nil {
		jumpCmd, recorded = m.crossOutliner.FinishOutlinerRelocation(capture, true)
	}
	feedbackCmd := m.setRecoveryFeedback(successMessage, transactionID, recorded)
	return tea.Batch(cmd, jumpCmd, feedbackCmd)
}

// handleOutlinerPlanningReorder persists a global Outliner planning-order reorder
// through the checked planning commit, rebuilds the in-memory planning caches, and
// reloads the cross outliner so the new order renders with the cursor kept on the
// moved task. The reorder is planning-domain only: task files are never rewritten.
//
// The Outliner computes the whole new workspace order and raises
// OutlinerPlanningReorderMsg; only the shell holds the application service that
// can commit it. See outliner_planning_order.go for the row side.
func (m *AppModel) handleOutlinerPlanningReorder(msg outliner.OutlinerPlanningReorderMsg) tea.Cmd {
	order := make([]app.TaskLocator, 0, len(msg.Order))
	for _, loc := range msg.Order {
		order = append(order, app.TaskLocator{ProjectID: loc.ProjectID, TaskID: loc.TaskID})
	}
	var err error
	switch msg.Kind {
	case outliner.ReorderNextOrder:
		err = m.domain.service.ReorderNext(order)
	case outliner.ReorderToday:
		err = m.domain.service.ReorderToday(order)
	}
	if err != nil {
		m.feedback.err = err
		return status.ErrorCmd("%s", err)
	}

	m.refreshRuntime()
	m.rebuildPlanningFromRuntime()
	if m.crossOutliner != nil {
		m.crossOutliner.Population.Invalidate()
	}
	m.setInfoStatus("Reordered planning list")
	return tea.Batch(m.refreshTaskViewsAfterPlanningMutation()...)
}

// The Outliner's project sheet groups into two categories only. Project holds
// the reversible edits and Danger the two acts that end the project's presence
// in the workspace, so the destructive pair is never mixed into the list a user
// arrows through looking for Edit.
const (
	outlinerProjectActionCategoryProject listsurface.ListCategoryID = "outliner-project-actions:project"
	outlinerProjectActionCategoryDanger  listsurface.ListCategoryID = "outliner-project-actions:danger"
)

var outlinerProjectActionCategories = []listsurface.ListCategory{
	{ID: outlinerProjectActionCategoryProject, Label: "Project"},
	{ID: outlinerProjectActionCategoryDanger, Label: "Danger"},
}

// outlinerProjectOperationManifest is the Outliner's project action list and the
// source the project sheet groups and orders by. It is deliberately separate
// from the Project Hub's ProjectOperationManifest: one-path is a per-surface
// rule, so the same operation is legitimately a direct key on one surface and a
// menu row on the other.
//
// The DirectOnly entries stay listed here so ValidateOperationOnePath can see
// them, but the sheet skips them: each already has a key on this surface. Pin
// joined them when the project header gained `*`, the same rule that took
// Comments and Attachments out of the task menu when they gained `;` and `&`.
// Edit project joined them last, when `e` learned to answer from a project
// header: the Outliner's `e` edits whatever the cursor names, so the menu row
// became the second path the rule exists to prevent. Add project comment joined
// on the same terms when `C` learned to answer from a project header.
//
// Three project operations are absent on purpose. Open project already has
// Enter through OperationCoreActivateGroup and Open project notes already has
// Ctrl+N through OperationCoreNotes; listing either would be the second path the
// rule exists to prevent. Unarchive project is absent for a different reason: an
// Outliner project header is drawn from TaskWorkspaceProjects(), which is
// non-archived by definition, so the row could never be offered. Archiving is
// therefore one-way from here — the Hub is where an archived project comes back.
func outlinerProjectOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operation.OperationProjectSetTags, Label: "Set project tags", Aliases: []string{"project tags"}, Provider: operation.ValueProviderProjectTags, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationProjectActions, Label: "Project actions", Aliases: []string{"project action sheet"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operation.OperationScopeRenameProject, Label: "Edit project", Aliases: []string{"project settings"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationProjectAddComment, Label: "Add project comment", Aliases: []string{"comment on project", "project comment"}, Provider: operation.ValueProviderProjectComment, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationProjectComments, Label: "Project comments", Aliases: []string{"read project comments"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operation.OperationScopePinProject, Label: "Toggle project pin", Aliases: []string{"pin project", "unpin project"}, Category: outlinerProjectActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationScopeArchiveProject, Label: "Archive project", Category: outlinerProjectActionCategoryDanger, Risk: operation.OperationRiskDestructive},
		{ID: operation.OperationProjectDelete, Label: "Delete project", Aliases: []string{"remove project", "destroy project"}, Category: outlinerProjectActionCategoryDanger, Risk: operation.OperationRiskDestructive},
	}
}

// outlinerProjectOperationManifestEntryFor is the closed allowlist behind the
// project sheet, the counterpart of taskOperationManifestEntryFor. The two stay
// separate so a project operation can never appear in the task Space menu and a
// task operation can never appear in this one.
func outlinerProjectOperationManifestEntryFor(id operation.OperationID) (operation.OperationManifestEntry, bool) {
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return operation.OperationManifestEntry{}, false
}

// projectActionSheetOperationDescriptor states the sheet's applicability apart
// from registerProjectOperations because that family shares one applicability —
// "the context names a project" — which a section header in a single-project
// scope also satisfies. This one asks the stricter question the row kind
// answers, so the key opens a project sheet only where a project header actually
// is.
func projectActionSheetOperationDescriptor() operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                   operation.OperationProjectActions,
		Label:                "Project Actions",
		Description:          "Open the project action menu",
		Aliases:              []string{"project action sheet", "project menu"},
		Category:             operation.OperationCategoryProject,
		TargetSource:         operation.OperationTargetCurrentEntity,
		TargetCardinality:    operation.TargetCardinalityOne,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Risk:                 operation.OperationRiskReadOnly,
		Applicability: func(ctx operation.CommandContext) bool {
			return operation.CoreOutlinerApplicability(ctx) &&
				!ctx.Selection.Visual &&
				!ctx.Entity.HasChecklistCursor &&
				ctx.Entity.HasProjectHeaderCursor
		},
	}
}

// registerProjectActionSheetOperation binds Space on a project header.
func (m *AppModel) registerProjectActionSheetOperation() error {
	if m.operations.Registry == nil || m.operations.Executor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	return m.registerOperation(projectActionSheetOperationDescriptor(), operation.OperationBinding{
		SingleHandler: func(
			invocation operation.OperationInvocation,
			_ operation.OperationTarget,
		) operation.OperationHandlerResult {
			if err := m.openProjectActionSheet(invocation.Context); err != nil {
				m.setInfoStatus(err.Error())
				return operation.OperationHandlerResult{Message: commandMessage(status.DefaultStatusTimeout())}
			}
			return operation.OperationHandlerResult{}
		},
	})
}

// NewProjectActionSheetModel builds the Outliner project sheet on the shared
// contextual engine. The CommandContext carries the frozen project identity, so
// a cursor move beneath the modal cannot retarget the eventual operation.
func NewProjectActionSheetModel(
	registry *operation.OperationRegistry,
	executor *operation.OperationExecutor,
	context operation.CommandContext,
	projectName string,
) (*listsurface.ContextActionSheetModel, error) {
	return listsurface.NewContextActionSheetModel(
		registry,
		executor,
		context,
		ProjectActionSheetOperationIDs(registry, context),
		listsurface.ContextActionSheetConfig{
			Host:            listsurface.ContextActionHostOutliner,
			Completion:      listsurface.ContextActionCompleteCloseSheet,
			Title:           "Project Actions",
			Subtitle:        strings.TrimSpace(projectName),
			ItemIDPrefix:    "project-action:",
			Categories:      outlinerProjectActionCategories,
			CategoryFor:     outlinerProjectActionCategory,
			SecondaryFor:    operation.ActionSecondaryText,
			ImmediateFilter: true,
			Targets:         1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No project actions available.",
				Detail: "Move the cursor to a project header.",
			},
			NoMatchState: listsurface.ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
}

// ProjectActionSheetOperationIDs returns the deterministic project-operation
// projection the sheet and its tests both read. Applicability is evaluated
// against the frozen project, so a row the project's own state forbids — pinning
// or archiving one that is already archived — drops out with no branching here:
// ProjectOperationRequirements already decides it.
func ProjectActionSheetOperationIDs(
	registry *operation.OperationRegistry,
	context operation.CommandContext,
) []operation.OperationID {
	if registry == nil {
		return []operation.OperationID{}
	}
	ids := make([]operation.OperationID, 0, len(outlinerProjectOperationManifest()))
	for _, entry := range outlinerProjectOperationManifest() {
		if entry.DirectOnly {
			continue
		}
		descriptor, ok := registry.Descriptor(entry.ID)
		if !ok || !descriptor.Applicable(context) {
			continue
		}
		if _, err := (operation.OperationTargetResolver{}).Resolve(descriptor, context); err != nil {
			continue
		}
		ids = append(ids, entry.ID)
	}
	return ids
}

func outlinerProjectActionCategory(descriptor operation.OperationDescriptor) listsurface.ListCategoryID {
	if entry, manifested := outlinerProjectOperationManifestEntryFor(descriptor.ID); manifested {
		return entry.Category
	}
	if descriptor.Risk == operation.OperationRiskDestructive {
		return outlinerProjectActionCategoryDanger
	}
	return outlinerProjectActionCategoryProject
}

// openProjectActionSheet raises the sheet for the project header under the
// cursor. The refusal names the row kind the key needs rather than the
// operation, so a user who pressed Space one row too high knows where to move.
func (m *AppModel) openProjectActionSheet(context operation.CommandContext) error {
	project := m.cursorProjectHeaderProject()
	if project == nil {
		return fmt.Errorf("move the cursor to a project header to open project actions")
	}
	sheet, err := NewProjectActionSheetModel(
		m.GetOperationRegistry(),
		m.operations.Executor,
		context,
		project.Name,
	)
	if err != nil {
		return err
	}
	m.operations.ActionSheet = sheet
	return nil
}
