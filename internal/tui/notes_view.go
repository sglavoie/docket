package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
	lastDeleted     *models.Note
	lastBulkDeleted []*models.Note
	lastTagUndo     []noteTagUndoEntry
	lastLinkUndo    []noteLinkUndoEntry

	// Details pane state
	detailsPaneFocused bool
	detailsScroll      int
	listHeight         int
	detailsHeight      int

	// Multi-select state
	multiSelectMode bool
	selectedNotes   map[string]bool // keyed by filename
	pendingGKey     bool            // waiting for second key after 'g'

	// Sort state
	sortMode noteSortMode

	// Rename state
	renaming    bool
	renameInput textinput.Model

	// Scroll indicator (set by renderDetailsPane)
	detailsMaxScroll int

	statusMessage string
}

// noteSortMode defines how notes are sorted.
type noteSortMode int

const (
	noteSortModified noteSortMode = iota
	noteSortCreated
	noteSortTitleAZ
	noteSortTitleZA
)

func (s noteSortMode) String() string {
	switch s {
	case noteSortModified:
		return "modified"
	case noteSortCreated:
		return "created"
	case noteSortTitleAZ:
		return "title A-Z"
	case noteSortTitleZA:
		return "title Z-A"
	default:
		return "modified"
	}
}

// noteTagUndoEntry stores a note's previous tags for undo.
type noteTagUndoEntry struct {
	Note    *models.Note
	OldTags []string
}

// noteLinkUndoEntry stores a note's previous links for undo.
type noteLinkUndoEntry struct {
	Note     *models.Note
	OldLinks []string
}

// notesConfig is a minimal interface for config access.
type notesConfig interface {
	GetEditor() string
}

// NewNotesViewModel creates a new notes view.
func NewNotesViewModel(project *models.Project, cfg ...notesConfig) *NotesViewModel {
	m := &NotesViewModel{
		project:       project,
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
	}
	if len(cfg) > 0 && cfg[0] != nil {
		m.config = cfg[0]
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
	// Apply sort mode (pinned notes always first)
	m.sortNotes(notes)
	m.notes = notes
	if m.cursor >= len(m.notes) {
		m.cursor = max(0, len(m.notes)-1)
	}
}

func (m *NotesViewModel) sortNotes(notes []*models.Note) {
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].Pinned != notes[j].Pinned {
			return notes[i].Pinned
		}
		switch m.sortMode {
		case noteSortCreated:
			return notes[i].CreatedAt.After(notes[j].CreatedAt)
		case noteSortTitleAZ:
			return strings.ToLower(notes[i].Title) < strings.ToLower(notes[j].Title)
		case noteSortTitleZA:
			return strings.ToLower(notes[i].Title) > strings.ToLower(notes[j].Title)
		default: // noteSortModified
			return notes[i].ModifiedAt.After(notes[j].ModifiedAt)
		}
	})
}

func (m *NotesViewModel) Init() tea.Cmd {
	return nil
}

func (m *NotesViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Split: top half for list, 1 for divider, bottom half for details
	// Reserve 4 lines for title/count header and 2 for footer
	usable := height - 6
	if usable < 4 {
		usable = 4
	}
	m.listHeight = usable / 2
	m.detailsHeight = usable - m.listHeight - 1 // -1 for divider
}

// Width returns the view width.
func (m *NotesViewModel) Width() int { return m.width }

// Height returns the view height.
func (m *NotesViewModel) Height() int { return m.height }

