package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// PickerItem defines the interface that any picker item must implement.
type PickerItem interface {
	// FilterText returns the text used for case-insensitive filtering.
	FilterText() string
	// DisplayText returns the primary text to display for this item.
	DisplayText() string
	// CountValue returns an optional count to display (0 if not applicable).
	CountValue() int
	// ItemID returns a unique identifier for selection tracking.
	ItemID() string
}

// PinnablePickerItem is an optional interface for items that can be pinned.
// Items implementing this interface will be rendered with pinned styling.
type PinnablePickerItem interface {
	IsPinned() bool
}

// PickerConfig holds customizable settings for a picker.
type PickerConfig struct {
	Title          string // Title displayed at the top
	Subtitle       string // Optional subtitle (e.g., current selection info)
	EmptyMessage   string // Message when no items exist
	NoMatchMessage string // Message when filter matches nothing
	MultiSelect    bool   // Whether multiple items can be selected
	ShowCounts     bool   // Whether to show count values
	BoxWidth       int    // Width of the picker box (0 for default)
}

// BasePicker provides shared functionality for all picker components.
// It handles search, filtering, cursor navigation, and rendering.
type BasePicker[T PickerItem] struct {
	BaseSearchableView
	Config   PickerConfig
	Items    []T
	Filtered []T
	Selected map[string]bool // Only used when MultiSelect=true
	Cursor   int
}

// NewBasePicker creates a new BasePicker with the given items and config.
func NewBasePicker[T PickerItem](items []T, config PickerConfig) *BasePicker[T] {
	p := &BasePicker[T]{
		Config:   config,
		Items:    items,
		Filtered: items,
		Selected: make(map[string]bool),
		Cursor:   0,
	}
	p.InitSearchInput("Type to filter...", 50, 30)
	return p
}

// PrimeAlwaysSearch puts the picker into always-search mode without returning
// a Bubble Tea command. Use this in constructors when typing should work before
// Init() is called.
func (p *BasePicker[T]) PrimeAlwaysSearch() {
	p.SetSearchActive(true)
	p.searchInput.Focus()
}

type alwaysSearchSinglePickerConfig[T PickerItem] struct {
	AllItems  []T
	UpKeys    []string
	DownKeys  []string
	PageSize  int
	MaxCursor func(filtered []T) int
	CancelMsg func() tea.Msg
	SelectMsg func(cursor int, filtered []T) tea.Msg
}

type alwaysSearchSinglePicker[T PickerItem] struct {
	picker    *BasePicker[T]
	allItems  []T
	upKeys    []string
	downKeys  []string
	pageSize  int
	maxCursor func(filtered []T) int
	cancelMsg func() tea.Msg
	selectMsg func(cursor int, filtered []T) tea.Msg
}

func newAlwaysSearchSinglePicker[T PickerItem](
	picker *BasePicker[T], cfg alwaysSearchSinglePickerConfig[T],
) *alwaysSearchSinglePicker[T] {
	if len(cfg.UpKeys) == 0 {
		cfg.UpKeys = []string{"up", "ctrl+k"}
	}
	if len(cfg.DownKeys) == 0 {
		cfg.DownKeys = []string{"down", "ctrl+j"}
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 10
	}
	if cfg.MaxCursor == nil {
		cfg.MaxCursor = func(filtered []T) int {
			return max(len(filtered)-1, 0)
		}
	}

	return &alwaysSearchSinglePicker[T]{
		picker:    picker,
		allItems:  cfg.AllItems,
		upKeys:    cfg.UpKeys,
		downKeys:  cfg.DownKeys,
		pageSize:  cfg.PageSize,
		maxCursor: cfg.MaxCursor,
		cancelMsg: cfg.CancelMsg,
		selectMsg: cfg.SelectMsg,
	}
}

func (p *alwaysSearchSinglePicker[T]) Init() tea.Cmd {
	return p.picker.ActivateSearch()
}

func (p *alwaysSearchSinglePicker[T]) Update(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.picker.SetSize(msg.Width, msg.Height)
		return true, nil
	case tea.KeyPressMsg:
		return true, p.handleKey(msg)
	default:
		return false, nil
	}
}

