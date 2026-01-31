package tui

import (
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
	cmd := filterSelectorUpdate(m.BasePicker, msg,
		func() tea.Msg { return NoteTagFilterSelectorDoneMsg{Canceled: true} },
		func(selected []string) tea.Msg {
			return NoteTagFilterSelectorDoneMsg{SelectedTags: selected, Canceled: false}
		},
	)
	return m, cmd
}

// View renders the note tag filter selector.
func (m *NoteTagFilterSelectorModel) View() string {
	return filterSelectorView(m.BasePicker, "tag")
}