func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle rename input mode
		if m.renaming {
			return m.handleRenameKey(msg)
		}

		// Handle delete confirmation mode
		if m.confirmDelete {
			m.confirmDelete = false
			if key == "d" {
				if m.multiSelectMode && m.selectedNoteCount() > 0 {
					return m, m.performBulkDelete()
				}
				if m.pendingDeleteIdx < len(m.notes) {
					return m, m.performDelete()
				}
				m.statusMessage = "Delete failed: note no longer exists"
				return m, nil
			}
			m.statusMessage = "Delete cancelled"
			return m, nil
		}

		m.statusMessage = ""

		// Handle pending 'g' prefix key
		if m.pendingGKey {
			m.pendingGKey = false
			if key == "X" && m.multiSelectMode {
				m.deselectAllNotes()
				return m, nil
			}
			// 'g' alone means jump to top
			m.cursor = 0
			m.detailsScroll = 0
			// If the second key wasn't consumed, don't process it further
			if key == "X" {
				return m, nil
			}
			// Fall through only if the key wasn't 'X' — let the key be handled normally below
		}

		// Tab toggles focus between list and details pane
		if key == "tab" {
			m.detailsPaneFocused = !m.detailsPaneFocused
			if m.detailsPaneFocused {
				m.detailsScroll = 0
			}
			return m, nil
		}

		// Multi-select exit takes priority over details pane routing
		if m.multiSelectMode && (key == "esc" || key == "q") {
			m.multiSelectMode = false
			m.selectedNotes = make(map[string]bool)
			m.statusMessage = ""
			return m, nil
		}

		// Route keys when details pane is focused
		if m.detailsPaneFocused {
			return m.handleDetailsPaneKey(key)
		}

		switch key {
		case "q", "esc":
			return m, func() tea.Msg { return CloseNotesMsg{} }
		case "j", "down":
			if m.cursor < len(m.notes)-1 {
				m.cursor++
				m.detailsScroll = 0
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.detailsScroll = 0
			}
		case "g":
			m.pendingGKey = true
			return m, nil
		case "G":
			if len(m.notes) > 0 {
				m.cursor = len(m.notes) - 1
				m.detailsScroll = 0
			}
		case "V":
			m.toggleNoteMultiSelectMode()
			return m, nil
		case " ":
			if m.multiSelectMode {
				m.toggleNoteSelection()
				// Move cursor down after toggle
				if m.cursor < len(m.notes)-1 {
					m.cursor++
					m.detailsScroll = 0
				}
				return m, nil
			}
		case "a":
			if m.multiSelectMode {
				m.selectAllNotes()
				return m, nil
			}
		case "enter":
			return m, m.viewCurrentNote()
		case "e":
			return m, m.editCurrentNote()
		case "n":
			return m, m.createNote()
		case "d":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				m.confirmDelete = true
				m.statusMessage = fmt.Sprintf("Press d again to delete %d notes", m.selectedNoteCount())
				return m, nil
			}
			return m, m.deleteCurrentNote()
		case "l":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkLinkNotes()
			}
			return m, m.linkCurrentNote()
		case "u":
			return m, m.undoDelete()
		case "p":
			m.togglePin()
		case "s":
			m.cycleSortMode()
			return m, nil
		case "y":
			return m, m.copyNoteTitle()
		case "Y":
			return m, m.copyNoteBody()
		case "ctrl+y":
			return m, m.copyNoteTitleAndBody()
		case "r":
			m.startRename()
			return m, nil
		case "t":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkTagNotes()
			}
			return m, m.showTagSelector()
		case "T":
			return m, func() tea.Msg {
				return ShowNoteTagFilterSelectorMsg{Project: m.project}
			}
		case "C":
			if len(m.filterTags) > 0 {
				m.filterTags = make(map[string]bool)
				m.loadNotes()
				m.statusMessage = "Filter cleared"
				return m, nil
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

// ShouldCaptureEsc returns true when delete confirmation or multi-select is active.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirmDelete || m.detailsPaneFocused || m.multiSelectMode || m.renaming
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming
}

