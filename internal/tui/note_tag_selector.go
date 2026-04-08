package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// NoteTagSelectorModel allows multi-selecting tags for a note.
type NoteTagSelectorModel struct {
	*BasePicker[TagInfo]
	TagAdder
	note         *models.Note
	project      *models.Project
	notesDir     string
	isGlobal     bool
	originalTags []string
}

// NewNoteTagSelectorModel creates a new tag selector for a note.
func NewNoteTagSelectorModel(note *models.Note, project *models.Project, notesDir string, isGlobal bool, currentTags []string) *NoteTagSelectorModel {
	tags := buildNoteTagList(project, notesDir)

	titleStr := truncateToVisualWidth(note.Title, 30)

	picker := NewBasePicker(tags, PickerConfig{
		Title:          fmt.Sprintf("Tags for: %s", titleStr),
		EmptyMessage:   "No tags found (press 'a' to add).",
		NoMatchMessage: "No matching tags (press 'a' to add).",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	for _, tag := range currentTags {
		picker.PreSelectByID(tag)
	}
	if len(currentTags) > 0 {
		picker.SetCursorToID(currentTags[0])
	}

	return &NoteTagSelectorModel{
		BasePicker:   picker,
		TagAdder:     NewTagAdder(),
		note:         note,
		project:      project,
		notesDir:     notesDir,
		isGlobal:     isGlobal,
		originalTags: append([]string{}, currentTags...),
	}
}

// buildNoteTagList scans all notes and builds a sorted tag list.
func buildNoteTagList(project *models.Project, notesDir string) []TagInfo {
	tagCounts := make(map[string]int)
	var notes []*models.Note
	if project != nil {
		notes, _ = storage.ListNotes(project)
	} else if notesDir != "" {
		notes, _ = storage.ListNotesFrom(notesDir)
	}
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagCounts[tag]++
		}
	}
	// Also include task tags for cross-pollination
	if project != nil {
		for _, task := range project.Tasks {
			for _, tag := range task.Tags {
				if _, exists := tagCounts[tag]; !exists {
					tagCounts[tag] = 0
				}
			}
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
func (m *NoteTagSelectorModel) SetSize(width, height int) {
	m.BasePicker.SetSize(width, height)
}

// Init initializes the model.
func (m *NoteTagSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *NoteTagSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.AddTagMode {
			handled, cmd := m.HandleAddTagKey(msg, m.BasePicker)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return NoteTagSelectorDoneMsg{
					Note:     m.note,
					Project:  m.project,
					NotesDir: m.notesDir,
					IsGlobal: m.isGlobal,
					Canceled: true,
				}
			}

		case "enter":
			selectedTags := m.SelectedIDs()
			sort.Strings(selectedTags)
			return m, func() tea.Msg {
				return NoteTagSelectorDoneMsg{
					Note:         m.note,
					Project:      m.project,
					NotesDir:     m.notesDir,
					IsGlobal:     m.isGlobal,
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			return m, m.EnterAddMode()

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// View renders the note tag selector.
func (m *NoteTagSelectorModel) View() tea.View {
	return tea.NewView(RenderTagSelectorView(m.BasePicker, &m.TagAdder, m.Config.Title, m.Width(), m.Height()))
}

func (m *NoteTagSelectorModel) StatusBarLabel() string { return "Tag Note" }
