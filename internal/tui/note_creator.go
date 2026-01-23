package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// NoteCreatorModel handles creating a new note via TUI.
// Notes are simplified items without status, priority, or tags.
type NoteCreatorModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	// Form fields
	titleInput textarea.Model
	descInput  textarea.Model

	// Selectable fields
	parentID     *string
	parentCursor int
	validParents []*models.Task

	// Form state
	focusIndex int // 0=title, 1=parent, 2=description
	width      int
	height     int
	err        error
}

// NewNoteCreatorModel creates a new note creator.
func NewNoteCreatorModel(project *models.Project, cfg *config.Config, manager *config.Manager, suggestedParent *models.Task) *NoteCreatorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInputFocused("Note title")
	titleInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	// Description with larger height for notes (8 lines vs 4 for tasks)
	descInput := factory.NewDescriptionInput("Note content (optional)")
	descInput.SetHeight(8)
	descInput.CharLimit = 5000
	descInput.SetPromptFunc(0, func(lineIdx int) string { return "" })

	m := &NoteCreatorModel{
		project:       project,
		config:        cfg,
		configManager: manager,
		titleInput:    titleInput,
		descInput:     descInput,
		parentID:      nil,
		parentCursor:  0,
		focusIndex:    0,
		width:         80,
		height:        24,
	}

	// Set initial parent by finding nearest valid ancestor
	if suggestedParent != nil {
		// Walk up the parent chain to find a task that can accept notes
		candidate := suggestedParent
		for candidate != nil {
			if candidate.CanAddChild(models.TaskTypeNote) {
				m.parentID = &candidate.ID
				break
			}
			candidate = candidate.Parent
		}
	}

	m.updateValidParents()

	return m
}

// updateValidParents filters tasks that can be parents of a note.
func (m *NoteCreatorModel) updateValidParents() {
	m.validParents = []*models.Task{}

	for _, task := range m.project.Tasks {
		if task.CanAddChild(models.TaskTypeNote) {
			m.validParents = append(m.validParents, task)
		}
	}

	// Reset parent selection if current parent is no longer valid
	if m.parentID != nil {
		found := false
		for i, p := range m.validParents {
			if p.ID == *m.parentID {
				found = true
				m.parentCursor = i + 1 // +1 because 0 is "None"
				break
			}
		}
		if !found {
			m.parentID = nil
			m.parentCursor = 0
		}
	}
}

// SetSize sets the view dimensions.
func (m *NoteCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
	formFactory.ResizeTextarea(&m.descInput, width)
}

func (m *NoteCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *NoteCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ParentSelectedMsg:
		// Handle parent selection from modal
		if msg.Source == ViewNoteCreator {
			m.parentID = msg.ParentID
			// Update cursor position to match
			if msg.ParentID == nil {
				m.parentCursor = 0
			} else {
				for i, p := range m.validParents {
					if p.ID == *msg.ParentID {
						m.parentCursor = i + 1
						break
					}
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 3
			} else {
				m.focusIndex = (m.focusIndex + 2) % 3
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			if m.focusIndex == 2 {
				// In description, allow newlines
				m.descInput, cmd = m.descInput.Update(msg)
				return m, cmd
			}
			// Move to next field
			if m.focusIndex < 2 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Submit form
			return m, m.createNote()

		case "/":
			// Open parent selector modal when on parent field
			if m.focusIndex == 1 {
				return m, func() tea.Msg {
					return OpenParentSelectorMsg{
						ValidParents: m.validParents,
						CurrentID:    m.parentID,
						AllowNone:    true,
						Source:       ViewNoteCreator,
					}
				}
			}

		case "left", "h":
			if m.focusIndex == 1 { // Parent selector
				if m.parentCursor > 0 {
					m.parentCursor--
					if m.parentCursor == 0 {
						m.parentID = nil
					} else {
						m.parentID = &m.validParents[m.parentCursor-1].ID
					}
				}
			}

		case "right", "l":
			if m.focusIndex == 1 { // Parent selector
				if m.parentCursor < len(m.validParents) {
					m.parentCursor++
					m.parentID = &m.validParents[m.parentCursor-1].ID
				}
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case 2:
		m.descInput, cmd = m.descInput.Update(msg)
	}

	return m, cmd
}

func (m *NoteCreatorModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Blur()
	case 2:
		m.descInput.Blur()
	}
}

func (m *NoteCreatorModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.titleInput.Focus()
	case 2:
		m.descInput.Focus()
	}
}

func (m *NoteCreatorModel) createNote() tea.Cmd {
	return func() tea.Msg {
		// Replace newlines with spaces since titles should be single logical lines
		title := strings.ReplaceAll(m.titleInput.Value(), "\n", " ")
		title = strings.TrimSpace(title)
		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("note title cannot be empty")}
		}

		// Create note
		task := &models.Task{
			ID:          uuid.New().String(),
			ShortID:     m.project.GetNextShortID(),
			Type:        models.TaskTypeNote,
			Title:       title,
			Description: strings.TrimSpace(m.descInput.Value()),
			Status:      models.StatusTodo, // Notes still have status internally
			Priority:    3,                 // Default priority, not displayed
			ParentID:    m.parentID,
			Section:     models.SectionNext,
		}

		// Add task to project and reorder so it appears before completed tasks
		m.project.Tasks = append(m.project.Tasks, task)
		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Create backup before saving for undo support
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Save project
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		storage.LoadProject(m.project)

		return TaskCreatedMsg{Task: task}
	}
}