func (m *NotesViewModel) View() string {
	var b strings.Builder

	pinnedStyle := lipgloss.NewStyle().Foreground(ColorSuccess)

	// Title
	title := fmt.Sprintf("%s - Notes", m.project.Name)
	if len(m.filterTags) > 0 {
		title += " [filtered]"
	}
	b.WriteString(TitleStyle.Render(title))
	if len(m.filterTags) > 0 {
		tags := make([]string, 0, len(m.filterTags))
		for t := range m.filterTags {
			tags = append(tags, t)
		}
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [tags: %s]", strings.Join(tags, ", "))))
	}
	b.WriteString("\n")

	// Note count and sort mode
	countInfo := fmt.Sprintf("%d notes", len(m.notes))
	if m.sortMode != noteSortModified {
		countInfo += fmt.Sprintf(" (sort: %s)", m.sortMode)
	}
	b.WriteString(MutedStyle.Render(countInfo))
	b.WriteString("\n\n")

	if len(m.notes) == 0 {
		b.WriteString(MutedStyle.Render("No notes yet. Press 'n' to create one."))
		b.WriteString("\n")
	}

	// Calculate visible area for the list pane
	// Reserve 1 line at the bottom of the list area for the footer help
	listHeight := m.listHeight
	if listHeight < 1 {
		listHeight = 10
	}
	noteListHeight := listHeight - 1 // -1 for footer in list pane
	if noteListHeight < 1 {
		noteListHeight = 1
	}

	// Scrolling window
	startIdx := 0
	if m.cursor >= noteListHeight {
		startIdx = m.cursor - noteListHeight + 1
	}
	endIdx := startIdx + noteListHeight
	if endIdx > len(m.notes) {
		endIdx = len(m.notes)
	}

	// Dim the selected style when details pane is focused
	selectedStyle := TaskJumperSelectedStyle
	if m.detailsPaneFocused {
		selectedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	}

	for i := startIdx; i < endIdx; i++ {
		note := m.notes[i]
		isCurrent := i == m.cursor

		// Build row
		checkbox := ""
		if m.multiSelectMode {
			if m.isNoteSelected(note.Filename) {
				checkbox = "[x] "
			} else {
				checkbox = "[ ] "
			}
		}

		pin := " "
		if note.Pinned {
			pin = pinnedStyle.Render("P")
		}

		titleStr := note.Title
		maxTitle := 40
		if m.multiSelectMode {
			maxTitle = 36 // account for checkbox width
		}
		if len(titleStr) > maxTitle {
			titleStr = titleStr[:maxTitle-3] + "..."
		}
		// Pad to fixed width before applying color so ANSI escapes don't break alignment
		paddedTitle := fmt.Sprintf("%-40s", checkbox+titleStr)

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
			b.WriteString(selectedStyle.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}
	// Pad remaining list lines if list is shorter than noteListHeight
	rendered := endIdx - startIdx
	for i := rendered; i < noteListHeight; i++ {
		b.WriteString("\n")
	}

	// Footer help (rendered in the list pane to maximize details pane space)
	if m.renaming {
		b.WriteString(MutedStyle.Render("Enter: confirm  Esc: cancel"))
	} else if m.detailsPaneFocused {
		b.WriteString(MutedStyle.Render("j/k: scroll  g/G: top/bottom  Ctrl+d/u: half page  Tab/Esc: list pane  Enter: view  e: edit  q: close"))
	} else if m.multiSelectMode {
		b.WriteString(MutedStyle.Render("space: toggle  a: all  gX: clear  d: delete  t: tag  l: link  y: copy  V/Esc: exit multi-select"))
	} else {
		footer := "j/k: nav  Tab: pane  e: edit  n: new  d: del  r: rename  s: sort  y: copy  p: pin  t: tags  T: filter  V: multi  /: search"
		if len(m.filterTags) > 0 {
			footer += "  C: clear filter"
		}
		b.WriteString(MutedStyle.Render(footer))
	}
	b.WriteString("\n")

	// Details pane (render first so scroll values are clamped before building divider)
	detailsHeight := m.detailsHeight
	if detailsHeight < 1 {
		detailsHeight = 5
	}
	var detailsContent string
	if len(m.notes) > 0 && m.cursor < len(m.notes) {
		detailsContent = m.renderDetailsPane(m.notes[m.cursor], m.width, detailsHeight)
	}

	// Divider
	dividerColor := ColorMuted
	if m.detailsPaneFocused {
		dividerColor = ColorPrimary
	}
	dividerStyle := lipgloss.NewStyle().Foreground(dividerColor)
	dividerWidth := m.width
	if dividerWidth <= 0 {
		dividerWidth = 80
	}
	dividerLine := strings.Repeat("─", dividerWidth)
	// Add scroll position indicator when content overflows
	if m.detailsMaxScroll > 0 && len(m.notes) > 0 && m.cursor < len(m.notes) {
		indicator := fmt.Sprintf(" [%d/%d] ", m.detailsScroll+1, m.detailsMaxScroll+1)
		if len(indicator) < dividerWidth {
			dividerLine = strings.Repeat("─", dividerWidth-len(indicator)) + indicator
		}
	}
	b.WriteString(dividerStyle.Render(dividerLine))
	b.WriteString("\n")

	// Output pre-rendered details
	if detailsContent != "" {
		b.WriteString(detailsContent)
		detailLines := strings.Count(detailsContent, "\n") + 1
		for i := detailLines; i < detailsHeight; i++ {
			b.WriteString("\n")
		}
	} else {
		for i := 0; i < detailsHeight; i++ {
			b.WriteString("\n")
		}
	}

	// Status message
	if m.renaming {
		b.WriteString("Rename: ")
		b.WriteString(m.renameInput.View())
	} else if m.confirmDelete {
		if m.pendingDeleteIdx < len(m.notes) {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Press d again to delete: %s", m.notes[m.pendingDeleteIdx].Title)))
		}
	} else if m.statusMessage != "" {
		b.WriteString(SuccessStyle.Render(m.statusMessage))
	}

	return b.String()
}

func (m *NotesViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	halfPage := m.detailsHeight / 2
	if halfPage < 1 {
		halfPage = 1
	}
	switch key {
	case "j", "down":
		m.detailsScroll++
	case "k", "up":
		if m.detailsScroll > 0 {
			m.detailsScroll--
		}
	case "ctrl+d":
		m.detailsScroll += halfPage
	case "ctrl+u":
		m.detailsScroll -= halfPage
		if m.detailsScroll < 0 {
			m.detailsScroll = 0
		}
	case "g":
		m.detailsScroll = 0
	case "G":
		// Scroll to bottom; renderDetailsPane will clamp to maxScroll
		m.detailsScroll = 99999
	case "esc":
		m.detailsPaneFocused = false
	case "q":
		return m, func() tea.Msg { return CloseNotesMsg{} }
	case "e":
		return m, m.editCurrentNote()
	case "enter":
		return m, m.viewCurrentNote()
	}
	return m, nil
}

func (m *NotesViewModel) renderDetailsPane(note *models.Note, width, height int) string {
	if note == nil || height <= 0 {
		return ""
	}

	var lines []string

	// Title
	titleLine := TitleStyle.Render(note.Title)
	lines = append(lines, titleLine)

	// Tags
	if len(note.Tags) > 0 {
		lines = append(lines, MutedStyle.Render("Tags: "+strings.Join(note.Tags, ", ")))
	}

	lines = append(lines, "")

	// Body: wrap lines to width
	if note.Body != "" {
		bodyLines := strings.Split(note.Body, "\n")
		for _, bl := range bodyLines {
			if len(bl) <= width && width > 0 {
				lines = append(lines, bl)
			} else if width > 0 {
				// Simple word wrap
				for len(bl) > width {
					cut := width
					// Try to break at space
					if idx := strings.LastIndex(bl[:cut], " "); idx > 0 {
						cut = idx + 1
					}
					lines = append(lines, bl[:cut])
					bl = bl[cut:]
				}
				lines = append(lines, bl)
			}
		}
	}

	// Clamp scroll so the last line is always visible at the bottom
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.detailsMaxScroll = maxScroll
	if m.detailsScroll > maxScroll {
		m.detailsScroll = maxScroll
	}
	if m.detailsScroll < 0 {
		m.detailsScroll = 0
	}

	// Apply scroll offset and cap to available height
	lines = lines[m.detailsScroll:]
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
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
	_, writeErr := tmpFile.WriteString(content)
	closeErr := tmpFile.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil
	}

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

	editor := ""
	if m.config != nil {
		editor = m.config.GetEditor()
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
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
	m.lastBulkDeleted = nil
	m.statusMessage = "Deleted: " + note.Title
	m.loadNotes()
	return nil
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	// Priority: delete > tag > link
	if m.lastBulkDeleted != nil {
		var restored, failed int
		for _, note := range m.lastBulkDeleted {
			if err := storage.SaveNote(m.project, note); err != nil {
				failed++
			} else {
				restored++
			}
		}
		m.lastBulkDeleted = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Restored %d notes (%d failed)", restored, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Restored %d notes", restored)
		}
		m.loadNotes()
		return nil
	}

	if m.lastDeleted != nil {
		if err := storage.SaveNote(m.project, m.lastDeleted); err != nil {
			m.statusMessage = "Failed to restore: " + err.Error()
			return nil
		}
		m.statusMessage = "Restored: " + m.lastDeleted.Title
		m.lastDeleted = nil
		m.loadNotes()
		return nil
	}

	if len(m.lastTagUndo) > 0 {
		var failed int
		for _, entry := range m.lastTagUndo {
			entry.Note.Tags = entry.OldTags
			entry.Note.ModifiedAt = time.Now()
			if err := storage.SaveNote(m.project, entry.Note); err != nil {
				failed++
			}
		}
		count := len(m.lastTagUndo)
		m.lastTagUndo = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Undid tags on %d notes (%d failed)", count, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Undid tags on %d notes", count)
		}
		m.loadNotes()
		return nil
	}

	if len(m.lastLinkUndo) > 0 {
		var failed int
		for _, entry := range m.lastLinkUndo {
			entry.Note.LinkedTasks = entry.OldLinks
			entry.Note.ModifiedAt = time.Now()
			if err := storage.SaveNote(m.project, entry.Note); err != nil {
				failed++
			}
		}
		count := len(m.lastLinkUndo)
		m.lastLinkUndo = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Undid links on %d notes (%d failed)", count, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Undid links on %d notes", count)
		}
		m.loadNotes()
		return nil
	}

	m.statusMessage = "Nothing to undo"
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

