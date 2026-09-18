package listsurface

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// ListItemID is the stable identity used for cursor and selection preservation.
type ListItemID string

// ListCategoryID is the stable identity of a list category. The type itself
// lives in presentation because the operation manifest names one below this
// layer; this alias keeps the list surface's own vocabulary unqualified.
type ListCategoryID = presentation.ListCategoryID

// ListSelectionMode controls whether a surface stores one or many selections.
type ListSelectionMode uint8

const (
	ListSelectionSingle ListSelectionMode = iota
	ListSelectionMulti
)

// ListEmptyKind distinguishes an empty source from a query with no matches.
type ListEmptyKind uint8

const (
	ListEmptySource ListEmptyKind = iota + 1
	ListEmptyNoMatches
)

// ListGuidanceProfile identifies the mutually exclusive interaction profile a
// list surface presents. Callers still own their keys and action registries;
// this shared state vocabulary keeps browse, search, selection, empty, and
// disabled guidance decisions consistent across renderers.
type ListGuidanceProfile uint8

const (
	ListGuidanceBrowse ListGuidanceProfile = iota
	ListGuidanceSearchEditing
	ListGuidanceSearchApplied
	ListGuidanceSelection
	ListGuidanceEmpty
	ListGuidanceDisabled
)

// ListRowKind distinguishes selectable item rows from category headers.
type ListRowKind uint8

const (
	ListRowItem ListRowKind = iota
	ListRowCategory
)

var (
	ErrListItemIDRequired       = errors.New("list item ID is required")
	ErrListItemDuplicateID      = errors.New("list item ID is duplicated")
	ErrListCategoryIDRequired   = errors.New("list category ID is required")
	ErrListCategoryDuplicateID  = errors.New("list category ID is duplicated")
	ErrListCategoryUnknown      = errors.New("list item references an unknown category")
	ErrListItemDisabled         = errors.New("list item is disabled")
	ErrListItemNotFound         = errors.New("list item was not found")
	ErrListSelectionMode        = errors.New("invalid list selection mode")
	ErrListSelectionCardinality = errors.New("single-selection list accepts at most one selection")
	ErrListActionUnavailable    = errors.New("list action is unavailable")
)

// ListPreviewField is one renderer-neutral label/value pair in an item preview.
type ListPreviewField struct {
	Label string
	Value string
}

// ListPreview is text-only preview content that compact and wide renderers can
// arrange without depending on a picker-specific model.
type ListPreview struct {
	Title   string
	Summary string
	Body    string
	Fields  []ListPreviewField
}

// Empty reports whether the preview has any displayable content.
func (preview ListPreview) Empty() bool {
	return preview.Title == "" &&
		preview.Summary == "" &&
		preview.Body == "" &&
		len(preview.Fields) == 0
}

// ListItem is the complete renderer-neutral representation of one row.
// Data is an adapter-owned payload; the behavior model never inspects it.
type ListItem struct {
	ID         ListItemID
	CategoryID ListCategoryID
	// GroupID is the section the row is drawn under when that differs from the
	// kind of value it is. It is presentation metadata only, which is the whole
	// point: CategoryID stays the row's identity, so a category jump, an
	// insertion order, and a vocabulary keep reading what the row is while the
	// surface draws it somewhere else. Empty means the row draws under its own
	// category, so the two agree by default.
	GroupID        ListCategoryID
	PrimaryText    string
	SecondaryText  string
	Aliases        []string
	DisabledReason string
	// Indeterminate renders the third state used by bulk membership editors.
	// It is presentation metadata only; adapters still own how toggling the row
	// changes the underlying value.
	Indeterminate bool
	// PrimaryStyle colors the primary text alone. Like Indeterminate it is
	// presentation metadata the behavior model never inspects: query, ranking, and
	// ordering keep reading the plain PrimaryText, which is what lets a row carry a
	// color without becoming unfindable or unsortable.
	PrimaryStyle *lipgloss.Style
	// PrimaryMarker is pre-styled chrome drawn immediately before the primary
	// text. Like PrimaryStyle it is presentation metadata the behavior model never
	// inspects, and for the same reason: query and ranking read the plain
	// PrimaryText. Unlike PrimaryStyle it survives the cursor row, because
	// SelectedLine strips styling but not characters.
	PrimaryMarker string
	Preview       ListPreview
	OperationIDs  []operation.OperationID
	Data          any
}

