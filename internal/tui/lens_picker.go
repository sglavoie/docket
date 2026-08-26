package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// The named-lens picker is the one surface the lens lifecycle is reachable
// from. It is a transient overlay rather than a routed view: it adds no
// PresentationID, owns no navigation frame, and dies with the overlay stack.
//
// Two rules shape everything below. Neither action row — Save current view or
// Add built-in view — is part of the filtered population, so a query that
// matches no saved lens still leaves both acts one Enter away; that is why this
// model filters the saved lenses itself and hands the surface an
// already-narrowed item list instead of setting the surface's own query. And
// every mutation goes back through the shared registry/executor pipeline, so the
// picker itself never writes machine state: Enter raises a request, the action
// sheet raises a request, and the nested prompt, gallery, and confirmation
// belong to the executor exactly as they do everywhere else.

const (
	lensPickerItemPrefix                   = "lens-picker:"
	lensPickerCreateItemID      ListItemID = lensPickerItemPrefix + "create"
	lensPickerAddTemplateItemID ListItemID = lensPickerItemPrefix + "add-template"
	lensPickerLensPrefix                   = lensPickerItemPrefix + "lens:"
)

// lensPickerCreateLabel and lensPickerAddTemplateLabel head every opening of the
// picker, including an empty one, in this order: saving what is on screen is the
// act the picker was opened from, and browsing the catalog is the fallback for a
// workspace that has nothing worth saving yet.
const (
	lensPickerCreateLabel           = "Save current view…"
	lensPickerAddTemplateLabel      = "Add built-in view…"
	lensPickerAddTemplateSecondary  = "Browse ready-made views"
	lensPickerCreateSpaceNotice     = "Press Enter to save the current view"
	lensPickerAddTemplateSpaceHint  = "Press Enter to browse built-in views"
	lensPickerCreateEnterLabel      = "save view"
	lensPickerAddTemplateEnterLabel = "browse built-ins"
)

// namedLensActionSheetOperations is the Space menu over one saved lens: the
// three management acts, in the order the product contract names them. Apply is
// absent because Enter already is it, which is the one-path rule the entity
// surfaces hold to.
var namedLensActionSheetOperations = []OperationID{
	operationLensRename,
	operationLensReplace,
	operationLensDelete,
}

// NamedLensPickerClosedMsg dismisses the picker without executing anything.
type NamedLensPickerClosedMsg struct{}

// NamedLensPickerResultMsg reports one execution attempt raised by the picker
// itself — applying a saved lens, or saving the current view.
type NamedLensPickerResultMsg struct {
	Result OperationExecutionResult
}

// NamedLensActionsRequestMsg asks the shell to open the management sheet over
// the picker for one saved lens. The identity travels rather than a pointer, so
// a later refresh cannot retarget the sheet.
type NamedLensActionsRequestMsg struct {
	Entity EntityIdentity
	Name   string
}

// NamedLensPickerSnapshot is everything the picker draws: the saved population,
// which of them owns the current projection, whether that projection has since
// diverged, and a summary of the view the Save row would capture.
type NamedLensPickerSnapshot struct {
	Lenses         config.NamedLensList
	ActiveID       string
	Modified       bool
	CurrentSummary string
}

// NamedLensPickerModel is the searchable projection of the saved lenses.
type NamedLensPickerModel struct {
	surface  *ListSurfaceModel
	executor *OperationExecutor
	// view is frozen at construction: the picker floats over the route that
	// opened it, and its own presence must not change the context it builds.
	view      PresentationID
	snapshot  NamedLensPickerSnapshot
	sorted    config.NamedLensList
	listed    int
	query     string
	searching bool
	notice    string
}

// NewNamedLensPickerModel builds the picker over one machine-state snapshot.
// The executor is the whole of its authority: it owns the registry the picker
// would otherwise hold a second reference to, so the picker names only the seam
// it actually dispatches through.
func NewNamedLensPickerModel(
	executor *OperationExecutor,
	view PresentationID,
	snapshot NamedLensPickerSnapshot,
) (*NamedLensPickerModel, error) {
	if executor == nil {
		return nil, fmt.Errorf("the lens picker requires an operation executor")
	}
	model := &NamedLensPickerModel{
		executor: executor,
		view:     view,
	}
	model.adopt(snapshot)
	surface, err := NewListSurfaceModel(model.items(), nil, ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
	})
	if err != nil {
		return nil, err
	}
	model.surface = surface
	model.selectActive()
	return model, nil
}

