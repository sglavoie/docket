package notes

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
)

type NotesLocationPickerPurpose uint8

const (
	NotesLocationPickerScope NotesLocationPickerPurpose = iota
	NotesLocationPickerCreation
	NotesLocationPickerMove
)

type NotesLocationPickerItem struct {
	Label    string
	Scope    route.NotesScope
	Location app.NoteLocation
}

func (item NotesLocationPickerItem) FilterText() string { return item.Label }

func (item NotesLocationPickerItem) DisplayText() string { return item.Label }

func (item NotesLocationPickerItem) CountValue() int { return 0 }

func (item NotesLocationPickerItem) ItemID() string {
	switch item.Scope.Kind {
	case route.NotesScopeAll:
		return "notes-location:all"
	case route.NotesScopeGlobal:
		return "notes-location:global"
	case route.NotesScopeProject:
		return "notes-location:project:" + item.Scope.ProjectID
	default:
		return ""
	}
}

type NotesLocationPickerResultMsg struct {
	Purpose  NotesLocationPickerPurpose
	Item     NotesLocationPickerItem
	Selected bool
}

// notesLocationPicker is the shared searchable interaction used for route
// scope, note creation, and note moves. Purpose is retained in every result so
// callers cannot accidentally apply a scope as a destination or vice versa.
type notesLocationPicker struct {
	*listsurface.ListSurfaceAdapter[NotesLocationPickerItem]
	Purpose      NotesLocationPickerPurpose
	alwaysSearch *listsurface.AlwaysSearchSinglePicker[NotesLocationPickerItem]
}