func (m *NotesViewModel) performBulkDelete() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}

	var deleted []*models.Note
	var errors int
	for _, note := range selected {
		if err := storage.DeleteNote(m.project, note.Filename); err == nil {
			deleted = append(deleted, note)
		} else {
			errors++
		}
	}

	m.selectedNotes = make(map[string]bool)
	m.multiSelectMode = false

	if errors > 0 {
		m.statusMessage = fmt.Sprintf("Deleted %d/%d notes (%d failed)", len(deleted), len(selected), errors)
	} else {
		m.statusMessage = fmt.Sprintf("Deleted %d notes", len(deleted))
	}

	m.lastBulkDeleted = deleted
	m.lastDeleted = nil
	m.loadNotes()
	return nil
}

func (m *NotesViewModel) bulkTagNotes() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}
	// Use first note to open tag selector; results applied to all via BulkNoteTagDoneMsg
	return func() tea.Msg {
		return BulkNoteTagMsg{
			Notes:   selected,
			Project: m.project,
		}
	}
}

func (m *NotesViewModel) bulkLinkNotes() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}
	return func() tea.Msg {
		return BulkNoteLinkMsg{
			Notes:   selected,
			Project: m.project,
		}
	}
}

func (m *NotesViewModel) cycleSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	m.loadNotes()
	m.statusMessage = fmt.Sprintf("Sort: %s", m.sortMode)
}