// Refresh reloads the population after a mutation. The surface preserves its
// cursor by stable ID, which is the whole of the retention contract: a rename
// keeps the same UUID and therefore the same row even after the alphabetical
// order changes around it, and a delete drops the anchor so the clamped cursor
// lands on the nearest surviving row.
func (model *NamedLensPickerModel) Refresh(snapshot NamedLensPickerSnapshot) error {
	if model == nil || model.surface == nil {
		return nil
	}
	model.adopt(snapshot)
	return model.surface.SetItems(model.items())
}

func (model *NamedLensPickerModel) adopt(snapshot NamedLensPickerSnapshot) {
	model.snapshot = NamedLensPickerSnapshot{
		Lenses:         snapshot.Lenses.Clone(),
		ActiveID:       snapshot.ActiveID,
		Modified:       snapshot.Modified,
		CurrentSummary: snapshot.CurrentSummary,
	}
	model.sorted = sortedNamedLenses(model.snapshot.Lenses)
}

// sortedNamedLenses is the display order: case-insensitive by name, then by the
// stable UUID so two lenses that differ only in case never swap between builds.
func sortedNamedLenses(lenses config.NamedLensList) config.NamedLensList {
	sorted := lenses.Clone()
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if folded := strings.EqualFold(left.Name, right.Name); !folded {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		return left.ID < right.ID
	})
	return sorted
}

// items renders both action rows plus every saved lens the query admits.
// Filtering happens here rather than on the surface so the two action rows are
// exempt from it and the saved rows keep their alphabetical order instead of a
// relevance order.
func (model *NamedLensPickerModel) items() []ListItem {
	actions := lensPickerActionRows(model.snapshot)
	items := make([]ListItem, 0, len(model.sorted)+len(actions))
	items = append(items, actions...)
	listed := 0
	for _, lens := range model.sorted {
		if !namedLensMatchesQuery(lens, model.query) {
			continue
		}
		listed++
		items = append(items, namedLensPickerItem(lens, model.snapshot))
	}
	model.listed = listed
	return items
}

// lensPickerActionRows is the query-exempt head of the list, in display order.
// Both rows carry no Data, which is what CurrentLens reads to tell an action row
// from a saved one; their identity travels as the stable ListItemID instead.
func lensPickerActionRows(snapshot NamedLensPickerSnapshot) []ListItem {
	return []ListItem{
		{
			ID:            lensPickerCreateItemID,
			PrimaryText:   lensPickerCreateLabel,
			Aliases:       []string{"save", "new lens", "create lens"},
			SecondaryText: snapshot.CurrentSummary,
			OperationIDs:  []OperationID{operationLensCreate},
		},
		{
			ID:          lensPickerAddTemplateItemID,
			PrimaryText: lensPickerAddTemplateLabel,
			// The aliases are the words someone reaches for before they know what
			// this product calls a lens, so the row answers to all three.
			Aliases:       []string{"template", "preset", "built-in view"},
			SecondaryText: lensPickerAddTemplateSecondary,
			OperationIDs:  []OperationID{operationLensAddTemplate},
		},
	}
}

func namedLensPickerItem(lens config.NamedLens, snapshot NamedLensPickerSnapshot) ListItem {
	marker := ""
	if lens.ID == snapshot.ActiveID {
		label := "●"
		if snapshot.Modified {
			label = "●*"
		}
		marker = MutedStyle.Render(label) + " "
	}
	item := ListItem{
		ID:            lensPickerItemID(lens.ID),
		PrimaryText:   lens.Name,
		PrimaryMarker: marker,
		SecondaryText: namedLensSummary(lens),
		OperationIDs:  append([]OperationID{operationLensApply}, namedLensActionSheetOperations...),
		Data:          lens,
	}
	// The accent is read from the definition every time a row is built, which is
	// why it belongs here rather than beside any one caller: initial
	// construction, a query change, and Refresh after rename, replace, create, or
	// delete all rebuild through items(). A neutral definition sets no style at
	// all, leaving the row the list's ordinary text.
	if style, ok := namedLensPrimaryStyle(lens); ok {
		item.PrimaryStyle = &style
	}
	return item
}

func lensPickerItemID(id string) ListItemID {
	return ListItemID(lensPickerLensPrefix + id)
}

// namedLensMatchesQuery scores a saved lens against the picker's own query. The
// summary participates so a search for "waiting" finds a lens whose facets say
// waiting even when its name does not.
func namedLensMatchesQuery(lens config.NamedLens, query string) bool {
	normalized := strings.Join(strings.Fields(query), " ")
	if normalized == "" {
		return true
	}
	return fuzzy.ScoreTerms(lens.Name+" "+namedLensSummary(lens), normalized) != 0
}

