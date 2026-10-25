package listsurface

import (
	"fmt"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/keymap"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// ContextActionSheetHost names the surface that opened one contextual sheet.
// The engine never branches on it; it travels with every emitted message so a
// host can recognize its own sheet without the sheet importing host messages.
type ContextActionSheetHost string

const (
	ContextActionHostOutliner    ContextActionSheetHost = "outliner"
	ContextActionHostProjectHub  ContextActionSheetHost = "project-hub"
	ContextActionHostNotes       ContextActionSheetHost = "notes"
	ContextActionHostAttachments ContextActionSheetHost = "attachments"
	ContextActionHostComments    ContextActionSheetHost = "comments"
	ContextActionHostLensPicker  ContextActionSheetHost = "lens-picker"
)

// ContextActionSheetCompletion is the lifecycle applied after an action
// completes. Cancellation, a pending nested step, and failure never apply it.
type ContextActionSheetCompletion string

const (
	// ContextActionCompleteCloseSheet closes only the sheet and leaves the host
	// route exactly as it was.
	ContextActionCompleteCloseSheet ContextActionSheetCompletion = "close-sheet"
)

// ContextActionSheetClosedMsg dismisses a sheet without executing anything.
type ContextActionSheetClosedMsg struct{}

// ContextActionSheetResultMsg reports one centralized execution attempt and the
// lifecycle its host must apply.
type ContextActionSheetResultMsg struct {
	Result operation.OperationExecutionResult
}

// ContextActionSheetConfig is the complete host-supplied description of one
// contextual sheet. Row-based hosts pass the ListItem.OperationIDs intersection
// their ListSurface already computes and freeze their captured row target in
// Value; the Outliner passes its own ordered task projection instead.
type ContextActionSheetConfig struct {
	Host       ContextActionSheetHost
	Completion ContextActionSheetCompletion
	Title      string
	Subtitle   string
	// ItemIDPrefix namespaces row IDs so two hosts never collide.
	ItemIDPrefix string
	Categories   []ListCategory
	CategoryFor  func(operation.OperationDescriptor) ListCategoryID
	SecondaryFor func(operation.OperationDescriptor) string
	DisabledFor  func(operation.OperationDescriptor) string
	// Value is a frozen host payload delivered to the executor with every
	// request raised from this sheet.
	Value    any
	HasValue bool
	// Targets and Bulk describe the frozen target set for the header only.
	Targets      int
	Bulk         bool
	EmptyState   ListEmptyState
	NoMatchState ListEmptyState
}

// ContextActionSheetModel is the shared ListSurface projection of the
// operations available to one frozen target set. It knows nothing about tasks,
// Scope Hub or the Outliner: hosts supply identity, ordering, and decoration.
type ContextActionSheetModel struct {
	Surface   *ListSurfaceModel
	executor  *operation.OperationExecutor
	Context   operation.CommandContext
	Config    ContextActionSheetConfig
	Notice    string
	Searching bool
}

// NewContextActionSheetModel freezes ctx and projects operationIDs, in the
// order the host supplied, into one searchable sheet. Operations that are not
// registered or not applicable in the frozen context are dropped so a sheet
// only ever offers actions its host really has.
func NewContextActionSheetModel(
	registry *operation.OperationRegistry,
	executor *operation.OperationExecutor,
	ctx operation.CommandContext,
	operationIDs []operation.OperationID,
	config ContextActionSheetConfig,
) (*ContextActionSheetModel, error) {
	if registry == nil {
		return nil, fmt.Errorf("contextual action sheet requires an operation registry")
	}
	config = normalizeContextActionSheetConfig(config)
	frozen := operation.CloneCommandContext(ctx)

	items := make([]ListItem, 0, len(operationIDs))
	seen := make(map[operation.OperationID]bool, len(operationIDs))
	for _, id := range operationIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		descriptor, ok := registry.Descriptor(id)
		if !ok || !descriptor.Applicable(frozen) {
			continue
		}
		items = append(items, contextActionSheetItem(descriptor, config))
	}

	surface, err := NewListSurfaceModel(items, config.Categories, ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState:    config.EmptyState,
		NoMatchState:  config.NoMatchState,
	})
	if err != nil {
		return nil, err
	}
	return &ContextActionSheetModel{
		Surface:  surface,
		executor: executor,
		Context:  frozen,
		Config:   config,
	}, nil
}

