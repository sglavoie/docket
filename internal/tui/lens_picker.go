package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/config"
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
//
// The row construction lives in lens_picker_items.go, key dispatch in
// lens_picker_keys.go, and the overlay chrome in lens_picker_render.go.

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

// The quick-apply ordinals number the listed saved lenses, one digit each, so
// `\` then a digit is the whole route to a lens. Nine is the ceiling because
// there is no tenth digit key, and the number column is a fixed two columns on
// every row — numbered or not — so the variable-width active-lens dot behind it
// cannot bend it.
const (
	lensPickerMaxOrdinal         = 9
	lensPickerOrdinalMarkerWidth = 2
	// The refusal for an empty population is a constant rather than a format,
	// because "0 lenses are numbered" would be arithmetic where the picker
	// already has an empty-state line saying what to do instead.
	lensPickerOrdinalEmptyRefusal = "No lenses are listed to number"
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
	view     PresentationID
	snapshot NamedLensPickerSnapshot
	sorted   config.NamedLensList
	listed   int
	// ordinals is the quick-apply assignment, saved lens IDs in listed order and
	// capped at nine. It is rebuilt by items() in the same pass that counts
	// listed, which is what keeps a printed digit and the row under it from ever
	// naming different lenses.
	ordinals  []string
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
// under. CapabilityLensPicker is granted here and, for Create alone, by
// AppModel.saveViewAsLens; every operation that needs a record under the cursor
// is reachable only through this one, because nothing else supplies the entity.
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