// selectActive parks the cursor on the live lens when there is one, so opening
// the picker inside a lens starts on the row that names it.
func (model *NamedLensPickerModel) selectActive() {
	if model.snapshot.ActiveID == "" {
		return
	}
	model.surface.SetCursorToID(lensPickerItemID(model.snapshot.ActiveID))
}

// SelectLens moves the cursor onto one saved lens by UUID.
func (model *NamedLensPickerModel) SelectLens(id string) bool {
	if model == nil || model.surface == nil {
		return false
	}
	return model.surface.SetCursorToID(lensPickerItemID(id))
}

// CurrentLens returns the saved lens under the cursor. The Save row is not one,
// which is what every caller distinguishing the two rows asks.
func (model *NamedLensPickerModel) CurrentLens() (config.NamedLens, bool) {
	if model == nil || model.surface == nil {
		return config.NamedLens{}, false
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return config.NamedLens{}, false
	}
	lens, ok := item.Data.(config.NamedLens)
	return lens, ok
}

// cursorIs reports whether one stable row owns the cursor. Only the two
// query-exempt action rows are ever asked, since a saved lens is identified by
// its record through CurrentLens instead.
func (model *NamedLensPickerModel) cursorIs(id ListItemID) bool {
	if model == nil || model.surface == nil {
		return false
	}
	item, ok := model.surface.CurrentItem()
	return ok && item.ID == id
}

// Notice returns the retained failure text, which stays on the picker so a
// refused save can be retried from the row that raised it.
func (model *NamedLensPickerModel) Notice() string {
	if model == nil {
		return ""
	}
	return model.notice
}

// commandContext freezes the capability and the row identity one request acts
// under. CapabilityLensPicker is granted only here, which is what keeps the five
// lens operations unreachable from every other surface.
func (model *NamedLensPickerModel) commandContext(entity EntityIdentity) CommandContext {
	ctx := PresentationCommandContext(model.view)
	ctx.Capabilities.Add(CapabilityLensPicker)
	ctx.CurrentEntity = entity
	return ctx
}

func (model *NamedLensPickerModel) request(
	id OperationID,
	entity EntityIdentity,
) OperationExecutionRequest {
	return OperationExecutionRequest{ID: id, Context: model.commandContext(entity)}
}

// Execute runs one request through the shared executor and reports whether the
// picker is finished. A pending value or confirmation keeps it open beneath the
// nested step, a failure keeps it open carrying the reason, and only a completed
// action dismisses it.
func (model *NamedLensPickerModel) Execute(
	request OperationExecutionRequest,
) (OperationExecutionResult, bool) {
	if model == nil {
		return OperationExecutionResult{}, false
	}
	if model.executor == nil {
		err := fmt.Errorf("lens actions are unavailable")
		model.notice = err.Error()
		return OperationExecutionResult{Err: err}, false
	}
	result := model.executor.Execute(request)
	switch {
	case result.Err != nil:
		model.notice = result.Err.Error()
		return result, false
	case result.ValueRequired != nil, result.ConfirmationRequired != nil:
		model.notice = ""
		return result, false
	default:
		model.notice = ""
		return result, true
	}
}

// setQuery re-narrows the saved rows. It is the picker's own filter, so the
// surface never sees a query and the Save row never leaves the list.
func (model *NamedLensPickerModel) setQuery(raw string) {
	if model.query == raw {
		return
	}
	model.query = raw
	_ = model.surface.SetItems(model.items())
}

// handleKey implements the shell's compact key-interceptor contract.
//
//nolint:gocyclo // Key dispatch stays centralized so every picker state has one unwind path.
func (model *NamedLensPickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, NamedLensPickerClosedMsg{}
	}
	key := msg.String()
	if model.searching {
		switch key {
		case "esc":
			// One Esc clears a typed query; the next leaves search. Closing the
			// picker is the third, and belongs to browse mode alone.
			if model.query != "" {
				model.setQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			if model.query != "" {
				_, size := utf8.DecodeLastRuneInString(model.query)
				model.setQuery(model.query[:len(model.query)-size])
			}
			return nil, false, nil
		case "up":
			model.surface.MoveCursor(-1)
			return nil, false, nil
		case "down":
			model.surface.MoveCursor(1)
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.setQuery(model.query + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, NamedLensPickerClosedMsg{}
	case "/":
		model.searching = true
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "enter":
		return model.activateCursor()
	case "space":
		return model.openActionsForCursor()
	}
	return nil, false, nil
}

// activateCursor is Enter: Save opens the name prompt and Add built-in view
// opens the gallery, both through the executor's own value step, and a lens row
// applies and closes. Neither action row names a target, because the record each
// one writes does not exist yet.
func (model *NamedLensPickerModel) activateCursor() (tea.Cmd, bool, tea.Msg) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil, false, nil
	}
	if action, isAction := lensPickerActionOperation(item.ID); isAction {
		result, dismissed := model.Execute(model.request(action, EntityIdentity{}))
		return nil, dismissed, NamedLensPickerResultMsg{Result: result}
	}
	lens, ok := item.Data.(config.NamedLens)
	if !ok {
		model.notice = "That row is not a saved lens"
		return nil, false, StatusMsg(model.notice)
	}
	result, dismissed := model.Execute(model.request(operationLensApply, LensEntity(lens.ID)))
	return nil, dismissed, NamedLensPickerResultMsg{Result: result}
}

