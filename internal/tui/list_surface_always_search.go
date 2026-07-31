package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

// PrimeAlwaysSearch puts the picker into always-search mode without returning
// a Bubble Tea command. Use this in constructors when typing should work before
// Init() is called.
func (p *ListSurfaceAdapter[T]) PrimeAlwaysSearch() {
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
	picker    *ListSurfaceAdapter[T]
	allItems  []T
	upKeys    []string
	downKeys  []string
	pageSize  int
	maxCursor func(filtered []T) int
	cancelMsg func() tea.Msg
	selectMsg func(cursor int, filtered []T) tea.Msg
}

func newAlwaysSearchSinglePicker[T PickerItem](
	picker *ListSurfaceAdapter[T], cfg alwaysSearchSinglePickerConfig[T],
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
	case tea.PasteMsg:
		oldValue := p.picker.searchInput.Value()
		var cmd tea.Cmd
		p.picker.searchInput, cmd = p.picker.searchInput.Update(msg)
		if value := p.picker.searchInput.Value(); value != oldValue {
			p.picker.SetSearchQuery(value)
			p.fuzzyFilter(value)
		}
		return true, cmd
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
	p.picker.surface.MoveCursor(delta)
}

func (p *alwaysSearchSinglePicker[T]) fuzzyFilter(query string) {
	p.picker.Filter(query)
	if query == "" {
		p.picker.surface.MoveCursorToStart()
	}
}

func (p *alwaysSearchSinglePicker[T]) selectCurrent() tea.Cmd {
	filtered := p.picker.FilteredItems()
	cursor := p.picker.CursorIndex()
	if p.selectMsg == nil || cursor < 0 || cursor >= len(filtered) {
		return nil
	}
	return func() tea.Msg {
		return p.selectMsg(cursor, filtered)
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
	return slices.Contains(keys, key)
}
