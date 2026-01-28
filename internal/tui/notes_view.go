package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ShowNoteTagSelectorMsg opens the tag selector for a note.
type ShowNoteTagSelectorMsg struct {
	Note        *models.Note
	Project     *models.Project
	CurrentTags []string
}

// NoteTagSelectorDoneMsg returns the result from the note tag selector.
type NoteTagSelectorDoneMsg struct {
	Note         *models.Note
	Project      *models.Project
	SelectedTags []string
	Canceled     bool
}

// NotesViewModel displays and manages project notes.
type NotesViewModel struct {
	width  int
	height int

	project *models.Project
	config  interface{ GetEditor() string }

	notes  []*models.Note
	cursor int

	// Filter state
	filterTags map[string]bool
	filtering  bool

	// Delete confirmation state
	confirmDelete    bool
	pendingDeleteIdx int

	// Undo state
	lastDeleted *models.Note

	statusMessage string
}

// notesConfig is a minimal interface for config access.
type notesConfig interface {
	GetEditor() string
}

// NewNotesViewModel creates a new notes view.
func NewNotesViewModel(project *models.Project) *NotesViewModel {
	m := &NotesViewModel{
		project:    project,
		filterTags: make(map[string]bool),
	}
	m.loadNotes()
	return m
}

func (m *NotesViewModel) loadNotes() {
	notes, err := storage.ListNotes(m.project)
	if err != nil {
		m.notes = nil
		return
	}
	if len(m.filterTags) > 0 {
		var filtered []*models.Note
		for _, n := range notes {
			for _, t := range n.Tags {
				if m.filterTags[t] {
					filtered = append(filtered, n)
					break
				}
			}
		}
		notes = filtered
	}
	m.notes = notes
	if m.cursor >= len(m.notes) {
		m.cursor = max(0, len(m.notes)-1)
	}
}

func (m *NotesViewModel) Init() tea.Cmd {
	return nil
}

func (m *NotesViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Width returns the view width.
func (m *NotesViewModel) Width() int { return m.width }

// Height returns the view height.
func (m *NotesViewModel) Height() int { return m.height }

func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle delete confirmation mode
		if m.confirmDelete {
			m.confirmDelete = false
			if key == "d" {
				return m, m.performDelete()
			}
			m.statusMessage = "Delete cancelled"
			return m, nil
		}

		m.statusMessage = ""
		switch key {
		case "q", "esc":
			return m, func() tea.Msg { return CloseNotesMsg{} }
		case "j", "down":
			if m.cursor < len(m.notes)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			if len(m.notes) > 0 {
				m.cursor = len(m.notes) - 1
			}
		case "enter":
			return m, m.viewCurrentNote()
		case "e":
			return m, m.editCurrentNote()
		case "n":
			return m, m.createNote()
		case "d":
			return m, m.deleteCurrentNote()
		case "l":
			return m, m.linkCurrentNote()
		case "u":
			return m, m.undoDelete()
		case "p":
			m.togglePin()
		case "t":
			return m, m.showTagSelector()
		case "T":
			return m, func() tea.Msg {
				return ShowNoteTagFilterSelectorMsg{Project: m.project}
			}
		case "/":
			return m, func() tea.Msg {
				return ShowNotesSearchMsg{Project: m.project}
			}
		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					Items:      m.GetHelpItems(),
					SourceView: ViewNotes,
					ViewName:   "Notes",
				}
			}
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case NotesRefreshMsg:
		m.loadNotes()
	case NoteTaskLinkedMsg:
		m.loadNotes()
		m.statusMessage = "Linked note to task"
	}
	return m, nil
}

// ShouldCaptureEsc returns true when delete confirmation is active.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirmDelete
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return false
}

