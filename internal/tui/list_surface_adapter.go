package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// PickerItem is the typed payload contract used by specialized ListSurface
// adapters. Selection, ranking, cursor, and viewport behavior remain owned by
// ListSurfaceModel.
type PickerItem interface {
	FilterText() string
	DisplayText() string
	CountValue() int
	ItemID() string
}

// PinnablePickerItem optionally marks an adapted row as pinned.
type PinnablePickerItem interface {
	IsPinned() bool
}

// PickerConfig contains presentation choices for a specialized list adapter.
type PickerConfig struct {
	Title          string
	EmptyMessage   string
	NoMatchMessage string
	MultiSelect    bool
	ShowCounts     bool
}

// ListSurfaceAdapter supplies Bubble Tea search and rendering around the
// canonical headless ListSurfaceModel. It does not retain cursor, filtered, or
// selection state of its own.
type ListSurfaceAdapter[T PickerItem] struct {
	BaseSearchableView
	Config  PickerConfig
	surface *ListSurfaceModel
}

func NewListSurfaceAdapter[T PickerItem](items []T, config PickerConfig) *ListSurfaceAdapter[T] {
	mode := ListSelectionSingle
	if config.MultiSelect {
		mode = ListSelectionMulti
	}
	surface, err := ListSourceFromPickerItems(items, config.ShowCounts).NewModel(ListSurfaceConfig{
		SelectionMode: mode,
		EmptyState:    ListEmptyState{Title: config.EmptyMessage},
		NoMatchState:  ListEmptyState{Title: config.NoMatchMessage},
	})
	if err != nil {
		panic(fmt.Sprintf("invalid list adapter source: %v", err))
	}
	adapter := &ListSurfaceAdapter[T]{Config: config, surface: surface}
	adapter.InitSearchInput("Type to filter...", 50, 30)
	return adapter
}

func (adapter *ListSurfaceAdapter[T]) SetSize(width, height int) {
	adapter.BaseSearchableView.SetSize(width, height)
	adapter.surface.SetSize(width, height)
}

func (adapter *ListSurfaceAdapter[T]) Filter(query string) {
	adapter.surface.SetQuery(query)
}

func (adapter *ListSurfaceAdapter[T]) ReplaceItems(items []T) error {
	return adapter.surface.SetItems(ListSourceFromPickerItems(items, adapter.Config.ShowCounts).Items)
}

func (adapter *ListSurfaceAdapter[T]) SourceItems() []T {
	return pickerPayloads[T](adapter.surface.Items())
}

func (adapter *ListSurfaceAdapter[T]) FilteredItems() []T {
	matches := adapter.surface.Matches()
	items := make([]T, 0, len(matches))
	for _, match := range matches {
		if item, ok := match.Item.Data.(T); ok {
			items = append(items, item)
		}
	}
	return items
}

func pickerPayloads[T PickerItem](items []ListItem) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		if payload, ok := item.Data.(T); ok {
			result = append(result, payload)
		}
	}
	return result
}

func (adapter *ListSurfaceAdapter[T]) CursorIndex() int {
	return adapter.surface.Cursor()
}

func (adapter *ListSurfaceAdapter[T]) SetCursorIndex(index int) {
	items := adapter.FilteredItems()
	if index >= 0 && index < len(items) {
		adapter.SetCursorToID(items[index].ItemID())
	}
}

func (adapter *ListSurfaceAdapter[T]) SelectedCount() int {
	return len(adapter.surface.SelectedIDs())
}

func (adapter *ListSurfaceAdapter[T]) SelectionMap() map[string]bool {
	selected := make(map[string]bool)
	for _, id := range adapter.surface.SelectedIDs() {
		selected[string(id)] = true
	}
	return selected
}

func (adapter *ListSurfaceAdapter[T]) HandleKeyMsg(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if adapter.IsSearchMode() {
		handled, cmd, changed := adapter.HandleSearchInput(msg)
		if changed {
			adapter.Filter(adapter.SearchQuery())
		}
		return handled, cmd
	}

	switch msg.String() {
	case "j", "down":
		adapter.surface.MoveCursor(1)
		return true, nil
	case "k", "up":
		adapter.surface.MoveCursor(-1)
		return true, nil
	case "/":
		return true, adapter.ActivateSearch()
	case "space":
		if adapter.Config.MultiSelect {
			_, _ = adapter.surface.ToggleCurrentSelection()
			return true, nil
		}
	case "c":
		if adapter.Config.MultiSelect {
			adapter.surface.ClearSelection()
			return true, nil
		}
	}
	return false, nil
}

func (adapter *ListSurfaceAdapter[T]) SelectedItem() T {
	var zero T
	item, ok := adapter.surface.CurrentItem()
	if !ok {
		return zero
	}
	payload, ok := item.Data.(T)
	if !ok {
		return zero
	}
	return payload
}

func (adapter *ListSurfaceAdapter[T]) SelectedItems() []T {
	return pickerPayloads[T](adapter.surface.SelectedItems())
}

func (adapter *ListSurfaceAdapter[T]) SelectedIDs() []string {
	ids := adapter.surface.SelectedIDs()
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, string(id))
	}
	return result
}

