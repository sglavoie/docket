package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) notesViewCommands() []Command {
	return []Command{
		{
			Name:        "edit note",
			Description: "Edit note in external editor",
			Shortcut:    "e",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"open note editor", "modify note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete note",
			Description: "Delete selected note",
			Shortcut:    "d",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"remove note", "trash note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "undo delete note",
			Description: "Undo last note deletion",
			Shortcut:    "u",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"restore note", "undelete note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "link note to task",
			Description: "Link note to a task",
			Shortcut:    "l",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"attach note", "note task link"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
			},
		},
		{
			Name:        "toggle note pin",
			Description: "Toggle pin status on note",
			Shortcut:    "p",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"pin note", "unpin note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
			},
		},
		{
			Name:        "edit note tags",
			Description: "Add or remove tags on note",
			Shortcut:    "t",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"tag note", "note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "cycle note tag filter",
			Description: "Open note tag filter selector",
			Shortcut:    "T",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"filter notes by tag"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
			},
		},
		{
			Name:        "search notes",
			Description: "Search notes by title or content",
			Shortcut:    "/",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"find note", "note search"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
			},
		},
		{
			Name:        "create note",
			Description: "Create a new note",
			Shortcut:    "n",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "rename note",
			Description: "Rename selected note inline",
			Shortcut:    "r",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"note rename", "change note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "cycle note sort",
			Description: "Cycle sort mode (date/title/pinned)",
			Shortcut:    "s",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"sort notes", "note sort mode"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "clear tag filter",
			Description: "Clear all note tag filters",
			Shortcut:    "C",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"reset note filters", "clear note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
			},
		},
		{
			Name:        "copy note title",
			Description: "Copy note title to clipboard",
			Shortcut:    "y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy note body",
			Description: "Copy note body to clipboard",
			Shortcut:    "Y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note body"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy note title and body",
			Description: "Copy title and body to clipboard",
			Shortcut:    "ctrl+y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note full"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
		{
			Name:        "toggle multi-select",
			Description: "Toggle note multi-select mode",
			Shortcut:    "V",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"multi select notes"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}}
			},
		},
		{
			Name:        "view note",
			Description: "View selected note",
			Shortcut:    "enter",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"open note", "read note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		},
	}
}
