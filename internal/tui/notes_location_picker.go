package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

type notesLocationPickerPurpose uint8

const (
	notesLocationPickerScope notesLocationPickerPurpose = iota
	notesLocationPickerCreation
	notesLocationPickerMove
)

type notesLocationPickerItem struct {
	Label    string
	Scope    NotesScope
	Location app.NoteLocation
}

func (item notesLocationPickerItem) FilterText() string { return item.Label }

func (item notesLocationPickerItem) DisplayText() string { return item.Label }

func (item notesLocationPickerItem) CountValue() int { return 0 }

func (item notesLocationPickerItem) ItemID() string {
	switch item.Scope.Kind {
	case NotesScopeAll:
		return "notes-location:all"
	case NotesScopeGlobal:
		return "notes-location:global"
	case NotesScopeProject:
		return "notes-location:project:" + item.Scope.ProjectID
	default:
		return ""
	}
}

type notesLocationPickerResultMsg struct {
	Purpose  notesLocationPickerPurpose
	Item     notesLocationPickerItem
	Selected bool
}

// notesLocationPicker is the shared searchable interaction used for route
// scope, note creation, and note moves. Purpose is retained in every result so
// callers cannot accidentally apply a scope as a destination or vice versa.
type notesLocationPicker struct {
	*ListSurfaceAdapter[notesLocationPickerItem]
	purpose      notesLocationPickerPurpose
	alwaysSearch *alwaysSearchSinglePicker[notesLocationPickerItem]
}

func (picker *notesLocationPicker) contains(item notesLocationPickerItem) bool {
	id := item.ItemID()
	if id == "" {
		return false
	}
	for _, candidate := range picker.SourceItems() {
		if candidate.ItemID() == id &&
			candidate.Scope == item.Scope &&
			candidate.Location == item.Location {
			return true
		}
	}
	return false
}

func newNotesLocationPicker(
	purpose notesLocationPickerPurpose,
	currentScope NotesScope,
	excludedLocation app.NoteLocation,
	projects []*models.Project,
) *notesLocationPicker {
	items := make([]notesLocationPickerItem, 0, len(projects)+2)
	if purpose == notesLocationPickerScope {
		items = append(items, notesLocationPickerItem{
			Label: "All Notes",
			Scope: AllNotesScope(),
		})
	}
	global := app.GlobalNoteLocation()
	if purpose != notesLocationPickerMove || global != excludedLocation {
		items = append(items, notesLocationPickerItem{
			Label:    "Global",
			Scope:    GlobalNotesScope(),
			Location: global,
		})
	}
	for _, project := range projects {
		if project == nil || strings.TrimSpace(project.ID) == "" {
			continue
		}
		location := app.ProjectNoteLocation(project.ID)
		if purpose == notesLocationPickerMove && location == excludedLocation {
			continue
		}
		label := strings.TrimSpace(project.Name)
		if label == "" {
			label = project.ID
		}
		items = append(items, notesLocationPickerItem{
			Label:    label,
			Scope:    ProjectNotesScope(project.ID),
			Location: location,
		})
	}

	emptyMessage := "No note locations available."
	if purpose == notesLocationPickerMove {
		emptyMessage = "No move destinations available."
	}
	picker := NewListSurfaceAdapter(items, PickerConfig{
		EmptyMessage:   emptyMessage,
		NoMatchMessage: "No matching note locations.",
		MultiSelect:    false,
		ShowCounts:     false,
	})
	model := &notesLocationPicker{
		ListSurfaceAdapter: picker,
		purpose:            purpose,
	}
	model.alwaysSearch = newAlwaysSearchSinglePicker(
		model.ListSurfaceAdapter,
		alwaysSearchSinglePickerConfig[notesLocationPickerItem]{
			CancelMsg: func() tea.Msg {
				return notesLocationPickerResultMsg{Purpose: purpose}
			},
			SelectMsg: func(cursor int, filtered []notesLocationPickerItem) tea.Msg {
				return notesLocationPickerResultMsg{
					Purpose:  purpose,
					Item:     filtered[cursor],
					Selected: true,
				}
			},
		},
	)
	model.PrimeAlwaysSearch()

	selectedScope := currentScope
	switch purpose {
	case notesLocationPickerCreation:
		selectedScope = notesCreationDefaultScope(currentScope)
	case notesLocationPickerMove:
		selectedScope = NotesScope{}
	case notesLocationPickerScope:
	}
	for _, item := range items {
		if item.Scope == selectedScope {
			model.SetCursorToID(item.ItemID())
			break
		}
	}
	return model
}