// Enabled reports whether the item may be selected or activated.
func (item ListItem) Enabled() bool {
	return strings.TrimSpace(item.DisabledReason) == ""
}

// GroupOrCategory reports the section the row is drawn under. It is the one
// answer bucketing, headers, and row categories read, so a promoted row can
// never be grouped in one place and headed in another.
func (item ListItem) GroupOrCategory() ListCategoryID {
	if item.GroupID != "" {
		return item.GroupID
	}
	return item.CategoryID
}

// ListCategory describes one stable group. Slice order supplied to the model
// is the category display order.
type ListCategory struct {
	ID    ListCategoryID
	Label string
}

// ListEmptyState is renderer-neutral copy for an empty surface.
type ListEmptyState struct {
	Title  string
	Detail string
}

// ListSurfaceConfig contains behavior that varies between list callers.
type ListSurfaceConfig struct {
	SelectionMode ListSelectionMode
	EmptyState    ListEmptyState
	NoMatchState  ListEmptyState
}

// ListQuery retains both the caller's input and its whitespace-normalized fuzzy
// form. Terms are exposed for renderers that display query diagnostics.
type ListQuery struct {
	Raw        string
	Normalized string
	Terms      []string
}

// NewListQuery builds the canonical fuzzy query used by ListSurfaceModel.
func NewListQuery(raw string) ListQuery {
	terms := strings.Fields(raw)
	return ListQuery{
		Raw:        raw,
		Normalized: strings.Join(terms, " "),
		Terms:      append([]string(nil), terms...),
	}
}

// ListMatch exposes the fuzzy score while retaining stable source identity.
type ListMatch struct {
	Item ListItem
}

// ListSurfaceRow is one headless row. Category rows are never cursor targets.
type ListSurfaceRow struct {
	Kind     ListRowKind
	Category ListCategory
	Match    ListMatch
	Cursor   bool
	Selected bool
}

// ListViewport is a snapshot of the bounded list window.
type ListViewport struct {
	Width         int
	Height        int
	Offset        int
	TotalRows     int
	CanScrollUp   bool
	CanScrollDown bool
}

// ListEmptyContext pairs the configured copy with the reason it is displayed.
type ListEmptyContext struct {
	Kind  ListEmptyKind
	State ListEmptyState
}

// ListGuidanceState carries profile-specific supplemental context without
// embedding presentation strings or key choices in the headless list model.
type ListGuidanceState struct {
	Profile        ListGuidanceProfile
	Query          string
	SelectionCount int
	Empty          ListEmptyContext
	DisabledReason string
}

// ListActionState describes the operations common to the active item set.
// In multi-select mode explicit selections win; otherwise the cursor item is
// the target. Adapters remain responsible for mapping item IDs into the
// CommandContext identities consumed by OperationExecutor.
type ListActionState struct {
	ItemIDs        []ListItemID
	OperationIDs   []operation.OperationID
	DisabledReason string
}

// Enabled reports whether the active item set can offer contextual actions.
func (state ListActionState) Enabled() bool {
	return len(state.ItemIDs) > 0 && state.DisabledReason == ""
}

// Descriptors resolves available action metadata without invoking handlers.
func (state ListActionState) Descriptors(
	registry *operation.OperationRegistry,
	ctx operation.CommandContext,
) ([]operation.OperationDescriptor, error) {
	if registry == nil {
		return nil, fmt.Errorf("list action state requires an operation registry")
	}
	if !state.Enabled() {
		return []operation.OperationDescriptor{}, nil
	}

	descriptors := make([]operation.OperationDescriptor, 0, len(state.OperationIDs))
	for _, id := range state.OperationIDs {
		descriptor, ok := registry.Descriptor(id)
		if !ok {
			return nil, fmt.Errorf("%w: operation %s is not registered", ErrListActionUnavailable, id)
		}
		if descriptor.Applicable(ctx) {
			descriptors = append(descriptors, descriptor)
		}
	}
	return descriptors, nil
}

// ListTextLayout is a visual-width-safe primary/secondary row layout.
type ListTextLayout struct {
	Primary   string
	Secondary string
	Gap       string
}

// Text returns the fitted inline text.
func (layout ListTextLayout) Text() string {
	if layout.Secondary == "" {
		return layout.Primary
	}
	return layout.Primary + layout.Gap + layout.Secondary
}

type listSurfaceMatch struct {
	item        ListItem
	score       int
	sourceIndex int
}
