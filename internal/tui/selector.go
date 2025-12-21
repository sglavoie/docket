package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectorItem represents an item that can be selected.
type SelectorItem struct {
	ID      string
	Data    any
	Columns []string
}

// SelectorModel is the BubbleTea model for selection.
type SelectorModel struct {
	title         string
	items         []SelectorItem
	filteredItems []SelectorItem
	table         table.Model
	done          bool
	cancelled     bool
	selectedItem  *SelectorItem
	err           error
	showHelp      bool
	searchMode    bool
	searchInput   textinput.Model
	searchFocused bool
}

// NewSelectorModel creates a new selector.
func NewSelectorModel(title string, items []SelectorItem) SelectorModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Type to search..."
	searchInput.CharLimit = 50
	searchInput.Width = 40

	// Determine columns based on items
	var columns []table.Column
	if len(items) > 0 && len(items[0].Columns) > 0 {
		columnCount := len(items[0].Columns)
		baseWidth := 70 / columnCount
		columns = make([]table.Column, columnCount)
		for i := range columnCount {
			columns[i] = table.Column{
				Title: fmt.Sprintf("Col %d", i+1),
				Width: baseWidth,
			}
		}
	}

	rows := make([]table.Row, len(items))
	for i, item := range items {
		rows[i] = table.Row(item.Columns)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return SelectorModel{
		title:         title,
		items:         items,
		filteredItems: items,
		table:         t,
		showHelp:      true,
		searchMode:    false,
		searchInput:   searchInput,
		searchFocused: false,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SelectorModel) rebuildTable() {
	rows := make([]table.Row, len(m.filteredItems))
	for i, item := range m.filteredItems {
		rows[i] = table.Row(item.Columns)
	}
	m.table.SetRows(rows)
}

func (m *SelectorModel) filterItems(query string) {
	if query == "" {
		m.filteredItems = m.items
	} else {
		var filtered []SelectorItem
		queryTerms := strings.Fields(strings.ToLower(query))

		for _, item := range m.items {
			var textBuilder strings.Builder
			for i, col := range item.Columns {
				if i > 0 {
					textBuilder.WriteString(" ")
				}
				textBuilder.WriteString(strings.ToLower(col))
			}
			searchableText := textBuilder.String()

			allTermsMatch := true
			for _, term := range queryTerms {
				if !strings.Contains(searchableText, term) {
					allTermsMatch = false
					break
				}
			}

			if allTermsMatch {
				filtered = append(filtered, item)
			}
		}
		m.filteredItems = filtered
	}

	m.rebuildTable()

	if len(m.filteredItems) > 0 && m.table.Cursor() >= len(m.filteredItems) {
		m.table.SetCursor(0)
	}
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchFocused = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.filterItems("")
				return m, nil
			case "enter":
				if m.searchFocused {
					m.searchFocused = false
					m.searchInput.Blur()
					return m, nil
				} else {
					if len(m.filteredItems) > 0 {
						selectedRow := m.table.Cursor()
						if selectedRow < len(m.filteredItems) {
							m.done = true
							m.selectedItem = &m.filteredItems[selectedRow]
							return m, tea.Quit
						}
					}
				}
			case "tab":
				m.searchFocused = !m.searchFocused
				if m.searchFocused {
					m.searchInput.Focus()
				} else {
					m.searchInput.Blur()
				}
				return m, textinput.Blink
			case "up", "ctrl+p", "down", "ctrl+n":
				if m.searchFocused {
					m.searchFocused = false
					m.searchInput.Blur()
				}
				m.table, cmd = m.table.Update(msg)
				return m, cmd
			default:
				if m.searchFocused {
					oldValue := m.searchInput.Value()
					m.searchInput, cmd = m.searchInput.Update(msg)
					newValue := m.searchInput.Value()
					if oldValue != newValue {
						m.filterItems(newValue)
					}
					return m, cmd
				} else {
					m.table, cmd = m.table.Update(msg)
					return m, cmd
				}
			}
		} else {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.done = true
				m.cancelled = true
				m.err = fmt.Errorf("cancelled")
				return m, tea.Quit
			case "enter":
				if len(m.filteredItems) > 0 {
					selectedRow := m.table.Cursor()
					if selectedRow < len(m.filteredItems) {
						m.done = true
						m.selectedItem = &m.filteredItems[selectedRow]
						return m, tea.Quit
					}
				}
			case "/", "f":
				m.searchMode = true
				m.searchFocused = true
				m.searchInput.Focus()
				return m, textinput.Blink
			case "?", "h":
				m.showHelp = !m.showHelp
				return m, nil
			default:
				m.table, cmd = m.table.Update(msg)
				return m, cmd
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m SelectorModel) View() string {
	if len(m.items) == 0 {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")).
			Render("No items to display")
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n")

	if m.searchMode {
		searchStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33")).
			MarginBottom(1)
		b.WriteString(searchStyle.Render("Search: "))
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
	}

	if m.searchMode && len(m.filteredItems) < len(m.items) {
		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginBottom(1)
		count := fmt.Sprintf("Showing %d of %d items", len(m.filteredItems), len(m.items))
		b.WriteString(countStyle.Render(count))
		b.WriteString("\n")
	}

	if len(m.filteredItems) == 0 {
		noResultsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			MarginTop(1).
			MarginBottom(1)
		b.WriteString(noResultsStyle.Render("No matching items found"))
		b.WriteString("\n")
	} else {
		baseStyle := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
		b.WriteString(baseStyle.Render(m.table.View()))
		b.WriteString("\n")
	}

	if m.showHelp {
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		help := "j/k: Navigate | /: Search | Enter: Select | q/Esc: Cancel"
		b.WriteString(helpStyle.Render(help))
	}

	return b.String()
}

func (m SelectorModel) GetSelectedItem() *SelectorItem {
	return m.selectedItem
}

func (m SelectorModel) GetError() error {
	return m.err
}

func (m SelectorModel) IsCancelled() bool {
	return m.cancelled
}

// RunSelector runs the selector TUI and returns the selected item.
func RunSelector(title string, items []SelectorItem) (*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}

	model := NewSelectorModel(title, items)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run selector: %w", err)
	}

	selectorModel := finalModel.(SelectorModel)
	if selectorModel.GetError() != nil {
		return nil, selectorModel.GetError()
	}

	if selectorModel.IsCancelled() {
		return nil, fmt.Errorf("cancelled")
	}

	return selectorModel.GetSelectedItem(), nil
}
