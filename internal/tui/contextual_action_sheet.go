package tui

import (
	"fmt"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ContextActionSheetHost names the surface that opened one contextual sheet.
// The engine never branches on it; it travels with every emitted message so a
// host can recognize its own sheet without the sheet importing host messages.
type ContextActionSheetHost string

const (
	ContextActionHostOutliner   ContextActionSheetHost = "outliner"
	ContextActionHostScopeHub   ContextActionSheetHost = "scope-hub"
	ContextActionHostProjectHub ContextActionSheetHost = "project-hub"
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
	Result OperationExecutionResult
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
	CategoryFor  func(OperationDescriptor) ListCategoryID
	SecondaryFor func(OperationDescriptor) string
	DisabledFor  func(OperationDescriptor) string
	// ImmediateFilter makes printable text update the plain list query without
	// requiring a search-mode key. Arrow keys remain navigation.
	ImmediateFilter bool
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
	surface   *ListSurfaceModel
	executor  *OperationExecutor
	context   CommandContext
	config    ContextActionSheetConfig
	notice    string
	searching bool
}

// NewContextActionSheetModel freezes ctx and projects operationIDs, in the
// order the host supplied, into one searchable sheet. Operations that are not
// registered or not applicable in the frozen context are dropped so a sheet
// only ever offers actions its host really has.
func NewContextActionSheetModel(
	registry *OperationRegistry,
	executor *OperationExecutor,
	ctx CommandContext,
	operationIDs []OperationID,
	config ContextActionSheetConfig,
) (*ContextActionSheetModel, error) {
	if registry == nil {
		return nil, fmt.Errorf("contextual action sheet requires an operation registry")
	}
	config = normalizeContextActionSheetConfig(config)
	frozen := cloneCommandContext(ctx)

	items := make([]ListItem, 0, len(operationIDs))
	seen := make(map[OperationID]bool, len(operationIDs))
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
		surface:  surface,
		executor: executor,
		context:  frozen,
		config:   config,
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
	}
	config.Categories = append([]ListCategory(nil), config.Categories...)
	return config
}

func contextActionSheetItem(
	descriptor OperationDescriptor,
	config ContextActionSheetConfig,
) ListItem {
	item := ListItem{
		ID:           ListItemID(config.ItemIDPrefix + string(descriptor.ID)),
		PrimaryText:  descriptor.Label,
		Aliases:      append([]string(nil), descriptor.Aliases...),
		OperationIDs: []OperationID{descriptor.ID},
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

// operationIDs returns the available actions in host-supplied order.
func (model *ContextActionSheetModel) operationIDs() []OperationID {
	if model == nil || model.surface == nil {
		return nil
	}
	items := model.surface.Items()
	ids := make([]OperationID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.OperationIDs...)
	}
	return ids
}

// request builds the frozen execution request for one operation. The cloned
// context makes later host cursor movement unable to retarget it.
func (model *ContextActionSheetModel) request(id OperationID) OperationExecutionRequest {
	return OperationExecutionRequest{
		ID:       id,
		Context:  cloneCommandContext(model.context),
		Value:    model.config.Value,
		HasValue: model.config.HasValue,
	}
}

// Execute runs one frozen request through the shared executor and reports
// whether the sheet is finished. A pending value or confirmation keeps the
// sheet open beneath its nested step, a failure keeps it open with the reason,
// and only a completed action dismisses it.
func (model *ContextActionSheetModel) Execute(
	request OperationExecutionRequest,
) (OperationExecutionResult, bool) {
	if model == nil {
		return OperationExecutionResult{}, false
	}
	if model.executor == nil {
		err := fmt.Errorf("contextual actions are unavailable")
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

// handleKey implements the shell's compact key-interceptor contract.
//
//nolint:gocyclo // Key dispatch stays centralized so every action-sheet state has one unwind path.
func (model *ContextActionSheetModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, ContextActionSheetClosedMsg{}
	}
	key := msg.String()
	if model.config.ImmediateFilter {
		switch key {
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				model.searching = false
				return nil, false, nil
			}
			return nil, true, ContextActionSheetClosedMsg{}
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			model.searching = model.surface.Query().Raw != ""
			return nil, false, nil
		case "/":
			model.searching = true
			return nil, false, nil
		case "up", "ctrl+k":
			model.surface.MoveCursor(-1)
			return nil, false, nil
		case "down", "ctrl+j":
			model.surface.MoveCursor(1)
			return nil, false, nil
		case "home":
			model.surface.MoveCursorToStart()
			return nil, false, nil
		case "end":
			model.surface.MoveCursorToEnd()
			return nil, false, nil
		case "pgup":
			model.surface.PageCursor(-1)
			return nil, false, nil
		case "pgdown":
			model.surface.PageCursor(1)
			return nil, false, nil
		case "enter":
			return model.activateCursor()
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
				model.searching = true
			}
			return nil, false, nil
		}
	}
	if model.searching {
		switch key {
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, ContextActionSheetClosedMsg{}
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
	}
	return nil, false, nil
}

func (model *ContextActionSheetModel) activateCursor() (tea.Cmd, bool, tea.Msg) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil, false, nil
	}
	if !item.Enabled() {
		model.notice = item.DisabledReason
		return nil, false, StatusMsg(item.DisabledReason)
	}
	descriptor, ok := item.Data.(OperationDescriptor)
	if !ok {
		model.notice = "Action is unavailable"
		return nil, false, StatusMsg("Action is unavailable")
	}
	request := model.request(descriptor.ID)
	result, dismissed := model.Execute(request)
	return nil, dismissed, ContextActionSheetResultMsg{
		Result: result,
	}
}

func (model *ContextActionSheetModel) render(width int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 48), max(width-6, 24))
	listHeight := min(max(len(model.surface.Rows()), 7), 16)
	query := model.surface.Query().Raw
	search := "Press / to search"
	if model.config.ImmediateFilter {
		search = "Type to filter"
	}
	if model.searching || query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	header := MutedStyle.Render(search)
	if model.notice != "" {
		header = ErrorStatusStyle.Render(truncateToVisualWidth(model.notice, max(innerWidth-4, 12))) +
			"\n" + header
	}
	content := header + "\n\n" + RenderCompactListSurface(model.surface, innerWidth-4, listHeight)

	titleText := model.config.Title
	if model.config.Bulk {
		titleText = fmt.Sprintf("%s · %d selected", titleText, model.config.Targets)
	}
	if model.config.Subtitle != "" {
		titleText += ": " + truncateToVisualWidth(model.config.Subtitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	footerText := "Enter choose · / search · Esc close"
	if model.config.ImmediateFilter {
		footerText = "Type filter · ↑↓/Ctrl+j/k move · Enter · Esc"
	}
	footer := HelpStyle.Render(footerText)
	return renderOverlayBox(content, innerWidth, title, footer, lipgloss.NewStyle().Foreground(ColorPrimary))
}
