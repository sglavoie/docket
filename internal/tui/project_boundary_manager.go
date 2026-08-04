package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The Boundary Manager is a child of the Project Hub rather than a fourth
// Manage module. Boundaries only mean anything as the grouping the Hub lists
// projects under, and administering them has to return to that listing: a
// module swap would unwind Esc all the way to the Outliner instead. Being a
// child also means the Hub's own refresh seam already reaches it, so one
// committed mutation rebuilds both surfaces from the same runtime.
//
// Like every other Hub child it is synchronous, because ProjectHubModel.Update
// drops every message that is not a key press and could never observe a
// command-driven submodel.

const (
	operationBoundaryCreate OperationID = "tui.boundary.create"
	operationBoundaryRename OperationID = "tui.boundary.rename"
	operationBoundaryDelete OperationID = "tui.boundary.delete"
)

const (
	boundaryManagerFooterLabel = "Boundaries"

	boundaryManagerItemPrefix = "boundary-manager:"

	boundaryManagerCategoryBoundaries ListCategoryID = "boundary-manager:boundaries"

	boundaryManagerActionCategoryBoundary ListCategoryID = "boundary-manager-actions:boundary"
	boundaryManagerActionCategoryDanger   ListCategoryID = "boundary-manager-actions:danger"
)

var boundaryManagerCategories = []ListCategory{
	{ID: boundaryManagerCategoryBoundaries, Label: "Boundaries"},
}

var boundaryManagerActionCategories = []ListCategory{
	{ID: boundaryManagerActionCategoryBoundary, Label: "Boundary"},
	{ID: boundaryManagerActionCategoryDanger, Label: "Danger"},
}

// projectBoundaryActionTarget freezes boundary identity when a browse key or the
// action sheet raises an operation, so later cursor movement cannot retarget the
// rename or deletion the user approved.
type projectBoundaryActionTarget struct {
	BoundaryID string
	Name       string
}

// projectBoundaryInlineActionMsg carries a registered operation back to the
// manager that raised it. Create, rename, and delete each need one more answer
// from the user before any mutation exists, so the registry names the action and
// the manager collects the rest.
type projectBoundaryInlineActionMsg struct {
	OperationID OperationID
	Target      projectBoundaryActionTarget
}

func projectBoundaryOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Aliases:              aliases,
			Category:             OperationCategoryProject,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationBoundaryCreate, "New boundary", "Create an empty boundary", OperationRiskMutation, "create boundary"),
		descriptor(operationBoundaryRename, "Rename boundary", "Retitle this boundary", OperationRiskMutation),
		descriptor(operationBoundaryDelete, "Delete boundary", "Delete this boundary and move its projects", OperationRiskDestructive),
	}
}

