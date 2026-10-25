package tui

import (
	"errors"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/notesops"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
)

// The shell satisfies the narrow ports the clusters below it declare. Each
// method here is a spelling of state the shell already owns, exported for one
// port and nothing else: a cluster that needs shell behavior asks for it by
// name in its own interface, which is what keeps the port a list rather than a
// second copy of AppModel.

// Outliner is the one scope-aware Outliner the shell owns.
func (m *AppModel) Outliner() *outliner.OutlinerModel {
	if m == nil {
		return nil
	}
	return m.crossOutliner
}

// Registry is the descriptor authority.
func (m *AppModel) Registry() *operation.OperationRegistry {
	if m == nil {
		return nil
	}
	return m.operations.Registry
}

// Executor is the single invocation pipeline.
func (m *AppModel) Executor() *operation.OperationExecutor {
	if m == nil {
		return nil
	}
	return m.operations.Executor
}

// RegisterOperation registers one descriptor and its binding.
func (m *AppModel) RegisterOperation(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
) error {
	return m.registerOperation(descriptor, binding)
}

// TaskWorkspaceProjects is the base Outliner population.
func (m *AppModel) TaskWorkspaceProjects() []*models.Project {
	return m.taskWorkspaceProjects()
}

// AvailableProjects is every non-archived project, parked ones included.
func (m *AppModel) AvailableProjects() []*models.Project {
	return m.availableProjects()
}

// ArchivedProjects completes the project-tag vocabulary, which counts over the
// whole catalog rather than the current facets.
func (m *AppModel) ArchivedProjects() []*models.Project {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	return m.domain.runtime.ArchivedProjects()
}

// Machine is the cached machine-local snapshot, or nil when none is available.
func (m *AppModel) Machine() *config.MachineState {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	return m.domain.runtime.Machine()
}

// CommitMachine publishes a validated machine snapshot, choosing the seam the
// workspace allows. With no writable workspace rooted, the in-memory seam is the
// whole commit: the runtime accessors observe the new state exactly as they
// would after a disk write.
func (m *AppModel) CommitMachine(machine *config.MachineState) error {
	if m == nil || m.domain.runtime == nil {
		return errors.New("workspace runtime is unavailable")
	}
	if m.domain.runtime.DataDir() == "" {
		m.domain.runtime.SetMachineInMemory(machine)
		return nil
	}
	return m.domain.runtime.CommitMachine(machine)
}

// commitSearchHistory uses the same machine-local domain as chrome preferences.
// Only accepted queries cross this boundary, never a search draft or view state.
func (m *AppModel) commitSearchHistory(entries []string) error {
	machine := m.Machine()
	if machine == nil {
		return errors.New("machine-local search history is unavailable")
	}
	if slices.Equal(machine.SearchHistory, entries) {
		return nil
	}
	next := machine.Clone()
	next.SearchHistory = slices.Clone(entries)
	return m.CommitMachine(next)
}

// registerFilterOperations, registerSortOperations, and registerLensOperations
// are the catalog's entries into the clusters below the shell. They stay
// AppModel methods because the catalog is a table of registrars named in order;
// the behavior itself lives in filterops and lensops.
func (m *AppModel) registerFilterOperations() error {
	return m.filters.RegisterFilterOperations(m)
}

func (m *AppModel) registerSortOperations() error {
	return m.filters.RegisterSortOperations(m)
}

func (m *AppModel) registerLensOperations() error {
	return m.lenses.RegisterLensOperations(m)
}

func (m *AppModel) registerAttachmentOperations() error {
	return m.entities.RegisterAttachmentOperations(m)
}

func (m *AppModel) registerCommentOperations() error {
	return m.entities.RegisterCommentOperations(m)
}

func (m *AppModel) registerChecklistRowOperations() error {
	return m.entities.RegisterChecklistRowOperations(m)
}

func (m *AppModel) registerProjectHubOperations() error {
	return m.project.RegisterProjectHubOperations(m)
}

func (m *AppModel) registerProjectOperations() error {
	return m.project.RegisterProjectOperations(m)
}

func (m *AppModel) registerProjectBulkOperations() error {
	return m.project.RegisterProjectBulkOperations(m)
}

func (m *AppModel) registerProjectTagVocabularyOperations() error {
	return m.project.RegisterProjectTagVocabularyOperations(m)
}

func (m *AppModel) registerNoteOperations() error {
	return m.notes.RegisterNoteOperations(m)
}

func (m *AppModel) registerNotesSurfaceOperations() error {
	return m.notes.RegisterNotesSurfaceOperations(m)
}

// MountActionSheet holds the contextual sheet a cluster below the shell opened.
// The overlay stack is the shell's, so the sheet is stored here whoever built
// it.
func (m *AppModel) MountActionSheet(sheet *listsurface.ContextActionSheetModel) {
	if m == nil {
		return
	}
	m.operations.ActionSheet = sheet
}