func (picker *notesLocationPicker) contains(item NotesLocationPickerItem) bool {
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
	purpose NotesLocationPickerPurpose,
	currentScope route.NotesScope,
	excludedLocation app.NoteLocation,
	projects []*models.Project,
) *notesLocationPicker {
	items := make([]NotesLocationPickerItem, 0, len(projects)+2)
	if purpose == NotesLocationPickerScope {
		items = append(items, NotesLocationPickerItem{
			Label: "All Notes",
			Scope: route.AllNotesScope(),
		})
	}
	global := app.GlobalNoteLocation()
	if purpose != NotesLocationPickerMove || global != excludedLocation {
		items = append(items, NotesLocationPickerItem{
			Label:    "Global",
			Scope:    route.GlobalNotesScope(),
			Location: global,
		})
	}
	for _, project := range projects {
		if project == nil || strings.TrimSpace(project.ID) == "" {
			continue
		}
		location := app.ProjectNoteLocation(project.ID)
		if purpose == NotesLocationPickerMove && location == excludedLocation {
			continue
		}
		label := strings.TrimSpace(project.Name)
		if label == "" {
			label = project.ID
		}
		items = append(items, NotesLocationPickerItem{
			Label:    label,
			Scope:    route.ProjectNotesScope(project.ID),
			Location: location,
		})
	}

	emptyMessage := "No note locations available."
	if purpose == NotesLocationPickerMove {
		emptyMessage = "No move destinations available."
	}
	picker := listsurface.NewListSurfaceAdapter(items, listsurface.PickerConfig{
		EmptyMessage:   emptyMessage,
		NoMatchMessage: "No matching note locations.",
		MultiSelect:    false,
		ShowCounts:     false,
	})
	model := &notesLocationPicker{
		ListSurfaceAdapter: picker,
		Purpose:            purpose,
	}
	model.alwaysSearch = listsurface.NewAlwaysSearchSinglePicker(
		model.ListSurfaceAdapter,
		listsurface.AlwaysSearchSinglePickerConfig[NotesLocationPickerItem]{
			CancelMsg: func() tea.Msg {
				return NotesLocationPickerResultMsg{Purpose: purpose}
			},
			SelectMsg: func(cursor int, filtered []NotesLocationPickerItem) tea.Msg {
				return NotesLocationPickerResultMsg{
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
	case NotesLocationPickerCreation:
		selectedScope = notesCreationDefaultScope(currentScope)
	case NotesLocationPickerMove:
		selectedScope = route.NotesScope{}
	case NotesLocationPickerScope:
	}
	for _, item := range items {
		if item.Scope == selectedScope {
			model.SetCursorToID(item.ItemID())
			break
		}
	}
	return model
}

func notesCreationDefaultScope(scope route.NotesScope) route.NotesScope {
	if scope.Kind == route.NotesScopeProject && strings.TrimSpace(scope.ProjectID) != "" {
		return route.ProjectNotesScope(scope.ProjectID)
	}
	return route.GlobalNotesScope()
}

func (picker *notesLocationPicker) Init() tea.Cmd {
	return picker.alwaysSearch.Init()
}

func (picker *notesLocationPicker) Update(msg tea.Msg) (bool, tea.Cmd) {
	// Location selection owns Esc as a one-step cancellation even when a query
	// is present. That keeps the manager route intact and avoids requiring a
	// second Esc that the shell could mistake for route navigation.
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
		return true, picker.alwaysSearch.Cancel()
	}
	return picker.alwaysSearch.Update(msg)
}

func (picker *notesLocationPicker) View() tea.View {
	width := max(picker.Width(), 1)
	height := max(picker.Height(), 1)
	title := "Select scope"
	switch picker.Purpose {
	case NotesLocationPickerCreation:
		title = "Create note in"
	case NotesLocationPickerMove:
		title = "Move note to"
	case NotesLocationPickerScope:
	}
	header := lipgloss.JoinVertical(
		lipgloss.Left,
		chrome.RenderFullscreenTitle(title, "Notes", picker.AccentColor()),
		"Search: "+picker.SearchInputView(),
	)
	guidance, _ := picker.guidanceSpec()
	footer := chrome.RenderGuidance(guidance, width, picker.AccentColor())
	contentHeight := chrome.FullscreenContentHeightForBlocks(height, header, footer)
	content := listsurface.NewListSurfaceRenderer(listsurface.ListSurfaceRenderConfig{
		Width:    width,
		Height:   contentHeight,
		Layout:   listsurface.ListSurfaceLayoutCompact,
		HideHelp: true,
	}).Render(picker.Surface)
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
	switch picker.Purpose {
	case NotesLocationPickerCreation:
		verb = "choose"
	case NotesLocationPickerMove:
		verb = "move"
	case NotesLocationPickerScope:
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("up", "Move selection", "Navigation",
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("↑/↓ or Ctrl+J/K")),
		keymap.Bind("enter", "Apply selection", "Actions",
			keymap.WithGate(hasSelection), keymap.WithFooter(1, verb, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Cancel picker", "Navigation",
			keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc")),
	}}
	spec, ok := listsurface.ListSurfaceGuidanceSpec(picker.Surface.GuidanceState(true), km, nil)
	if !ok {
		return chrome.GuidanceSpec{}, false
	}
	spec.Supplemental = fmt.Sprintf("%d %s", count, listsurface.Pluralize(count, "location", "locations"))
	return spec, true
}

func (picker *notesLocationPicker) GetHelpItems() []operation.HelpItem {
	selectionDescription := "Apply Notes scope"
	switch picker.Purpose {
	case NotesLocationPickerCreation:
		selectionDescription = "Choose creation location"
	case NotesLocationPickerMove:
		selectionDescription = "Move note immediately"
	case NotesLocationPickerScope:
	}
	return []operation.HelpItem{
		{Key: "↑/↓", Description: "Move selection", Category: "Navigation"},
		{Key: "Ctrl+J/K", Description: "Move selection", Category: "Navigation"},
		{Key: "type", Description: "Filter locations", Category: "Filters"},
		{Key: "Enter", Description: selectionDescription, Category: "Actions"},
		{Key: "Esc", Description: "Cancel picker", Category: "Navigation"},
	}
}