// registerProjectBoundaryOperations gives the Boundary Manager the same stable
// descriptors for its browse keys, its action sheet, and harness discovery.
func (m *AppModel) registerProjectBoundaryOperations() error {
	for _, descriptor := range projectBoundaryOperationDescriptors() {
		id := descriptor.ID
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
			Handler: func(invocation OperationInvocation) OperationHandlerResult {
				target, ok := invocation.Value.(projectBoundaryActionTarget)
				if !ok {
					return OperationHandlerResult{
						Err: fmt.Errorf("boundary action %s has no frozen target", id),
					}
				}
				return OperationHandlerResult{
					Message: projectBoundaryInlineActionMsg{OperationID: id, Target: target},
				}
			},
			// The boundary commit handlers own the checked transaction and the
			// manager owns the confirmation, so the registry stays the single
			// invocation catalog without duplicating either policy.
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// boundaryActionUnavailable names why one operation cannot apply to a boundary.
// It is the single decision the disabled action rows and the browse keys both
// read, so the sheet can never offer what a key press would refuse.
func boundaryActionUnavailable(boundaryID string, id OperationID) string {
	if boundaryID != models.ActiveBoundaryID {
		return ""
	}
	switch id {
	case operationBoundaryRename:
		return config.ActiveBoundaryName + " is the protected task workspace and cannot be renamed"
	case operationBoundaryDelete:
		return config.ActiveBoundaryName + " is the protected task workspace and cannot be deleted"
	default:
		return ""
	}
}

// boundaryMembers counts the projects one boundary holds. Counts come from
// catalog metadata rather than the loaded project cache, because a parked or
// archived project is not loaded and would otherwise count as zero.
type boundaryMembers struct {
	Total      int
	Unarchived int
	Archived   int
	Names      []string
}

func boundaryMemberIndex(catalog *config.WorkspaceCatalog) map[string]boundaryMembers {
	index := make(map[string]boundaryMembers)
	if catalog == nil {
		return index
	}
	for _, project := range catalog.Projects {
		id := resolvedProjectBoundaryID(catalog, project.ID)
		members := index[id]
		members.Total++
		if project.Archived {
			members.Archived++
		} else {
			members.Unarchived++
		}
		members.Names = append(members.Names, project.Name)
		index[id] = members
	}
	return index
}

func boundaryMemberSummary(members boundaryMembers) string {
	return fmt.Sprintf(
		"%d project%s · %d unarchived · %d archived",
		members.Total, pluralize(members.Total, "", "s"), members.Unarchived, members.Archived,
	)
}

func boundaryManagerItemID(boundaryID string) ListItemID {
	return ListItemID(boundaryManagerItemPrefix + boundaryID)
}

func boundaryManagerLabel(boundary config.ProjectBoundary) string {
	if boundary.ID == models.ActiveBoundaryID {
		return boundary.Name + " (protected)"
	}
	return boundary.Name
}

func boundaryManagerPreview(boundary config.ProjectBoundary, members boundaryMembers) ListPreview {
	kind := "Custom boundary"
	summary := "outside the task workspace"
	if boundary.ID == models.ActiveBoundaryID {
		kind = "Protected boundary"
		summary = "the task workspace"
	}
	body := "No projects are in this boundary."
	if len(members.Names) > 0 {
		body = "Projects\n" + strings.Join(members.Names, "\n")
	}
	return ListPreview{
		Title:   boundary.Name,
		Summary: kind + " · " + summary,
		Fields: []ListPreviewField{
			{Label: "Projects", Value: fmt.Sprintf("%d", members.Total)},
			{Label: "Unarchived", Value: fmt.Sprintf("%d", members.Unarchived)},
			{Label: "Archived", Value: fmt.Sprintf("%d", members.Archived)},
		},
		Body: body,
	}
}

func boundaryManagerItems(runtime *config.RuntimeState) []ListItem {
	catalog := runtime.Catalog()
	index := boundaryMemberIndex(catalog)
	boundaries := orderedProjectBoundaries(catalog)
	items := make([]ListItem, 0, len(boundaries))
	for _, boundary := range boundaries {
		members := index[boundary.ID]
		items = append(items, ListItem{
			ID:            boundaryManagerItemID(boundary.ID),
			CategoryID:    boundaryManagerCategoryBoundaries,
			PrimaryText:   boundaryManagerLabel(boundary),
			SecondaryText: boundaryMemberSummary(members),
			Aliases:       []string{boundary.Name, "boundary"},
			Preview:       boundaryManagerPreview(boundary, members),
			OperationIDs: []OperationID{
				operationBoundaryCreate,
				operationBoundaryRename,
				operationBoundaryDelete,
			},
			Data: projectBoundaryActionTarget{BoundaryID: boundary.ID, Name: boundary.Name},
		})
	}
	return items
}

// boundaryNameEditor collects the one field create and rename share. An empty
// boundaryID means creation, so the same prompt covers both without a second
// surface to keep in sync with the naming rules.
type boundaryNameEditor struct {
	boundaryID string
	original   string
	input      textinput.Model
	err        string
}

func (e *boundaryNameEditor) hasChanges() bool {
	return strings.TrimSpace(e.input.Value()) != e.original
}

// pendingBoundaryDelete is the frozen deletion the confirmation is guarding.
// Both IDs are captured before the prompt appears, so moving the cursor while it
// is open cannot change what the answer applies to.
type pendingBoundaryDelete struct {
	BoundaryID      string
	BoundaryName    string
	DestinationID   string
	DestinationName string
	Members         int
}

// projectBoundaryManager is the Project Hub's create, rename, and delete surface
// for boundaries.
type projectBoundaryManager struct {
	runtime  *config.RuntimeState
	registry *OperationRegistry
	executor *OperationExecutor

	surface     *ListSurfaceModel
	actions     *ContextActionSheetModel
	editor      *boundaryNameEditor
	destination *projectHubFilterPicker

	searching   bool
	searchInput textinput.Model
	query       string

	pending        pendingBoundaryDelete
	confirm        ConfirmDialog
	confirmDiscard bool
	discardOverlay ConfirmDialog

	notice string
}

func newProjectBoundaryManager(
	runtime *config.RuntimeState,
	registry *OperationRegistry,
	executor *OperationExecutor,
	width int,
) (*projectBoundaryManager, error) {
	surface, err := NewListSurfaceModel(
		boundaryManagerItems(runtime),
		boundaryManagerCategories,
		ListSurfaceConfig{
			SelectionMode: ListSelectionSingle,
			EmptyState: ListEmptyState{
				Title:  "No boundaries are defined.",
				Detail: "Press n to create one.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No boundaries match this search.",
				Detail: "Press Esc to clear the query.",
			},
		},
	)
	if err != nil {
		return nil, err
	}
	searchInput := formFactory.NewTextInput("Search boundaries")
	searchInput.CharLimit = 64
	manager := &projectBoundaryManager{
		runtime:     runtime,
		registry:    registry,
		executor:    executor,
		surface:     surface,
		searchInput: searchInput,
	}
	manager.resize(width)
	return manager, nil
}

func (manager *projectBoundaryManager) resize(width int) {
	formFactory.ResizeTextInput(&manager.searchInput, width)
	if manager.editor != nil {
		formFactory.ResizeTextInput(&manager.editor.input, width)
	}
}

func (manager *projectBoundaryManager) currentItemID() ListItemID {
	item, ok := manager.surface.CurrentItem()
	if !ok {
		return ""
	}
	return item.ID
}

// refresh re-derives the listing from the mutated catalog, keeping the cursor on
// the same durable boundary when it survived.
func (manager *projectBoundaryManager) refresh() {
	prefer := manager.currentItemID()
	if err := manager.surface.Reload(boundaryManagerItems(manager.runtime), boundaryManagerCategories); err != nil {
		manager.notice = err.Error()
		return
	}
	manager.surface.SetQuery(manager.query)
	if prefer != "" {
		manager.surface.SetCursorToID(prefer)
	}
}

func (manager *projectBoundaryManager) selectedTarget() (projectBoundaryActionTarget, bool) {
	item, ok := manager.surface.CurrentItem()
	if !ok {
		return projectBoundaryActionTarget{}, false
	}
	target, ok := item.Data.(projectBoundaryActionTarget)
	return target, ok
}

func (manager *projectBoundaryManager) focusBoundary(boundaryID string) {
	manager.surface.SetCursorToID(boundaryManagerItemID(boundaryID))
}

func (manager *projectBoundaryManager) openEditor(boundaryID, name string) {
	input := formFactory.NewTextInput("Boundary name")
	input.CharLimit = config.MaxBoundaryNameLength
	input.SetValue(name)
	input.CursorEnd()
	input.Focus()
	manager.editor = &boundaryNameEditor{boundaryID: boundaryID, original: name, input: input}
	manager.notice = ""
}

func (manager *projectBoundaryManager) closeEditor() {
	manager.editor = nil
	manager.confirmDiscard = false
	manager.discardOverlay.Cancel()
}

func (manager *projectBoundaryManager) startSearch() {
	manager.searching = true
	manager.searchInput.SetValue(manager.query)
	manager.searchInput.CursorEnd()
	manager.searchInput.Focus()
}

func (manager *projectBoundaryManager) clearSearch() {
	manager.searching = false
	manager.searchInput.Blur()
	manager.searchInput.SetValue("")
	manager.query = ""
	manager.surface.SetQuery("")
}

// destinationOptions lists every boundary that can receive the members of the
// one being deleted. The source is excluded because the catalog refuses a
// boundary as its own destination, and Active is always offered so a deletion
// can never be left without a valid answer.
func (manager *projectBoundaryManager) destinationOptions(sourceID string) []projectHubFilterOption {
	catalog := manager.runtime.Catalog()
	index := boundaryMemberIndex(catalog)
	boundaries := orderedProjectBoundaries(catalog)
	options := make([]projectHubFilterOption, 0, len(boundaries))
	for _, boundary := range boundaries {
		if boundary.ID == sourceID {
			continue
		}
		options = append(options, projectHubFilterOption{
			ID:    boundary.ID,
			Label: boundary.Name,
			Count: index[boundary.ID].Total,
		})
	}
	return options
}

func (manager *projectBoundaryManager) openDestinationPicker(sourceID string) {
	options := manager.destinationOptions(sourceID)
	if len(options) == 0 {
		manager.notice = "No other boundary can receive these projects"
		return
	}
	picker, err := newProjectHubFilterPicker(
		"Move projects to",
		"Type filter · ↑↓/Ctrl+j/k move · Enter choose · Esc cancel",
		options,
		"",
	)
	if err != nil {
		manager.notice = err.Error()
		return
	}
	manager.notice = ""
	manager.destination = picker
}

// armDeleteConfirmation states the source, the destination, and how many
// projects move, so the answer is given against the whole effect rather than a
// boundary name alone.
func (manager *projectBoundaryManager) armDeleteConfirmation() {
	pending := manager.pending
	prompt := fmt.Sprintf("Delete boundary %q? It holds no projects.", pending.BoundaryName)
	if pending.Members > 0 {
		prompt = fmt.Sprintf(
			"Delete boundary %q? Its %d project%s move to %q.",
			pending.BoundaryName,
			pending.Members,
			pluralize(pending.Members, "", "s"),
			pending.DestinationName,
		)
	}
	manager.notice = ""
	manager.confirm.ShowWithDetail("delete-boundary", prompt, &ConfirmDetail{UndoHint: true})
}

func (manager *projectBoundaryManager) confirmDialog() *ConfirmDialog {
	if manager.confirmDiscard {
		manager.discardOverlay.Show("discard", "Discard unsaved changes?")
		return &manager.discardOverlay
	}
	manager.discardOverlay.Cancel()
	return &manager.confirm
}

func (manager *projectBoundaryManager) subtitle() string {
	if manager.editor != nil {
		if manager.editor.boundaryID == "" {
			return "new boundary"
		}
		return "renaming " + manager.editor.original
	}
	custom := max(len(orderedProjectBoundaries(manager.runtime.Catalog()))-1, 0)
	return fmt.Sprintf(
		"%s · %d custom boundar%s",
		config.ActiveBoundaryName, custom, pluralize(custom, "y", "ies"),
	)
}

func (manager *projectBoundaryManager) searchBar() string {
	if manager.searching {
		return "/" + manager.searchInput.View()
	}
	if manager.query == "" {
		return ""
	}
	return "/" + manager.query +
		MutedStyle.Render(fmt.Sprintf("  (%d matches)", len(manager.surface.Matches())))
}

func (manager *projectBoundaryManager) view(width, height int) string {
	if manager.editor != nil {
		return manager.editorView(width)
	}
	if manager.destination != nil {
		return manager.destination.render(width)
	}
	if manager.actions != nil {
		return manager.actions.render(width)
	}
	return NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:       width,
		Height:      height,
		Layout:      ListSurfaceLayoutAuto,
		ShowPreview: true,
		HideHelp:    true,
	}).Render(manager.surface)
}