// lensPickerActionOperation maps one query-exempt row onto the operation Enter
// dispatches from it. A saved-lens row answers false, which is what keeps the
// two act rows and the population rows on separate dispatch paths.
func lensPickerActionOperation(id ListItemID) (OperationID, bool) {
	switch id {
	case lensPickerCreateItemID:
		return operationLensCreate, true
	case lensPickerAddTemplateItemID:
		return operationLensAddTemplate, true
	default:
		return "", false
	}
}

// openActionsForCursor is Space: the management sheet nests above the picker
// rather than replacing it, so canceling anything inside it lands back here.
func (model *NamedLensPickerModel) openActionsForCursor() (tea.Cmd, bool, tea.Msg) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil, false, nil
	}
	lens, ok := item.Data.(config.NamedLens)
	if !ok {
		// Each action row writes a record that does not exist yet, so neither names
		// a target and neither has anything to rename, replace, or delete. The
		// notice names that row's own act rather than a shared refusal, so Space is
		// still a hint about where Enter goes.
		model.notice = lensPickerCreateSpaceNotice
		if item.ID == lensPickerAddTemplateItemID {
			model.notice = lensPickerAddTemplateSpaceHint
		}
		return nil, false, StatusMsg(model.notice)
	}
	model.notice = ""
	return nil, false, NamedLensActionsRequestMsg{
		Entity: LensEntity(lens.ID),
		Name:   lens.Name,
	}
}

// namedLensPickerInnerWidth is the overlay box width the picker claims out of
// the terminal.
func namedLensPickerInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (model *NamedLensPickerModel) render(width int) string {
	return model.renderWithin(width, 0)
}

// renderWithin budgets the list after reserving rows for the modal chrome, so a
// minimum-supported terminal drops list rows rather than losing the search line,
// the state line, or the footer.
func (model *NamedLensPickerModel) renderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := namedLensPickerInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if model.notice != "" {
		header = append(header, ErrorStatusStyle.Render(
			truncateToVisualWidth(model.notice, contentWidth),
		))
	}
	if model.searching || model.query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		header = append(header, MutedStyle.Render("Search: "+model.query+cursor))
	}
	if state := model.stateLine(); state != "" {
		header = append(header, MutedStyle.Render(truncateToVisualWidth(state, contentWidth)))
	}

	listHeight := min(max(len(model.surface.Rows()), 4), 14)
	if height > 0 {
		// Halo, frame, title, the header block's separating row, and the footer
		// consume nine rows before any header line of its own.
		reserved := 9 + len(header)
		listHeight = min(listHeight, max(height-reserved, 1))
	}
	content := RenderCompactListSurface(model.surface, contentWidth, listHeight)
	if len(header) > 0 {
		content = strings.Join(header, "\n") + "\n\n" + content
	}

	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Named Lenses")
	guidance := model.guidanceSpec()
	return renderOverlayBox(
		content,
		innerWidth,
		title,
		renderGuidanceText(guidance, max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(ColorPrimary),
	)
}

// stateLine names an empty population or a query that matched none of it. Both
// action rows are always listed, so the surface itself is never empty and cannot
// carry this copy on the list's behalf.
func (model *NamedLensPickerModel) stateLine() string {
	switch {
	case len(model.sorted) == 0:
		return "No saved lenses yet — save this view or add a built-in."
	case model.listed == 0:
		return "No matching lenses — Esc clears the search."
	default:
		return ""
	}
}

