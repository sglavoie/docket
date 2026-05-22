package tui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
)

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