func (m *NotesViewModel) View() string {
	var b strings.Builder

	pinnedStyle := lipgloss.NewStyle().Foreground(ColorSuccess)

	// Title
	title := fmt.Sprintf("%s - Notes", m.project.Name)
	b.WriteString(TitleStyle.Render(title))
	if len(m.filterTags) > 0 {
		tags := make([]string, 0, len(m.filterTags))
		for t := range m.filterTags {
			tags = append(tags, t)
		}
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [tags: %s]", strings.Join(tags, ", "))))
	}
	b.WriteString("\n")

	// Note count
	b.WriteString(MutedStyle.Render(fmt.Sprintf("%d notes", len(m.notes))))
	b.WriteString("\n\n")

	if len(m.notes) == 0 {
		b.WriteString(MutedStyle.Render("No notes yet. Press 'n' to create one."))
		b.WriteString("\n")
	}

	// Calculate visible area
	availHeight := m.Height() - 6 // title + count + footer
	if availHeight < 1 {
		availHeight = 10
	}

	// Scrolling window
	startIdx := 0
	if m.cursor >= availHeight {
		startIdx = m.cursor - availHeight + 1
	}
	endIdx := startIdx + availHeight
	if endIdx > len(m.notes) {
		endIdx = len(m.notes)
	}

	for i := startIdx; i < endIdx; i++ {
		note := m.notes[i]
		isCurrent := i == m.cursor

		// Build row
		pin := " "
		if note.Pinned {
			pin = pinnedStyle.Render("P")
		}

		titleStr := note.Title
		if len(titleStr) > 40 {
			titleStr = titleStr[:37] + "..."
		}
		// Pad to fixed width before applying color so ANSI escapes don't break alignment
		paddedTitle := fmt.Sprintf("%-40s", titleStr)

		tags := ""
		if len(note.Tags) > 0 {
			tags = " [" + strings.Join(note.Tags, ", ") + "]"
		}

		linked := ""
		if len(note.LinkedTasks) > 0 {
			linked = fmt.Sprintf(" (%d)", len(note.LinkedTasks))
		}

		age := formatNoteAge(note.ModifiedAt)

		if note.Pinned {
			paddedTitle = pinnedStyle.Render(paddedTitle)
		}

		row := fmt.Sprintf("%s %s %8s%s%s", pin, paddedTitle, age, tags, linked)

		if isCurrent {
			b.WriteString(TaskJumperSelectedStyle.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	// Status message
	if m.confirmDelete {
		b.WriteString("\n")
		if m.pendingDeleteIdx < len(m.notes) {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Press d again to delete: %s", m.notes[m.pendingDeleteIdx].Title)))
		}
	} else if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(SuccessStyle.Render(m.statusMessage))
	}

	// Footer help
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("j/k: nav  Enter: view  e: edit  n: new  d: delete  u: undo  l: link  p: pin  t: tags  T: filter tags  /: search  ?: help"))

	return b.String()
}

func (m *NotesViewModel) viewCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	if !external.GlowAvailable() {
		m.statusMessage = "glow not available for viewing"
		return nil
	}

	content := fmt.Sprintf("# %s\n\n", note.Title)
	if len(note.Tags) > 0 {
		content += fmt.Sprintf("Tags: %s\n\n", strings.Join(note.Tags, ", "))
	}
	content += note.Body

	tmpFile, err := os.CreateTemp("", "tt-note-*.md")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		return NotesRefreshMsg{}
	})
}

func (m *NotesViewModel) editCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "code"
	}

	path := storage.NoteFilePath(m.project, note.Filename)

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", path)
	} else {
		c = exec.Command(editor, path)
	}

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return NotesRefreshMsg{}
	})
}

func (m *NotesViewModel) createNote() tea.Cmd {
	return func() tea.Msg {
		return ShowNoteCreatorMsg{Project: m.project}
	}
}

func (m *NotesViewModel) deleteCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}

	m.confirmDelete = true
	m.pendingDeleteIdx = m.cursor
	return nil
}

func (m *NotesViewModel) performDelete() tea.Cmd {
	if m.pendingDeleteIdx >= len(m.notes) {
		return nil
	}
	note := m.notes[m.pendingDeleteIdx]

	if err := storage.DeleteNote(m.project, note.Filename); err != nil {
		m.statusMessage = "Failed to delete: " + err.Error()
		return nil
	}

	m.lastDeleted = note
	m.statusMessage = "Deleted: " + note.Title
	m.loadNotes()
	return nil
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	if m.lastDeleted == nil {
		m.statusMessage = "Nothing to undo"
		return nil
	}

	if err := storage.SaveNote(m.project, m.lastDeleted); err != nil {
		m.statusMessage = "Failed to restore: " + err.Error()
		return nil
	}

	m.statusMessage = "Restored: " + m.lastDeleted.Title
	m.lastDeleted = nil
	m.loadNotes()
	return nil
}

func (m *NotesViewModel) linkCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	return func() tea.Msg {
		return ShowNoteTaskLinkerMsg{Project: m.project, Note: note}
	}
}

func (m *NotesViewModel) showTagSelector() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	return func() tea.Msg {
		return ShowNoteTagSelectorMsg{
			Note:        note,
			Project:     m.project,
			CurrentTags: note.Tags,
		}
	}
}

func (m *NotesViewModel) togglePin() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	note := m.notes[m.cursor]
	note.Pinned = !note.Pinned
	note.ModifiedAt = time.Now()
	if err := storage.SaveNote(m.project, note); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
		return
	}
	if note.Pinned {
		m.statusMessage = "Pinned: " + note.Title
	} else {
		m.statusMessage = "Unpinned: " + note.Title
	}
	m.loadNotes()
}

// GetHelpItems returns keybinding help for the notes view.
func (m *NotesViewModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Description: "Navigate up/down"},
		{Key: "Enter", Description: "View note (glow)"},
		{Key: "e", Description: "Edit note"},
		{Key: "n", Description: "Create new note"},
		{Key: "d", Description: "Delete note (confirm)"},
		{Key: "u", Description: "Undo last delete"},
		{Key: "l", Description: "Link note to task"},
		{Key: "p", Description: "Toggle pin"},
		{Key: "t", Description: "Edit note tags"},
		{Key: "T", Description: "Filter by tags"},
		{Key: "/", Description: "Search notes"},
		{Key: "g/G", Description: "Jump to top/bottom"},
		{Key: "q", Description: "Close notes view"},
	}
}

func formatNoteAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
