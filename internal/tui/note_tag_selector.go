package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// NoteTagSelectorModel allows multi-selecting tags for a note.
type NoteTagSelectorModel struct {
	*BasePicker[TagInfo]
	note         *models.Note
	project      *models.Project
	originalTags []string

	// Add tag mode
	addTagMode  bool
	addTagInput textinput.Model
	addTagError string
}

// NewNoteTagSelectorModel creates a new tag selector for a note.
func NewNoteTagSelectorModel(note *models.Note, project *models.Project, currentTags []string) *NoteTagSelectorModel {
	tags := buildNoteTagList(project)

	titleStr := note.Title
	if len(titleStr) > 30 {
		titleStr = titleStr[:27] + "..."
	}

	picker := NewBasePicker(tags, PickerConfig{
		Title:          fmt.Sprintf("Tags for: %s", titleStr),
		EmptyMessage:   "No tags found (press 'a' to add)",
		NoMatchMessage: "No matching tags (press 'a' to add)",
		MultiSelect:    true,
		ShowCounts:     true,
	})

	// Pre-select current tags
	for _, tag := range currentTags {
		picker.PreSelectByID(tag)
	}
	if len(currentTags) > 0 {
		picker.SetCursorToID(currentTags[0])
	}

	addInput := textinput.New()
	addInput.CharLimit = 50

	return &NoteTagSelectorModel{
		BasePicker:   picker,
		note:         note,
		project:      project,
		originalTags: append([]string{}, currentTags...),
		addTagInput:  addInput,
	}
}

// buildNoteTagList scans all notes in the project and builds a sorted tag list.
func buildNoteTagList(project *models.Project) []TagInfo {
	tagCounts := make(map[string]int)
	notes, _ := storage.ListNotes(project)
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagCounts[tag]++
		}
	}
	// Also include task tags for cross-pollination
	for _, task := range project.Tasks {
		for _, tag := range task.Tags {
			if _, exists := tagCounts[tag]; !exists {
				tagCounts[tag] = 0
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

	case tea.KeyMsg:
		if m.addTagMode {
			return m.handleAddTagMode(msg)
		}

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return NoteTagSelectorDoneMsg{
					Note:     m.note,
					Project:  m.project,
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
					SelectedTags: selectedTags,
					Canceled:     false,
				}
			}

		case "a":
			m.addTagMode = true
			m.addTagError = ""
			m.addTagInput.Reset()
			m.addTagInput.SetValue("")
			m.addTagInput.Focus()
			return m, textinput.Blink

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

func (m *NoteTagSelectorModel) handleAddTagMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	case "enter":
		tagName := strings.TrimSpace(m.addTagInput.Value())
		if tagName == "" {
			m.addTagError = "Tag name cannot be empty"
			return m, nil
		}

		isDuplicate := false
		for _, item := range m.Items {
			if item.Name == tagName {
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			m.PreSelectByID(tagName)
			m.SetCursorToID(tagName)
			m.addTagError = fmt.Sprintf("Tag '%s' already exists - selected it", tagName)
			m.addTagMode = false
			m.addTagInput.Blur()
			return m, nil
		}

		newTag := TagInfo{Name: tagName, TaskCount: 0}
		m.Items = append(m.Items, newTag)
		sort.Slice(m.Items, func(i, j int) bool {
			return m.Items[i].Name < m.Items[j].Name
		})

		m.Filter(m.SearchQuery())
		m.PreSelectByID(tagName)
		m.SetCursorToID(tagName)

		m.addTagMode = false
		m.addTagError = ""
		m.addTagInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.addTagInput, cmd = m.addTagInput.Update(msg)
		return m, cmd
	}
}

// HasActiveInput returns true when in add tag mode.
func (m *NoteTagSelectorModel) HasActiveInput() bool {
	return m.addTagMode
}

// HasTextInput returns true when the add-tag input is active.
func (m *NoteTagSelectorModel) HasTextInput() bool {
	return m.HasActiveInput()
}

// ShouldCaptureEsc returns true when the add-tag input is active.
func (m *NoteTagSelectorModel) ShouldCaptureEsc() bool {
	return m.HasActiveInput()
}

// View renders the note tag selector.
func (m *NoteTagSelectorModel) View() string {
	if m.addTagMode {
		return m.renderAddTagMode()
	}

	boxWidth := 80
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	content.WriteString(m.RenderSearchBar())

	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	m.Config.BoxWidth = boxWidth
	content.WriteString(m.RenderList(maxVisible))

	content.WriteString("\n")
	content.WriteString(m.renderCustomFooter())

	selectedCount := len(m.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No tags selected"
	} else {
		subtitle = fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))
	}

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    m.Config.Title,
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

func (m *NoteTagSelectorModel) renderCustomFooter() string {
	if m.IsSearchMode() {
		return MutedStyle.Render("enter done | esc clear")
	}
	if len(m.Filtered) == 0 {
		return MutedStyle.Render("a add tag | esc cancel")
	}
	return MutedStyle.Render("space toggle | a add tag | / search | c clear | enter apply | esc cancel")
}

func (m *NoteTagSelectorModel) renderAddTagMode() string {
	var content strings.Builder

	content.WriteString(MutedStyle.Render("Enter a tag name (max 50 characters)"))
	content.WriteString("\n\n")

	content.WriteString(m.addTagInput.View())
	content.WriteString("\n\n")

	if m.addTagError != "" {
		errorStyle := BoldStyle.Foreground(ColorError)
		content.WriteString(errorStyle.Render(m.addTagError))
		content.WriteString("\n\n")
	}

	content.WriteString(MutedStyle.Render("enter add tag | esc cancel"))

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Add New Tag",
		Width:    50,
		Centered: true,
	})
}