func (manager *projectBoundaryManager) editorView(width int) string {
	editor := manager.editor
	title := "New Boundary"
	if editor.boundaryID != "" {
		title = "Rename Boundary: " + editor.original
	}
	var b strings.Builder
	b.WriteString(SectionHeaderStyle.Render(truncateToVisualWidth(title, width)))
	b.WriteString("\n\n")
	b.WriteString(FieldLabelFocusedStyle.Render("Name: ") + editor.input.View())
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render(truncateToVisualWidth(fmt.Sprintf(
		"Names are trimmed, must be unique, and hold at most %d characters.",
		config.MaxBoundaryNameLength,
	), width)))
	if editor.err != "" {
		b.WriteString("\n\n")
		b.WriteString(ValidationErrorStyle.Render(truncateToVisualWidth(editor.err, width)))
	}
	return b.String()
}

func (manager *projectBoundaryManager) footerHelpVariants() []string {
	switch {
	case manager.editor != nil:
		return []string{
			"Type a name · Enter save · Esc cancel",
			"Enter save · Esc cancel",
		}
	case manager.destination != nil:
		return []string{
			"Type filter · ↑↓/Ctrl+j/k move · Enter choose · Esc cancel",
			"↑↓ move · Enter choose · Esc cancel",
			"Enter · Esc",
		}
	case manager.actions != nil:
		return []string{
			"j/k choose · Enter run · / search · Esc back",
			"Enter run · Esc back",
		}
	case manager.searching:
		return []string{
			"Type to filter · ↑↓/Ctrl+j/k move · Enter keep · Esc clear",
			"Enter keep · Esc clear",
		}
	case manager.query != "":
		return []string{
			"j/k choose · Space actions · Esc clear search",
			"Space actions · Esc clear",
		}
	default:
		return []string{
			"j/k choose · n new · r rename · d delete · Space actions · / search · Esc projects",
			"j/k · n new · r rename · d delete · Space · / · Esc",
			"n · r · d · Space · Esc",
		}
	}
}

