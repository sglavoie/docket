package tui

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"

	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/tterrors"
)

// SelectorItem exposes the shared command-line selector payload.
type SelectorItem = selector.Item

// TerminalListModel is the interactive command-line adapter for ListSurface.
// ListSurface owns ranking, cursor, and selection behavior.
type TerminalListModel struct {
	title        string
	surface      *ListSurfaceModel
	selectedItem *SelectorItem
	canceled     bool
	err          error
	searching    bool
	showHelp     bool
}

func NewTerminalListModel(title string, items []SelectorItem) TerminalListModel {
	surface, err := ListSourceFromSelectorItems(items).NewModel(ListSurfaceConfig{
		SelectionMode: ListSelectionSingle,
		EmptyState:    ListEmptyState{Title: "No items to display"},
		NoMatchState:  ListEmptyState{Title: "No matching items found"},
	})
	return TerminalListModel{
		title:    title,
		surface:  surface,
		err:      err,
		showHelp: true,
	}
}

func (model TerminalListModel) Init() tea.Cmd {
	return nil
}

func (model TerminalListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || model.surface == nil {
		return model, nil
	}
	name := key.String()
	if model.searching {
		switch name {
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
			} else {
				model.searching = false
			}
		case "enter":
			model.searching = false
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
		case "up", "ctrl+p":
			model.surface.MoveCursor(-1)
		case "down", "ctrl+n":
			model.surface.MoveCursor(1)
		default:
			if key.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + key.Text)
			}
		}
		return model, nil
	}

	switch name {
	case "ctrl+c", "esc":
		model.canceled = true
		model.err = tterrors.ErrCanceled
		return model, tea.Quit
	case "/", "f":
		model.searching = true
	case "?", "h":
		model.showHelp = !model.showHelp
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
		item, exists := model.surface.CurrentItem()
		if !exists {
			return model, nil
		}
		selected, valid := item.Data.(SelectorItem)
		if !valid {
			model.err = fmt.Errorf("selected item %q has an invalid payload", item.ID)
			return model, tea.Quit
		}
		model.selectedItem = &selected
		return model, tea.Quit
	}
	return model, nil
}

func (model TerminalListModel) View() tea.View {
	if model.surface == nil {
		return tea.NewView(EmptyErrorStyle.Render("No items to display"))
	}
	var content strings.Builder
	content.WriteString(SelectorTitleStyle.Render(model.title))
	content.WriteString("\n")
	if model.searching || model.surface.Query().Raw != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		content.WriteString(SelectorSearchStyle.Render("Search: "))
		content.WriteString(model.surface.Query().Raw + cursor)
		content.WriteString("\n")
	}
	content.WriteString(RenderCompactListSurface(model.surface, 80, 12))
	if model.showHelp {
		content.WriteString("\n")
		content.WriteString(HelpStyle.Render("j/k navigate · / search · Enter select · Esc cancel"))
	}
	view := tea.NewView(content.String())
	view.AltScreen = true
	return view
}

func (model TerminalListModel) GetSelectedItem() *SelectorItem {
	return model.selectedItem
}

func (model TerminalListModel) GetError() error {
	return model.err
}

func (model TerminalListModel) IsCanceled() bool {
	return model.canceled
}

func selectorHasInteractiveTerminal() bool {
	stdin := os.Stdin.Fd()
	stdout := os.Stdout.Fd()
	return (isatty.IsTerminal(stdin) || isatty.IsCygwinTerminal(stdin)) &&
		(isatty.IsTerminal(stdout) || isatty.IsCygwinTerminal(stdout))
}

// RunSelector runs the ListSurface terminal adapter and returns the selected item.
func RunSelector(title string, items []SelectorItem) (*SelectorItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select from")
	}
	if !selectorHasInteractiveTerminal() {
		return nil, fmt.Errorf("interactive selector requires terminal stdin/stdout")
	}

	program := tea.NewProgram(NewTerminalListModel(title, items))
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run selector: %w", err)
	}
	model, ok := finalModel.(TerminalListModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type from selector: %T", finalModel)
	}
	if model.GetError() != nil {
		return nil, model.GetError()
	}
	if model.IsCanceled() {
		return nil, tterrors.ErrCanceled
	}
	return model.GetSelectedItem(), nil
}