func notesCreationDefaultScope(scope NotesScope) NotesScope {
	if scope.Kind == NotesScopeProject && strings.TrimSpace(scope.ProjectID) != "" {
		return ProjectNotesScope(scope.ProjectID)
	}
	return GlobalNotesScope()
}

func (picker *notesLocationPicker) Init() tea.Cmd {
	return picker.alwaysSearch.Init()
}

func (picker *notesLocationPicker) Update(msg tea.Msg) (bool, tea.Cmd) {
	// Location selection owns Esc as a one-step cancellation even when a query
	// is present. That keeps the manager route intact and avoids requiring a
	// second Esc that the shell could mistake for route navigation.
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
		return true, picker.alwaysSearch.cancel()
	}
	return picker.alwaysSearch.Update(msg)
}

func (picker *notesLocationPicker) View() tea.View {
	width := max(picker.Width(), 1)
	height := max(picker.Height(), 1)
	title := "Select scope"
	switch picker.purpose {
	case notesLocationPickerCreation:
		title = "Create note in"
	case notesLocationPickerMove:
		title = "Move note to"
	case notesLocationPickerScope:
	}
	header := lipgloss.JoinVertical(
		lipgloss.Left,
		chrome.RenderFullscreenTitle(title, "Notes", picker.AccentColor()),
		"Search: "+picker.SearchInputView(),
	)
	guidance, _ := picker.guidanceSpec()
	footer := chrome.RenderGuidance(guidance, width, picker.AccentColor())
	contentHeight := chrome.FullscreenContentHeightForBlocks(height, header, footer)
	content := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:    width,
		Height:   contentHeight,
		Layout:   ListSurfaceLayoutCompact,
		HideHelp: true,
	}).Render(picker.surface)
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chrome.FitBlockHeight(content, contentHeight),
		footer,
	))
}

func (picker *notesLocationPicker) guidanceSpec() (chrome.GuidanceSpec, bool) {
	count := len(picker.FilteredItems())
	hasSelection := func() bool { return count > 0 }
	verb := "apply"
	switch picker.purpose {
	case notesLocationPickerCreation:
		verb = "choose"
	case notesLocationPickerMove:
		verb = "move"
	case notesLocationPickerScope:
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("↑/↓ or Ctrl+J/K")),
		keymap.Bind("enter", "Apply selection", "Actions",
			keymap.WithGate(hasSelection), keymap.WithFooter(1, verb, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Cancel picker", "Navigation",
			keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc")),
	}}
	spec, ok := listSurfaceGuidanceSpec(picker.surface.GuidanceState(true), km, nil)
	if !ok {
		return chrome.GuidanceSpec{}, false
	}
	spec.Supplemental = fmt.Sprintf("%d %s", count, pluralize(count, "location", "locations"))
	return spec, true
}

func (picker *notesLocationPicker) GetHelpItems() []HelpItem {
	selectionDescription := "Apply Notes scope"
	switch picker.purpose {
	case notesLocationPickerCreation:
		selectionDescription = "Choose creation location"
	case notesLocationPickerMove:
		selectionDescription = "Move note immediately"
	case notesLocationPickerScope:
	}
	return []HelpItem{
		{Key: "↑/↓", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+J/K", Description: "Move selection", Category: "Navigation"},
		{Key: "type", Description: "Filter locations", Category: "Filters"},
		{Key: "Enter", Description: selectionDescription, Category: "Actions"},
		{Key: "Esc", Description: "Cancel picker", Category: "Navigation"},
	}
}