func (p *alwaysSearchSinglePicker[T]) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()

	switch {
	case key == "esc":
		if p.picker.IsSearchMode() && p.picker.SearchQuery() != "" {
			p.picker.ClearSearch()
			p.fuzzyFilter("")
			return nil
		}
		return p.cancel()
	case key == "enter":
		return p.selectCurrent()
	case keyMatches(key, p.upKeys):
		p.moveCursor(-1)
		return nil
	case keyMatches(key, p.downKeys):
		p.moveCursor(1)
		return nil
	case key == "ctrl+u":
		p.moveCursor(-p.pageSize)
		return nil
	case key == "ctrl+d":
		p.moveCursor(p.pageSize)
		return nil
	default:
		handled, cmd, valueChanged := p.picker.HandleSearchInput(msg)
		if valueChanged {
			p.fuzzyFilter(p.picker.SearchQuery())
		}
		if handled {
			return cmd
		}
		return nil
	}
}

func (p *alwaysSearchSinglePicker[T]) moveCursor(delta int) {
	maxCursor := p.maxCursor(p.picker.Filtered)
	if maxCursor < 0 {
		maxCursor = 0
	}

	p.picker.Cursor += delta
	if p.picker.Cursor < 0 {
		p.picker.Cursor = 0
	}
	if p.picker.Cursor > maxCursor {
		p.picker.Cursor = maxCursor
	}
}