// RenderedView is the renderer installed under one presentation ID, which is
// how a cluster reaches the live panel it is acting on.
func (m *AppModel) RenderedView(id presentation.PresentationID) tea.Model {
	if m == nil || m.viewNav.views == nil {
		return nil
	}
	return m.viewNav.views.Get(id)
}

// SubviewCoordinator is the application-backed seam a checklist mutation
// commits through.
func (m *AppModel) SubviewCoordinator() coordinator.SubviewCoordinator {
	if m == nil {
		return coordinator.SubviewCoordinator{}
	}
	return m.secondaryFeatures.Subview
}

// OutlinerPrimary is the base Outliner, when the shell is holding one.
func (m *AppModel) OutlinerPrimary() (*outliner.OutlinerModel, bool) {
	return m.outlinerPrimary()
}

// TaskAndProjectForIdentity resolves a stable identity out of the current
// snapshot.
func (m *AppModel) TaskAndProjectForIdentity(
	identity taskref.TaskIdentity,
) (*models.Task, *models.Project) {
	return m.taskAndProjectForIdentity(identity)
}

// The lens cluster's shell-side spellings. Each is the same call the cluster
// exports, named as the shell already spelled it, so the picker host and the
// launch path read the same as before the behavior moved.
func (m *AppModel) commitActiveLensPointer(id string) error {
	return m.lenses.CommitActiveLensPointer(m, id)
}

func (m *AppModel) restorePendingLaunchLens() tea.Cmd {
	return m.lenses.RestorePendingLaunchLens(m)
}

func (m *AppModel) resolveLensTarget(identity operation.EntityIdentity) (config.NamedLens, error) {
	return m.lenses.ResolveLensTarget(m, identity)
}

func (m *AppModel) newLensTemplatePicker(
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	return m.lenses.NewLensTemplatePicker(m, request)
}

// OperationCommandContext is the view-scoped context a target-free sheet is
// built from.
func (m *AppModel) OperationCommandContext() operation.CommandContext {
	return m.operationCommandContext()
}

// ManageHub is the Manage destination the Project Hub is a module of.
func (m *AppModel) ManageHub() *projecthub.ManageHubModel {
	if m == nil {
		return nil
	}
	return m.manageHub
}

// ProjectByID resolves one project out of the loaded catalog by ID or slug,
// archived ones included: a project act checks the lifecycle itself, and a
// lookup that hid an archived project would refuse it as missing instead.
func (m *AppModel) ProjectByID(idOrSlug string) *models.Project {
	return m.projectByID(idOrSlug)
}

// AllProjectTags is the workspace-wide project-tag vocabulary.
func (m *AppModel) AllProjectTags() []string {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	return m.domain.runtime.AllProjectTags()
}

// ProjectTagVocabularyCounts is that same vocabulary counted by the facet that
// narrows on it.
func (m *AppModel) ProjectTagVocabularyCounts() map[string]int {
	counts, _ := m.filters.ProjectTagVocabularyCounts(m)
	return counts
}

// IsSystemUnfiledProject names the one project a delete must refuse.
func (m *AppModel) IsSystemUnfiledProject(id string) bool {
	if m == nil || m.domain.runtime == nil {
		return false
	}
	return m.domain.runtime.IsSystemUnfiledProject(id)
}

// The project cluster's shell-side spellings. Each is the same call the cluster
// exports, named as the shell already spelled it, so the Hub wiring and the
// project handlers read the same as before the behavior moved.
func (m *AppModel) projectHubTaskCounts() map[string]projecthub.ProjectTaskTally {
	return m.project.ProjectHubTaskCounts(m)
}

func (m *AppModel) refreshProjectHubIfActive() {
	m.project.RefreshProjectHubIfActive(m)
}

func (m *AppModel) openProjectTagVocabularyActions(tag string) tea.Cmd {
	return m.project.OpenProjectTagVocabularyActions(m, tag)
}

func (m *AppModel) newProjectTagDestinationPicker(
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	return m.project.NewProjectTagDestinationPicker(m, request)
}

func (m *AppModel) newProjectTagsPicker(
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	return m.project.NewProjectTagsPicker(m, request)
}

// DomainService is the application service a cluster's mutation commits
// through, and DomainRuntime is the workspace those effects are resolved below.
// They are the two domain handles the shell hands down whole, because a cluster
// that owns a mutation owns the call that submits it.
func (m *AppModel) DomainService() *app.Service {
	if m == nil {
		return nil
	}
	return m.domain.service
}

func (m *AppModel) DomainRuntime() *config.RuntimeState {
	if m == nil {
		return nil
	}
	return m.domain.runtime
}

// The notes cluster's shell-side spellings.
func (m *AppModel) requestNoteSearchIndexLoad() tea.Cmd {
	return m.notes.RequestNoteSearchIndexLoad(m)
}