func normalizeContextActionSheetConfig(config ContextActionSheetConfig) ContextActionSheetConfig {
	if config.Completion == "" {
		config.Completion = ContextActionCompleteCloseSheet
	}
	if config.ItemIDPrefix == "" {
		config.ItemIDPrefix = string(config.Host) + "-action:"
	}
	if config.Title == "" {
		config.Title = "Actions"
	}
	if config.EmptyState.Title == "" {
		config.EmptyState.Title = "No actions available."
	}
	if config.NoMatchState.Title == "" {
		config.NoMatchState.Title = "No matching actions."
		config.NoMatchState.Detail = "Clear or change the search."
	}
	config.Categories = append([]ListCategory(nil), config.Categories...)
	return config
}

func contextActionSheetItem(
	descriptor operation.OperationDescriptor,
	config ContextActionSheetConfig,
) ListItem {
	item := ListItem{
		ID:           ListItemID(config.ItemIDPrefix + string(descriptor.ID)),
		PrimaryText:  descriptor.Label,
		Aliases:      append([]string(nil), descriptor.Aliases...),
		OperationIDs: []operation.OperationID{descriptor.ID},
		Preview: ListPreview{
			Title: descriptor.Label,
			Fields: []ListPreviewField{
				{Label: "Risk", Value: string(descriptor.Risk)},
				{Label: "Value", Value: string(descriptor.ValueProviderID)},
			},
		},
		Data: descriptor,
	}
	if config.CategoryFor != nil {
		item.CategoryID = config.CategoryFor(descriptor)
	}
	if config.SecondaryFor != nil {
		item.SecondaryText = config.SecondaryFor(descriptor)
	}
	if config.DisabledFor != nil {
		item.DisabledReason = config.DisabledFor(descriptor)
	}
	return item
}

// OperationIDs returns the available actions in host-supplied order.
func (model *ContextActionSheetModel) OperationIDs() []operation.OperationID {
	if model == nil || model.Surface == nil {
		return nil
	}
	items := model.Surface.Items()
	ids := make([]operation.OperationID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.OperationIDs...)
	}
	return ids
}

// request builds the frozen execution request for one operation. The cloned
// context makes later host cursor movement unable to retarget it.
func (model *ContextActionSheetModel) request(id operation.OperationID) operation.OperationExecutionRequest {
	return operation.OperationExecutionRequest{
		ID:       id,
		Context:  operation.CloneCommandContext(model.Context),
		Value:    model.Config.Value,
		HasValue: model.Config.HasValue,
	}
}

// Execute runs one frozen request through the shared executor and reports
// whether the sheet is finished. A pending value or confirmation keeps the
// sheet open beneath its nested step, a failure keeps it open with the reason,
// and only a completed action dismisses it.
func (model *ContextActionSheetModel) Execute(
	request operation.OperationExecutionRequest,
) (operation.OperationExecutionResult, bool) {
	if model == nil {
		return operation.OperationExecutionResult{}, false
	}
	if model.executor == nil {
		err := fmt.Errorf("contextual actions are unavailable")
		model.Notice = err.Error()
		return operation.OperationExecutionResult{Err: err}, false
	}
	result := model.executor.Execute(request)
	switch {
	case result.Err != nil:
		model.Notice = result.Err.Error()
		return result, false
	case result.ValueRequired != nil, result.ConfirmationRequired != nil:
		model.Notice = ""
		return result, false
	default:
		model.Notice = ""
		return result, true
	}
}