func (m *NotesViewModel) startRename() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	ti := textinput.New()
	ti.SetValue(m.notes[m.cursor].Title)
	ti.CharLimit = 100
	ti.Width = 50
	ti.Focus()
	m.renameInput = ti
	m.renaming = true
}

func (m *NotesViewModel) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newTitle := strings.TrimSpace(m.renameInput.Value())
		if newTitle == "" {
			m.statusMessage = "Rename cancelled: empty title"
			m.renaming = false
			return m, nil
		}
		if m.cursor >= len(m.notes) {
			m.renaming = false
			return m, nil
		}
		note := m.notes[m.cursor]
		oldTitle := note.Title
		if newTitle == oldTitle {
			m.renaming = false
			m.statusMessage = "Rename cancelled: same title"
			return m, nil
		}
		_, err := storage.RenameNote(m.project, note.Filename, newTitle)
		if err != nil {
			m.statusMessage = "Rename failed: " + err.Error()
		} else {
			m.statusMessage = fmt.Sprintf("Renamed: %s → %s", oldTitle, newTitle)
			m.loadNotes()
		}
		m.renaming = false
		return m, nil
	case "esc":
		m.renaming = false
		m.statusMessage = "Rename cancelled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

func (m *NotesViewModel) copyNoteTitle() tea.Cmd {
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := external.CopyNoteTitles(selected)
		if err != nil {
			m.statusMessage = "Copy failed: " + err.Error()
		} else {
			m.statusMessage = fmt.Sprintf("Copied %d titles to clipboard", len(selected))
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := external.CopyNoteTitle(m.notes[m.cursor])
	if err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
	} else {
		m.statusMessage = "Copied title to clipboard"
	}
	return nil
}

