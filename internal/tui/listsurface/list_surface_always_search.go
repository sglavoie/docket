package listsurface

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

type AlwaysSearchSinglePickerConfig[T PickerItem] struct {
	UpKeys    []string
	DownKeys  []string
	PageSize  int
	CancelMsg func() tea.Msg
	SelectMsg func(cursor int, filtered []T) tea.Msg
}

type AlwaysSearchSinglePicker[T PickerItem] struct {
	picker    *ListSurfaceAdapter[T]
	upKeys    []string
	downKeys  []string
	pageSize  int
	cancelMsg func() tea.Msg
	selectMsg func(cursor int, filtered []T) tea.Msg
}

func NewAlwaysSearchSinglePicker[T PickerItem](
	picker *ListSurfaceAdapter[T], cfg AlwaysSearchSinglePickerConfig[T],
) *AlwaysSearchSinglePicker[T] {
	if len(cfg.UpKeys) == 0 {
		cfg.UpKeys = []string{"up", "ctrl+k"}
	}
	if len(cfg.DownKeys) == 0 {
		cfg.DownKeys = []string{"down", "ctrl+j"}
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 10
	}
	return &AlwaysSearchSinglePicker[T]{
		picker:    picker,
		upKeys:    cfg.UpKeys,
		downKeys:  cfg.DownKeys,
		pageSize:  cfg.PageSize,
		cancelMsg: cfg.CancelMsg,
		selectMsg: cfg.SelectMsg,
	}
}

func (p *AlwaysSearchSinglePicker[T]) Init() tea.Cmd {
	return p.picker.ActivateSearch()
}

func (p *AlwaysSearchSinglePicker[T]) Update(msg tea.Msg) (bool, tea.Cmd) {
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
			p.FuzzyFilter(value)
		}
		return true, cmd
	case tea.KeyPressMsg:
		return true, p.handleKey(msg)
	default:
		return false, nil
	}
}

func (p *AlwaysSearchSinglePicker[T]) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()

	switch {
	case key == "esc":
		if p.picker.IsSearchMode() && p.picker.SearchQuery() != "" {
			p.picker.ClearSearch()
			p.FuzzyFilter("")
			return nil
		}
		return p.Cancel()
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
			p.FuzzyFilter(p.picker.SearchQuery())
		}
		if handled {
			return cmd
		}
		return nil
	}
}

func (p *AlwaysSearchSinglePicker[T]) moveCursor(delta int) {
	p.picker.Surface.MoveCursor(delta)
}

func (p *AlwaysSearchSinglePicker[T]) FuzzyFilter(query string) {
	p.picker.Filter(query)
	if query == "" {
		p.picker.Surface.MoveCursorToStart()
	}
}

func (p *AlwaysSearchSinglePicker[T]) selectCurrent() tea.Cmd {
	filtered := p.picker.FilteredItems()
	cursor := p.picker.CursorIndex()
	if p.selectMsg == nil || cursor < 0 || cursor >= len(filtered) {
		return nil
	}
	return func() tea.Msg {
		return p.selectMsg(cursor, filtered)
	}
}

func (p *AlwaysSearchSinglePicker[T]) Cancel() tea.Cmd {
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