func (manager *projectBoundaryManager) helpItems() []HelpItem {
	if manager.editor != nil {
		return []HelpItem{
			{Key: "enter", Description: "Save the boundary name", Category: "Actions"},
			{Key: "esc", Description: "Discard the name and return to the boundary list", Category: "Navigation"},
		}
	}
	return []HelpItem{
		{Key: "j/k", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+d/u", Description: "Move half a page down/up", Category: "Navigation"},
		{Key: "n", Description: "Create an empty boundary", Category: "Actions"},
		{Key: "r", Description: "Rename the selected custom boundary", Category: "Actions"},
		{Key: "d", Description: "Delete the selected custom boundary", Category: "Actions"},
		{Key: "space", Description: "Open boundary administration actions", Category: "Actions"},
		{Key: "/", Description: "Search boundaries", Category: "Filters"},
		{Key: "esc", Description: "Clear search, then return to the project list", Category: "Navigation"},
	}
}

// openBoundaryManager hands the Hub's whole key stream to the manager until it
// closes, so Esc unwinds to the project list instead of the Outliner.
func (m *ProjectHubModel) openBoundaryManager() {
	manager, err := newProjectBoundaryManager(m.runtime, m.registry, m.executor, m.width)
	if err != nil {
		m.notice = err.Error()
		return
	}
	m.notice = ""
	m.boundaries = manager
}

func (m *ProjectHubModel) closeBoundaryManager() {
	m.boundaries = nil
}

//nolint:gocyclo // Overlay precedence stays in one place so every manager state has a single unwind path.
func (m *ProjectHubModel) updateBoundaryManager(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	manager := m.boundaries
	if manager.confirmDiscard {
		switch key.String() {
		case "y", "Y":
			manager.closeEditor()
		case "n", "N", "esc":
			manager.confirmDiscard = false
			manager.discardOverlay.Cancel()
		}
		return m, nil
	}
	if confirmed, handled := manager.confirm.HandleKey(key.String()); handled {
		return m, m.resolveBoundaryDelete(confirmed)
	}
	if manager.destination != nil {
		m.updateBoundaryDestination(key)
		return m, nil
	}
	if manager.editor != nil {
		return m, m.updateBoundaryEditor(key)
	}
	if manager.actions != nil {
		return m, m.updateBoundaryActions(key)
	}
	if manager.searching {
		return m, m.updateBoundarySearch(key)
	}

	manager.notice = ""
	switch key.String() {
	case "esc":
		if manager.query != "" {
			manager.clearSearch()
			return m, nil
		}
		m.closeBoundaryManager()
	case "/":
		manager.startSearch()
		return m, textinput.Blink
	case "j", "down":
		manager.surface.MoveCursor(1)
	case "k", "up":
		manager.surface.MoveCursor(-1)
	case "g", "home":
		manager.surface.MoveCursorToStart()
	case "G", "end":
		manager.surface.MoveCursorToEnd()
	case "ctrl+d":
		manager.surface.MoveCursor(HalfPage(manager.surface.Viewport().Height))
	case "ctrl+u":
		manager.surface.MoveCursor(-HalfPage(manager.surface.Viewport().Height))
	case "n":
		return m, m.executeBoundaryCreate()
	case "r":
		return m, m.executeBoundaryBrowseAction(operationBoundaryRename)
	case "d":
		return m, m.executeBoundaryBrowseAction(operationBoundaryDelete)
	case " ", "space":
		m.openBoundaryActions()
	}
	return m, nil
}

func (m *ProjectHubModel) updateBoundarySearch(key tea.KeyPressMsg) tea.Cmd {
	manager := m.boundaries
	switch key.String() {
	case "esc":
		manager.clearSearch()
		return nil
	case "enter":
		manager.searching = false
		manager.searchInput.Blur()
		return nil
	case "up", "ctrl+k":
		manager.surface.MoveCursor(-1)
		return nil
	case "down", "ctrl+j":
		manager.surface.MoveCursor(1)
		return nil
	}
	var cmd tea.Cmd
	manager.searchInput, cmd = manager.searchInput.Update(key)
	if manager.searchInput.Value() != manager.query {
		manager.query = manager.searchInput.Value()
		manager.surface.SetQuery(manager.query)
	}
	return cmd
}

func (m *ProjectHubModel) updateBoundaryEditor(key tea.KeyPressMsg) tea.Cmd {
	manager := m.boundaries
	editor := manager.editor
	switch key.String() {
	case "esc":
		if editor.hasChanges() {
			manager.confirmDiscard = true
			return nil
		}
		manager.closeEditor()
		return nil
	case "enter", "ctrl+s":
		return m.saveBoundaryEditor()
	}
	var cmd tea.Cmd
	editor.input, cmd = editor.input.Update(key)
	editor.err = ""
	return cmd
}

// saveBoundaryEditor refuses an invalid name against the same catalog rules the
// commit enforces, so a rejection is reported in the prompt that produced it
// rather than as a status error over a closed editor.
func (m *ProjectHubModel) saveBoundaryEditor() tea.Cmd {
	manager := m.boundaries
	editor := manager.editor
	name := strings.TrimSpace(editor.input.Value())
	if editor.boundaryID != "" && name == editor.original {
		manager.closeEditor()
		return nil
	}
	if err := m.runtime.Catalog().ValidateBoundaryName(name, editor.boundaryID); err != nil {
		editor.err = err.Error()
		return nil
	}
	boundaryID := editor.boundaryID
	manager.closeEditor()
	if boundaryID == "" {
		return func() tea.Msg { return CreateBoundaryMsg{Name: name} }
	}
	return func() tea.Msg { return RenameBoundaryMsg{BoundaryID: boundaryID, Name: name} }
}

func (m *ProjectHubModel) updateBoundaryDestination(key tea.KeyPressMsg) {
	manager := m.boundaries
	option, chosen, dismissed := manager.destination.handleKey(key)
	if chosen {
		manager.destination = nil
		manager.pending.DestinationID = option.ID
		manager.pending.DestinationName = option.Label
		manager.armDeleteConfirmation()
		return
	}
	if dismissed {
		manager.destination = nil
		manager.pending = pendingBoundaryDelete{}
	}
}

func (m *ProjectHubModel) resolveBoundaryDelete(confirmed bool) tea.Cmd {
	manager := m.boundaries
	pending := manager.pending
	manager.pending = pendingBoundaryDelete{}
	if !confirmed || pending.BoundaryID == "" || pending.DestinationID == "" {
		return nil
	}
	return func() tea.Msg {
		return DeleteBoundaryMsg{
			BoundaryID:    pending.BoundaryID,
			DestinationID: pending.DestinationID,
		}
	}
}

func (m *ProjectHubModel) openBoundaryActions() {
	manager := m.boundaries
	target, ok := manager.selectedTarget()
	if !ok {
		return
	}
	sheet, err := NewContextActionSheetModel(
		manager.registry,
		manager.executor,
		m.projectHubCommandContext(""),
		manager.surface.ActionState().OperationIDs,
		ContextActionSheetConfig{
			Host:        ContextActionHostProjectHub,
			Completion:  ContextActionCompleteCloseSheet,
			Title:       "Boundary actions",
			Subtitle:    target.Name,
			Categories:  boundaryManagerActionCategories,
			CategoryFor: boundaryManagerActionCategory,
			SecondaryFor: func(descriptor OperationDescriptor) string {
				return string(descriptor.Risk)
			},
			DisabledFor: func(descriptor OperationDescriptor) string {
				return boundaryActionUnavailable(target.BoundaryID, descriptor.ID)
			},
			// The frozen row travels as the value every boundary operation reads,
			// so no later cursor move can retarget it.
			Value:        target,
			HasValue:     true,
			ItemIDPrefix: "boundary-manager-action:",
			EmptyState:   ListEmptyState{Title: "No actions available."},
			NoMatchState: ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		manager.notice = err.Error()
		return
	}
	if len(sheet.operationIDs()) == 0 {
		manager.notice = "No administration actions are available for this boundary"
		return
	}
	manager.actions = sheet
}

func boundaryManagerActionCategory(descriptor OperationDescriptor) ListCategoryID {
	if descriptor.Risk == OperationRiskDestructive {
		return boundaryManagerActionCategoryDanger
	}
	return boundaryManagerActionCategoryBoundary
}

func (m *ProjectHubModel) updateBoundaryActions(key tea.KeyPressMsg) tea.Cmd {
	manager := m.boundaries
	if !manager.actions.searching && (key.String() == " " || key.String() == "space") {
		key = tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	cmd, dismissed, message := manager.actions.handleKey(key)
	if dismissed {
		manager.actions = nil
	}
	switch typed := message.(type) {
	case ContextActionSheetClosedMsg:
		manager.actions = nil
		return cmd
	case ContextActionSheetResultMsg:
		return m.applyBoundaryExecutionResult(typed.Result)
	case StatusMsg:
		manager.notice = string(typed)
	}
	return cmd
}

// executeBoundaryCreate raises the one boundary operation with no row subject.
func (m *ProjectHubModel) executeBoundaryCreate() tea.Cmd {
	manager := m.boundaries
	if manager.executor == nil {
		manager.notice = "Boundary actions are unavailable"
		return nil
	}
	result := manager.executor.Execute(OperationExecutionRequest{
		ID:       operationBoundaryCreate,
		Context:  m.projectHubCommandContext(""),
		Value:    projectBoundaryActionTarget{},
		HasValue: true,
	})
	return m.applyBoundaryExecutionResult(result)
}

// executeBoundaryBrowseAction routes r and d through the centralized executor
// with the row identity captured before execution.
func (m *ProjectHubModel) executeBoundaryBrowseAction(id OperationID) tea.Cmd {
	manager := m.boundaries
	target, ok := manager.selectedTarget()
	if !ok {
		manager.notice = "No boundary is selected"
		return nil
	}
	if manager.executor == nil {
		manager.notice = "Boundary actions are unavailable"
		return nil
	}
	result := manager.surface.ExecuteAction(manager.executor, OperationExecutionRequest{
		ID:       id,
		Context:  m.projectHubCommandContext(""),
		Value:    target,
		HasValue: true,
	})
	if errors.Is(result.Err, ErrListActionUnavailable) {
		manager.notice = "That action is unavailable for this boundary"
		return nil
	}
	return m.applyBoundaryExecutionResult(result)
}

func (m *ProjectHubModel) applyBoundaryExecutionResult(result OperationExecutionResult) tea.Cmd {
	manager := m.boundaries
	if result.Err != nil {
		manager.notice = result.Err.Error()
		return nil
	}
	message := result.AsMessage()
	if inline, ok := message.(projectBoundaryInlineActionMsg); ok {
		m.applyBoundaryInlineAction(inline)
		return nil
	}
	if message == nil {
		return nil
	}
	return func() tea.Msg { return message }
}

func (m *ProjectHubModel) applyBoundaryInlineAction(msg projectBoundaryInlineActionMsg) {
	manager := m.boundaries
	if reason := boundaryActionUnavailable(msg.Target.BoundaryID, msg.OperationID); reason != "" {
		manager.notice = reason
		return
	}
	switch msg.OperationID { //nolint:exhaustive // Only the three boundary operations reach the manager.
	case operationBoundaryCreate:
		manager.openEditor("", "")
	case operationBoundaryRename:
		manager.focusBoundary(msg.Target.BoundaryID)
		manager.openEditor(msg.Target.BoundaryID, msg.Target.Name)
	case operationBoundaryDelete:
		manager.focusBoundary(msg.Target.BoundaryID)
		m.requestBoundaryDelete(msg.Target)
	}
	manager.resize(m.width)
}

// requestBoundaryDelete freezes the source and its member count, then asks for a
// destination only when there are members to move. An empty boundary reassigns
// nothing, so it goes straight to the confirmation with the task workspace
// recorded as the destination the catalog requires.
func (m *ProjectHubModel) requestBoundaryDelete(target projectBoundaryActionTarget) {
	manager := m.boundaries
	members := boundaryMemberIndex(m.runtime.Catalog())[target.BoundaryID]
	manager.pending = pendingBoundaryDelete{
		BoundaryID:   target.BoundaryID,
		BoundaryName: target.Name,
		Members:      members.Total,
	}
	if members.Total == 0 {
		manager.pending.DestinationID = models.ActiveBoundaryID
		manager.pending.DestinationName = config.ActiveBoundaryName
		manager.armDeleteConfirmation()
		return
	}
	manager.openDestinationPicker(target.BoundaryID)
	if manager.destination == nil {
		manager.pending = pendingBoundaryDelete{}
	}
}