func (m *NotesViewModel) copyNoteBody() tea.Cmd {
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := external.CopyNoteBodies(selected)
		if err != nil {
			m.statusMessage = "Copy failed: " + err.Error()
		} else {
			m.statusMessage = fmt.Sprintf("Copied %d note bodies to clipboard", len(selected))
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := external.CopyNoteBody(m.notes[m.cursor])
	if err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
	} else {
		m.statusMessage = "Copied body to clipboard"
	}
	return nil
}

func (m *NotesViewModel) copyNoteTitleAndBody() tea.Cmd {
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := external.CopyNoteTitlesAndBodies(selected)
		if err != nil {
			m.statusMessage = "Copy failed: " + err.Error()
		} else {
			m.statusMessage = fmt.Sprintf("Copied %d notes to clipboard", len(selected))
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := external.CopyNoteTitleAndBody(m.notes[m.cursor])
	if err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
	} else {
		m.statusMessage = "Copied title and body to clipboard"
	}
	return nil
}

// GetHelpItems returns keybinding help for the notes view.
func (m *NotesViewModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Description: "Navigate up/down"},
		{Key: "Tab", Description: "Switch pane (list/details)"},
		{Key: "Enter", Description: "View note (glow)"},
		{Key: "e", Description: "Edit note"},
		{Key: "n", Description: "Create new note"},
		{Key: "d", Description: "Delete note (confirm)"},
		{Key: "u", Description: "Undo last delete"},
		{Key: "r", Description: "Rename note"},
		{Key: "s", Description: "Cycle sort mode"},
		{Key: "y", Description: "Copy title to clipboard"},
		{Key: "Y", Description: "Copy body to clipboard"},
		{Key: "Ctrl+y", Description: "Copy title and body to clipboard"},
		{Key: "l", Description: "Link note to task"},
		{Key: "p", Description: "Toggle pin"},
		{Key: "t", Description: "Edit note tags"},
		{Key: "T", Description: "Filter by tags"},
		{Key: "C", Description: "Clear tag filter"},
		{Key: "/", Description: "Search notes"},
		{Key: "g/G", Description: "Jump to top/bottom"},
		{Key: "V", Description: "Toggle multi-select mode"},
		{Key: "space", Description: "Toggle selection (multi-select)"},
		{Key: "a", Description: "Select all (multi-select)"},
		{Key: "gX", Description: "Deselect all (multi-select)"},
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