func (adapter *ListSurfaceAdapter[T]) IsSelected(item T) bool {
	return adapter.surface.IsSelected(pickerItemListID(item))
}

func (adapter *ListSurfaceAdapter[T]) PreSelectByID(id string) {
	ids := adapter.surface.SelectedIDs()
	target := ListItemID(id)
	for _, selected := range ids {
		if selected == target {
			return
		}
	}
	ids = append(ids, target)
	_ = adapter.surface.SetSelectedIDs(ids)
}

func (adapter *ListSurfaceAdapter[T]) SetCursorToID(id string) {
	if id == "" {
		adapter.surface.SetCursorToID("picker:none")
		return
	}
	adapter.surface.SetCursorToID(ListItemID(id))
}

func (adapter *ListSurfaceAdapter[T]) RenderList(maxVisible int) string {
	if maxVisible < 5 {
		maxVisible = 5
	}
	width := adapter.Width()
	if width <= 0 {
		width = 80
	}
	rendered := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:    max(width-4, 0),
		Height:   maxVisible,
		Layout:   ListSurfaceLayoutCompact,
		HideHelp: true,
	}).Render(adapter.surface)
	return rendered + "\n"
}

func (adapter *ListSurfaceAdapter[T]) RenderSearchBar() string {
	if !adapter.IsSearchMode() {
		return ""
	}
	return "Filter: " + adapter.SearchInputView() + "\n\n"
}

func (adapter *ListSurfaceAdapter[T]) guidanceSpec(extraHints ...GuidanceHint) (GuidanceSpec, bool) {
	if adapter == nil || adapter.surface == nil {
		return GuidanceSpec{}, false
	}
	searching := adapter.IsSearchMode()
	hasItems := func() bool { return len(adapter.FilteredItems()) > 0 }
	hasSelections := func() bool { return adapter.SelectedCount() > 0 }
	browsing := func() bool { return !searching }
	enterLabel := "select"
	if adapter.Config.MultiSelect {
		enterLabel = "apply"
	}
	if searching {
		enterLabel = "apply search"
	}
	escapeLabel := "cancel"
	if searching {
		escapeLabel = "cancel search"
		if adapter.SearchQuery() != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return browsing() && hasItems() }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k")),
		keymap.Bind("space", "Toggle selection", "Actions",
			keymap.WithGate(func() bool { return browsing() && adapter.Config.MultiSelect && hasItems() }),
			keymap.WithFooter(3, "toggle", false), keymap.WithFooterKey("Space")),
		keymap.Bind("/", "Search", "Filters", keymap.WithGate(browsing),
			keymap.WithFooter(20, "search", false)),
		keymap.Bind("c", "Clear selection", "Actions",
			keymap.WithGate(func() bool { return browsing() && adapter.Config.MultiSelect && hasSelections() }),
			keymap.WithFooter(4, "clear selection", false)),
		keymap.Bind("enter", "Apply current state", "Actions",
			keymap.WithGate(func() bool { return searching || hasItems() }),
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	return listSurfaceGuidanceSpec(adapter.surface.GuidanceState(searching), km, extraHints)
}

func (adapter *ListSurfaceAdapter[T]) alwaysSearchGuidanceSpec(
	selectLabel string,
	extraHints ...GuidanceHint,
) (GuidanceSpec, bool) {
	if adapter == nil || adapter.surface == nil {
		return GuidanceSpec{}, false
	}
	count := len(adapter.FilteredItems())
	escapeLabel := "cancel"
	if adapter.SearchQuery() != "" {
		escapeLabel = "clear search"
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return count > 0 }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("↑/↓ or Ctrl+J/K")),
		keymap.Bind("enter", "Apply selection", "Actions",
			keymap.WithGate(func() bool { return count > 0 }),
			keymap.WithFooter(1, selectLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	spec, ok := listSurfaceGuidanceSpec(adapter.surface.GuidanceState(true), km, extraHints)
	if !ok {
		return GuidanceSpec{}, false
	}
	spec.Supplemental = fmt.Sprintf("%d %s", count, pluralize(count, "item", "items"))
	return spec, true
}

func (adapter *ListSurfaceAdapter[T]) RenderFooter() string {
	spec, ok := adapter.guidanceSpec()
	if !ok {
		return ""
	}
	return strings.Join(guidanceLines(spec, max(adapter.Width(), 1)), "\n")
}

func (adapter *ListSurfaceAdapter[T]) RenderInBox(titleContent, headerContent string, maxVisible int) string {
	var content strings.Builder
	title := renderFullscreenTitle(titleContent, headerContent)
	if adapter.Width() > 0 {
		if headerContent != "" && lipgloss.Width(title) > adapter.Width() {
			title = renderFullscreenTitle(titleContent, "")
		}
		title = fitStyledChromeToWidth("picker title", title, adapter.Width())
	}
	content.WriteString(title)
	content.WriteString("\n\n")
	content.WriteString(adapter.RenderSearchBar())
	if maxVisible == 0 {
		maxVisible = 15
		if adapter.Height() > 0 {
			maxVisible = adapter.Height() - 8
		}
	}
	content.WriteString(adapter.RenderList(maxVisible))
	guidance, _ := adapter.guidanceSpec()
	content.WriteString(renderGuidance(guidance, adapter.Width()))
	return content.String()
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