func (m *AppModel) refreshNoteSearchIndexOwners() tea.Cmd {
	return m.notes.RefreshNoteSearchIndexOwners(m)
}

func (m *AppModel) handleNoteSearchIndexLoaded(msg notesops.NoteSearchIndexLoadedMsg) {
	m.notes.HandleNoteSearchIndexLoaded(m, msg)
}

func (m *AppModel) noteSearchIndexRows() []notes.LocatedNoteRow {
	return m.notes.NoteSearchIndexRows(m)
}

func (m *AppModel) resolveNoteTarget(identity operation.EntityIdentity) (notesops.ResolvedNote, error) {
	return m.notes.ResolveNoteTarget(m, identity)
}

// The operation host reads where the cursor is and builds the value step an
// invocation asks for. Every port below is one of those two: a cursor read the
// shell already answers, or a construction that dispatches across the shell's
// other clusters and so cannot move with the wiring.

// ActiveView is the live primary view the command context is resolved against.
func (m *AppModel) ActiveView() route.View { return m.activeView() }

// ActiveOutliner is the mounted Outliner, or nil when the route carries none.
func (m *AppModel) ActiveOutliner() *outliner.OutlinerModel { return m.activeOutliner() }

// CursorTaskAndProject is the task row under the cursor with its owning project.
func (m *AppModel) CursorTaskAndProject() (*models.Task, *models.Project) {
	return m.getCurrentCursorTaskAndProject()
}

// CursorProjectHeaderProject is the project a header row under the cursor names.
func (m *AppModel) CursorProjectHeaderProject() *models.Project {
	return m.cursorProjectHeaderProject()
}

// CursorChecklistItem is the checklist row under the cursor, when it is one.
func (m *AppModel) CursorChecklistItem() (outliner.ChecklistCursor, bool) {
	return m.cursorChecklistItem()
}

// CursorNamesSectionHeader reports whether the cursor sits on a section header.
func (m *AppModel) CursorNamesSectionHeader() bool { return m.cursorNamesSectionHeader() }

// CursorNoteIdentity is the note row under the cursor, when it is one.
func (m *AppModel) CursorNoteIdentity() (notes.NoteRowIdentity, bool) {
	return m.cursorNoteIdentity()
}

// CurrentProject is the stacked project view's own project, which is the scope
// an operation resolves against when no Outliner is mounted at all.
func (m *AppModel) CurrentProject() *models.Project { return m.currentProject }

// SetError retains an operation refusal on the shell's feedback line.
func (m *AppModel) SetError(err error) { m.feedback.err = err }

// NewValuePicker builds the list value step a provider asked for.
func (m *AppModel) NewValuePicker(
	request operation.OperationExecutionRequest,
	provider operation.ValueProviderID,
) (*valuepicker.OperationValuePickerModel, error) {
	return m.newOperationValuePicker(request, provider)
}

// NewTextPrompt builds the typed value step a provider asked for.
func (m *AppModel) NewTextPrompt(
	request operation.OperationExecutionRequest,
	provider operation.ValueProviderID,
) (*operationhost.OperationTextPromptModel, error) {
	return m.newOperationTextPrompt(request, provider)
}

// ResumeWithoutSheet runs a resumed request when no action sheet is mounted.
// The lens picker is the one surface an operation can be raised from without a
// sheet between, so it owns the resumed outcome the same way: a refusal stays
// on the list, and only completion tears it down. Both of its value-taking
// action rows resume here — the name prompt behind Save and the template
// gallery behind Add built-in view — so a refused add is retained on the picker
// exactly as a refused save is.
func (m *AppModel) ResumeWithoutSheet(request operation.OperationExecutionRequest) tea.Cmd {
	if picker := m.lenses.Picker(); picker != nil {
		result, dismissed := picker.Execute(request)
		if dismissed {
			m.closeNamedLensOverlays()
		}
		return m.applyNamedLensPickerResult(lens.NamedLensPickerResultMsg{Result: result})
	}
	return m.operations.ExecuteRequest(m, request)
}

// RefreshNamedLensPicker redraws the picker a dismissed sheet was opened over.
func (m *AppModel) RefreshNamedLensPicker() { m.refreshNamedLensPicker() }

// operationCommandContext is the shell's spelling of the context every
// dispatch site resolves its targets from.
func (m *AppModel) operationCommandContext() operation.CommandContext {
	return m.operations.CommandContext(m)
}

// operationTaskRefs resolves the selection, or the cursor when there is none,
// into the task and project pairs a handler mutates.
func (m *AppModel) operationTaskRefs(context operation.CommandContext) ([]taskref.TaskProjectRef, error) {
	return m.operations.TaskRefs(m, context)
}

// registerOperation declares one operation across both registries.
func (m *AppModel) registerOperation(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
) error {
	return m.operations.Register(descriptor, binding)
}
