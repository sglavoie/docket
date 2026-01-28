package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ShowNoteTagFilterSelectorMsg opens the tag filter selector for notes.
type ShowNoteTagFilterSelectorMsg struct {
	Project *models.Project
}

// NoteTagFilterSelectorDoneMsg returns the result from the note tag filter selector.
type NoteTagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Canceled     bool
}

// NoteTagFilterSelectorModel allows selecting multiple tags to filter notes by.
type NoteTagFilterSelectorModel struct {
	*BasePicker[TagInfo]
	project *models.Project
}

// NewNoteTagFilterSelectorModel creates a new tag filter selector for notes.
func NewNoteTagFilterSelectorModel(project *models.Project, currentFilterTags map[string]bool) *NoteTagFilterSelectorModel {
	tags := buildNoteFilterTagList(project)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          "Filter Notes by Tags",
		EmptyMessage:   "No tags in notes",
		NoMatchMessage: "No matching tags",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select currently filtered tags
	for tag := range currentFilterTags {
		picker.PreSelectByID(tag)
	}

	return &NoteTagFilterSelectorModel{
		BasePicker: picker,
		project:    project,
	}
}

// buildNoteFilterTagList scans all notes and builds a sorted list of tags with counts.
func buildNoteFilterTagList(project *models.Project) []TagInfo {
	tagCounts := make(map[string]int)
	notes, _ := storage.ListNotes(project)
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// SetSize sets the view dimensions.
func (m *NoteTagFilterSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *NoteTagFilterSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *NoteTagFilterSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return NoteTagFilterSelectorDoneMsg{Canceled: true}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return NoteTagFilterSelectorDoneMsg{
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the note tag filter selector.
func (m *NoteTagFilterSelectorModel) View() string {
	selectedCount := len(m.Selected)
	var headerContent string
	if selectedCount == 0 {
		headerContent = MutedStyle.Render("No tags selected (showing all)")
	} else {
		headerContent = MutedStyle.Render(fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s")))
	}

	return m.RenderInBox(m.Config.Title, headerContent, 0)
}