func (p *alwaysSearchSinglePicker[T]) fuzzyFilter(query string) {
	query = strings.TrimSpace(query)

	if query == "" {
		p.picker.Filtered = p.allItems
		p.picker.Cursor = 0
		return
	}

	type scoredItem struct {
		item  T
		score int
	}

	var scored []scoredItem
	for _, item := range p.allItems {
		score := fuzzy.ScoreTerms(item.FilterText(), query)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	p.picker.Filtered = make([]T, len(scored))
	for i, item := range scored {
		p.picker.Filtered[i] = item.item
	}

	p.moveCursor(0)
}

func (p *alwaysSearchSinglePicker[T]) selectCurrent() tea.Cmd {
	if p.selectMsg == nil {
		return nil
	}
	return func() tea.Msg {
		return p.selectMsg(p.picker.Cursor, p.picker.Filtered)
	}
}

func (p *alwaysSearchSinglePicker[T]) cancel() tea.Cmd {
	if p.cancelMsg == nil {
		return nil
	}
	return func() tea.Msg {
		return p.cancelMsg()
	}
}

func keyMatches(key string, keys []string) bool {
	for _, candidate := range keys {
		if key == candidate {
			return true
		}
	}
	return false
}

// Filter updates the filtered list based on the search query.
func (p *BasePicker[T]) Filter(query string) {
	if query == "" {
		p.Filtered = p.Items
	} else {
		queryLower := strings.ToLower(query)
		var filtered []T
		for _, item := range p.Items {
			if strings.Contains(strings.ToLower(item.FilterText()), queryLower) {
				filtered = append(filtered, item)
			}
		}
		p.Filtered = filtered
	}
	if p.Cursor >= len(p.Filtered) {
		p.Cursor = 0
	}
}

// HandleKeyMsg processes keyboard input for navigation and selection.
// Returns (handled, cmd) where handled indicates if the key was consumed.
func (p *BasePicker[T]) HandleKeyMsg(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// Handle search mode first
	if p.IsSearchMode() {
		handled, cmd, valueChanged := p.HandleSearchInput(msg)
		if valueChanged {
			p.Filter(p.SearchQuery())
		}
		if handled {
			return true, cmd
		}
		return false, cmd
	}

	// Navigation and selection
	switch msg.String() {
	case "j", "down":
		p.Cursor = WrapCursor(p.Cursor, len(p.Filtered), 1)
		return true, nil

	case "k", "up":
		p.Cursor = WrapCursor(p.Cursor, len(p.Filtered), -1)
		return true, nil

	case "/":
		return true, p.ActivateSearch()

	case "space":
		if p.Config.MultiSelect && len(p.Filtered) > 0 && p.Cursor < len(p.Filtered) {
			item := p.Filtered[p.Cursor]
			id := item.ItemID()
			if p.Selected[id] {
				delete(p.Selected, id)
			} else {
				p.Selected[id] = true
			}
			return true, nil
		}
		return false, nil

	case "c":
		if p.Config.MultiSelect {
			p.Selected = make(map[string]bool)
			return true, nil
		}
		return false, nil
	}

	return false, nil
}

// SelectedItem returns the currently highlighted item (for single-select pickers).
// Returns the zero value if no items are available.
func (p *BasePicker[T]) SelectedItem() T {
	var zero T
	if len(p.Filtered) == 0 || p.Cursor >= len(p.Filtered) {
		return zero
	}
	return p.Filtered[p.Cursor]
}

// SelectedItems returns all selected items (for multi-select pickers).
func (p *BasePicker[T]) SelectedItems() []T {
	var result []T
	for _, item := range p.Items {
		if p.Selected[item.ItemID()] {
			result = append(result, item)
		}
	}
	return result
}

// SelectedIDs returns the IDs of all selected items.
func (p *BasePicker[T]) SelectedIDs() []string {
	var ids []string
	for id := range p.Selected {
		ids = append(ids, id)
	}
	return ids
}

// IsSelected returns whether an item is currently selected.
func (p *BasePicker[T]) IsSelected(item T) bool {
	return p.Selected[item.ItemID()]
}

// PreSelectByID marks an item as selected by its ID.
func (p *BasePicker[T]) PreSelectByID(id string) {
	p.Selected[id] = true
}

// SetCursorToID moves the cursor to the item with the given ID.
func (p *BasePicker[T]) SetCursorToID(id string) {
	for i, item := range p.Filtered {
		if item.ItemID() == id {
			p.Cursor = i
			return
		}
	}
}

// RenderList renders the scrollable item list with cursor and selection indicators.
func (p *BasePicker[T]) RenderList(maxVisible int) string {
	if maxVisible < 5 {
		maxVisible = 5
	}

	var b strings.Builder

	if len(p.Filtered) == 0 {
		if len(p.Items) == 0 {
			b.WriteString(MutedStyle.Render(p.Config.EmptyMessage))
		} else {
			b.WriteString(MutedStyle.Render(p.Config.NoMatchMessage))
		}
		b.WriteString("\n")
		return b.String()
	}

	// Calculate scroll window
	start := 0
	if p.Cursor >= maxVisible {
		start = p.Cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(p.Filtered))

	// Determine width for selection row
	rowWidth := p.Width()
	if rowWidth <= 0 {
		rowWidth = 80
	}
	rowWidth -= 4 // Account for padding

	for i := start; i < end; i++ {
		item := p.Filtered[i]
		selected := i == p.Cursor

		// Check if item is pinned (optional interface)
		isPinned := false
		if pinnable, ok := any(item).(PinnablePickerItem); ok {
			isPinned = pinnable.IsPinned()
		}

		// Checkbox for multi-select
		checkbox := ""
		if p.Config.MultiSelect {
			if p.Selected[item.ItemID()] {
				checkbox = SelectionCheckStyle.Render("[x]") + " "
			} else {
				checkbox = MutedStyle.Render("[ ]") + " "
			}
		}

		// Display text with optional pinned styling
		displayText := item.DisplayText()
		if isPinned {
			if selected {
				displayText = SuccessBoldStyle.Render(displayText)
			} else {
				displayText = PinnedProjectStyle.Render(displayText)
			}
		}

		// Count (optional)
		countStr := ""
		if p.Config.ShowCounts {
			if cd, ok := any(item).(CountDisplayer); ok {
				countStr = " " + cd.CountDisplayText()
			} else {
				countStr = TagManagerCountStyle.Render(fmt.Sprintf(" (%d)", item.CountValue()))
			}
		}

		// Build content without cursor
		content := checkbox + displayText + countStr

		// Apply selection styling
		line := RenderSelectableRow(selected, false, false, false, false, false, content, rowWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(p.Filtered) > maxVisible {
		remaining := len(p.Filtered) - end
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderSearchBar renders the search input when active.
func (p *BasePicker[T]) RenderSearchBar() string {
	if !p.IsSearchMode() {
		return ""
	}
	return "Filter: " + p.SearchInputView() + "\n\n"
}

// RenderFooter renders context-appropriate keyboard hints.
func (p *BasePicker[T]) RenderFooter() string {
	if p.IsSearchMode() {
		return MutedStyle.Render("enter done · esc clear")
	}

	if len(p.Filtered) == 0 {
		return MutedStyle.Render("esc cancel")
	}

	if p.Config.MultiSelect {
		return MutedStyle.Render("space toggle · / search · c clear · enter apply · esc cancel")
	}

	return MutedStyle.Render("enter select · / search · esc cancel")
}

// RenderInBox renders the complete picker UI as a fullscreen view with title and footer.
func (p *BasePicker[T]) RenderInBox(titleContent, headerContent string, maxVisible int) string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle(titleContent, headerContent))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(p.RenderSearchBar())

	// List
	if maxVisible == 0 {
		maxVisible = 15
		if p.Height() > 0 {
			maxVisible = p.Height() - 8
		}
	}
	b.WriteString(p.RenderList(maxVisible))

	// Footer
	left := HelpStyle.Render(p.RenderFooter())
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(p.Filtered)))
	b.WriteString(renderStatusBarFooter(left, right, p.Width()))

	return b.String()
}

// pluralize returns singular if count is 1, plural otherwise.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