// HandleKey implements the shell's compact key-interceptor contract.
//
//nolint:gocyclo // Key dispatch stays centralized so every action-sheet state has one unwind path.
func (model *ContextActionSheetModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.Surface == nil {
		return nil, true, ContextActionSheetClosedMsg{}
	}
	key := msg.String()
	if model.Searching {
		switch key {
		case "esc":
			if model.Surface.Query().Raw != "" {
				model.Surface.SetQuery("")
				return nil, false, nil
			}
			model.Searching = false
			return nil, false, nil
		case "enter":
			model.Searching = false
			return nil, false, nil
		case "backspace":
			query := model.Surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.Surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.Surface.SetQuery(model.Surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		// One layer per press. A query narrowing
		// the list survives the Enter that leaves search, so it is still standing
		// between the cursor and the full list, and it is what this press spends
		// itself on; the sheet closes once nothing is left to peel.
		if model.Surface.Query().Raw != "" {
			model.Surface.SetQuery("")
			return nil, false, nil
		}
		return nil, true, ContextActionSheetClosedMsg{}
	case "/":
		model.Searching = true
	case "up", "k", "ctrl+p":
		model.Surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.Surface.MoveCursor(1)
	case "home", "g":
		model.Surface.MoveCursorToStart()
	case "end", "G":
		model.Surface.MoveCursorToEnd()
	case "pgup":
		model.Surface.PageCursor(-1)
	case "pgdown":
		model.Surface.PageCursor(1)
	case "enter":
		return model.activateCursor()
	}
	return nil, false, nil
}

func (model *ContextActionSheetModel) activateCursor() (tea.Cmd, bool, tea.Msg) {
	item, ok := model.Surface.CurrentItem()
	if !ok {
		return nil, false, nil
	}
	if !item.Enabled() {
		model.Notice = item.DisabledReason
		return nil, false, status.StatusMsg(item.DisabledReason)
	}
	descriptor, ok := item.Data.(operation.OperationDescriptor)
	if !ok {
		model.Notice = "Action is unavailable"
		return nil, false, status.StatusMsg("Action is unavailable")
	}
	request := model.request(descriptor.ID)
	result, dismissed := model.Execute(request)
	return nil, dismissed, ContextActionSheetResultMsg{
		Result: result,
	}
}

func (model *ContextActionSheetModel) Render(width int) string {
	if model == nil || model.Surface == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 48), max(width-6, 24))
	listHeight := min(max(len(model.Surface.Rows()), 7), 16)
	query := model.Surface.Query().Raw
	search := ""
	if model.Searching || query != "" {
		cursor := ""
		if model.Searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	header := ""
	if search != "" {
		header = theme.MutedStyle.Render(search)
	}
	if model.Notice != "" {
		notice := theme.ErrorStatusStyle.Render(textfit.TruncateToVisualWidth(model.Notice, max(innerWidth-4, 12)))
		if header == "" {
			header = notice
		} else {
			header = notice + "\n" + header
		}
	}
	content := RenderCompactListSurface(model.Surface, innerWidth-4, listHeight)
	if header != "" {
		content = header + "\n\n" + content
	}

	titleText := model.Config.Title
	if model.Config.Bulk {
		titleText = fmt.Sprintf("%s · %d selected", titleText, model.Config.Targets)
	}
	titleText = chrome.FitOverlayTitle(titleText, model.Config.Subtitle, "", max(innerWidth-4, 1))
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(titleText)
	guidance, _ := model.GuidanceSpec()
	footer := chrome.RenderGuidanceText(guidance, max(innerWidth-4, 1))
	return chrome.RenderOverlayBox(content, innerWidth, title, footer, lipgloss.NewStyle().Foreground(theme.ColorPrimary))
}

// actionSheetSearchHintMinItems is the item count a sheet must exceed before its
// footer advertises `/ search`.
const actionSheetSearchHintMinItems = 5

func (model *ContextActionSheetModel) GuidanceSpec() (chrome.GuidanceSpec, bool) {
	if model == nil || model.Surface == nil {
		return chrome.GuidanceSpec{}, false
	}
	query := model.Surface.Query().Raw
	searching := model.Searching
	hasSelection := func() bool {
		_, ok := model.Surface.CurrentItem()
		return ok
	}
	enterLabel := "run"
	if searching {
		enterLabel = "apply search"
	}
	// The ladder is HandleKey's own, in HandleKey's order: a typed query is peeled
	// first in every sub-state, an open search box is left next, and the sheet
	// closes only when neither is standing.
	escapeLabel := "close"
	switch {
	case query != "":
		escapeLabel = "clear search"
	case searching:
		escapeLabel = "cancel search"
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return !searching && hasSelection() }),
			keymap.WithFooter(10, "navigate", false),
			keymap.WithFooterKey("j/k")),
		keymap.Bind("enter", "Apply current action", "Actions",
			keymap.WithGate(func() bool { return hasSelection() || searching }),
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		// Searching a sheet short enough to read whole earns no footer entry: the
		// key still answers, but a two-row sheet advertising `/ search` reads as
		// filler. The bound is where scanning starts to cost more than typing; a
		// standing query keeps the entry, since `/` is how it is edited.
		keymap.Bind("/", "Search actions", "Filters",
			keymap.WithGate(func() bool {
				return !searching && (query != "" || len(model.Surface.Items()) > actionSheetSearchHintMinItems)
			}),
			keymap.WithFooter(20, "search", false)),
		keymap.Bind("esc", "Unwind action sheet", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	return ListSurfaceGuidanceSpec(model.Surface.GuidanceState(searching), km, nil)
}