// guidanceSpec returns the footer spec alone: this picker always draws a footer,
// so there is no "no guidance" case for a caller to branch on.
func (model *NamedLensPickerModel) guidanceSpec() GuidanceSpec {
	if model == nil || model.surface == nil {
		return GuidanceSpec{}
	}
	searching := model.searching
	onLens := func() bool {
		_, ok := model.CurrentLens()
		return ok
	}
	// Browse-mode Enter is row-specific: the two action rows do different things
	// and the footer would lie if it named only one of them.
	enterLabel := "apply"
	switch {
	case searching:
		enterLabel = "apply search"
	case model.cursorIs(lensPickerCreateItemID):
		enterLabel = lensPickerCreateEnterLabel
	case model.cursorIs(lensPickerAddTemplateItemID):
		enterLabel = lensPickerAddTemplateEnterLabel
	}
	escapeLabel := "close"
	if searching {
		escapeLabel = "cancel search"
		if model.query != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return !searching }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("enter", "Apply the current row", "Actions",
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("space", "Open lens actions", "Actions",
			keymap.WithGate(func() bool { return !searching && onLens() }),
			keymap.WithFooter(3, "manage", false), keymap.WithFooterKey("Space")),
		keymap.Bind("/", "Search lenses", "Filters",
			keymap.WithGate(func() bool { return !searching }),
			keymap.WithFooter(20, "search", false)),
		keymap.Bind("esc", "Unwind the lens picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	spec, _ := listSurfaceGuidanceSpec(model.surface.GuidanceState(searching), km, nil)
	return spec
}

// namedLensSummary renders one saved definition as a single compact line: the
// mode it opens, the query it narrows with, the facets it actually sets, and the
// manual sort it restores. Facets left at their default are omitted, because a
// summary that names every axis names nothing.
func namedLensSummary(lens config.NamedLens) string {
	parts := make([]string, 0, 12)
	parts = append(parts, namedLensModeLabel(lens.Mode))
	if query := strings.TrimSpace(lens.Query); query != "" {
		parts = append(parts, "/"+query)
	}
	parts = append(parts, namedLensTaskFacets(lens.Tasks)...)
	parts = append(parts, namedLensProjectFacets(lens.Projects)...)
	if sorted := namedLensSortLabel(lens.Sort); sorted != "" {
		parts = append(parts, sorted)
	}
	return strings.Join(parts, " · ")
}

// namedLensModeLabel prefers the catalog label and degrades to the stored token,
// so a machine file written by a newer build still names something.
func namedLensModeLabel(token string) string {
	id, err := lensModeAxis.value(token)
	if err != nil {
		return token
	}
	if mode, ok := OutlinerModeByID(id); ok {
		return mode.Label
	}
	return token
}

func namedLensTaskFacets(tasks config.LensTaskFilters) []string {
	facets := make([]string, 0, 9)
	if tasks.StatesSet {
		// An explicit state filter that admits nothing is a real predicate, so it
		// is named rather than dropped as an absent axis would be.
		states := "none"
		if len(tasks.States) > 0 {
			states = strings.Join(tasks.States, ",")
		}
		facets = append(facets, "states: "+states)
	}
	if tasks.Priority > 0 {
		facets = append(facets, "!"+strconv.Itoa(tasks.Priority))
	}
	for _, tag := range tasks.Tags {
		facets = append(facets, "#"+tag)
	}
	if tasks.DueDate != "" && tasks.DueDate != config.LensDueAll {
		facets = append(facets, "due: "+tasks.DueDate)
	}
	if tasks.CompletionAgeSet {
		age := tasks.CompletionAge
		if age == "" {
			age = config.LensCompletionAgeAll
		}
		facets = append(facets, "done: "+age)
	}
	if tasks.Hierarchy != "" && tasks.Hierarchy != config.LensHierarchyAll {
		facets = append(facets, "tree: "+tasks.Hierarchy)
	}
	if tasks.Stale {
		facets = append(facets, "stale")
	}
	if tasks.Deferred {
		facets = append(facets, "deferred")
	}
	if tasks.Pinned {
		facets = append(facets, "pinned")
	}
	return facets
}

func namedLensProjectFacets(projects config.LensProjectFilters) []string {
	facets := make([]string, 0, len(projects.Tags)+3)
	for _, tag := range projects.Tags {
		facets = append(facets, "%"+tag)
	}
	if projects.Untagged {
		facets = append(facets, "%untagged")
	}
	if mode := projects.ParkedMode; mode != "" && mode != config.LensPopulationExclude {
		facets = append(facets, "parked: "+mode)
	}
	if mode := projects.ArchivedMode; mode != "" && mode != config.LensPopulationExclude {
		facets = append(facets, "archived: "+mode)
	}
	return facets
}

func namedLensSortLabel(sorted config.LensSort) string {
	field, err := lensSortAxis.value(sorted.Field)
	if err != nil {
		return "sort: " + sorted.Field
	}
	if field == SortByDefault {
		return ""
	}
	direction := "↓"
	if sorted.Ascending {
		direction = "↑"
	}
	return "sort: " + field.String() + " " + direction
}