func (m *NoteCreatorModel) View() string {
	var b strings.Builder

	// Title input (textarea spans 2 lines)
	titleLabel := "Title:\n"
	if m.focusIndex == 0 {
		titleLabel = FieldLabelFocusedStyle.Render("Title:\n")
	} else {
		titleLabel = FieldLabelStyle.Render("Title:\n")
	}
	b.WriteString(titleLabel)
	b.WriteString(m.titleInput.View())
	b.WriteString("\n\n")

	// Parent selector
	parentLabel := "Parent: "
	if m.focusIndex == 1 {
		parentLabel = FieldLabelFocusedStyle.Render("Parent: ")
	} else {
		parentLabel = FieldLabelStyle.Render("Parent: ")
	}
	b.WriteString(parentLabel)

	// "None" option
	if m.parentCursor == 0 {
		if m.focusIndex == 1 {
			b.WriteString(SelectorOptionSelectedStyle.Render("[None]"))
		} else {
			b.WriteString(BoldStyle.Render("[None]"))
		}
	} else {
		b.WriteString(SelectorOptionStyle.Render(" None "))
	}

	// Show a few valid parents
	maxShow := 3
	startIdx := 0
	if m.parentCursor > maxShow {
		startIdx = m.parentCursor - maxShow
	}

	for i := startIdx; i < len(m.validParents) && i < startIdx+maxShow+1; i++ {
		p := m.validParents[i]
		display := util.Truncate(fmt.Sprintf("#%d %s", p.ShortID, p.Title), 20)
		if m.parentCursor == i+1 {
			if m.focusIndex == 1 {
				b.WriteString(SelectorOptionSelectedStyle.Render(fmt.Sprintf(" [%s]", display)))
			} else {
				b.WriteString(BoldStyle.Render(fmt.Sprintf(" [%s]", display)))
			}
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", display)))
		}
	}

	if len(m.validParents) > maxShow+1 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" ... (%d more)", len(m.validParents)-maxShow-1)))
	}
	b.WriteString("\n\n")

	// Description textarea (larger for notes)
	descLabel := "Content:\n"
	if m.focusIndex == 2 {
		descLabel = FieldLabelFocusedStyle.Render("Content:\n")
	} else {
		descLabel = FieldLabelStyle.Render("Content:\n")
	}
	b.WriteString(descLabel)
	b.WriteString(m.descInput.View())

	// Error display
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Help footer
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Tab: fields · ←/→: parent · /: search parents · Ctrl+S: save · Esc: cancel"))

	// Wrap in overlay card
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}
	config := OverlayCardConfig{
		Title:        "Create Note",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}
