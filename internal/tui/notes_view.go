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

	// Fullscreen pane toggle
	fullscreenPane bool

	// Scroll indicator (set by renderDetailsPane)
	detailsMaxScroll int

	// Cached tasks for displaying linked task titles
	tasksLoaded bool
	cachedTasks []*models.Task

	statusMessage string
	statusIsError bool
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
	// Load project tasks once for displaying linked task titles
	if !m.tasksLoaded {
		if loadErr := storage.LoadProject(m.project); loadErr == nil {
			m.cachedTasks = m.project.Tasks
		}
		m.tasksLoaded = true
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
			m.statusMessage = "Delete canceled"
			return m, nil
		}

		m.statusMessage = ""
		m.statusIsError = false

		// Handle pending 'g' prefix key — always consume the second keystroke
		if m.pendingGKey {
			m.pendingGKey = false
			if key == "X" && m.multiSelectMode {
				m.deselectAllNotes()
				return m, nil
			}
			// Any other key after 'g' means jump to top
			m.cursor = 0
			m.detailsScroll = 0
			return m, nil
		}

		// Tab toggles focus between list and details pane
		if key == "tab" {
			m.detailsPaneFocused = !m.detailsPaneFocused
			if m.detailsPaneFocused {
				m.detailsScroll = 0
			}
			return m, nil
		}

		// Z toggles fullscreen (no-op during rename/delete/multiselect)
		if key == "Z" && !m.renaming && !m.confirmDelete && !m.multiSelectMode {
			m.fullscreenPane = !m.fullscreenPane
			return m, nil
		}

		// Esc exits fullscreen before anything else
		if key == "esc" && m.fullscreenPane {
			m.fullscreenPane = false
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
		m.tasksLoaded = false
		m.loadNotes()
	case NoteTaskLinkedMsg:
		m.loadNotes()
		m.statusMessage = "Linked note to task"
	}
	return m, nil
}

// ShouldCaptureEsc returns true when delete confirmation or multi-select is active.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirmDelete || m.detailsPaneFocused || m.multiSelectMode || m.renaming || m.fullscreenPane
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming
}

func (m *NotesViewModel) View() string {
	var b strings.Builder

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

	// Fullscreen mode: show only the focused pane using full usable height
	usable := m.height - 6
	if usable < 4 {
		usable = 4
	}

	if m.fullscreenPane && m.detailsPaneFocused {
		// Fullscreen details pane
		fullHeight := usable
		var detailsContent string
		if len(m.notes) > 0 && m.cursor < len(m.notes) {
			detailsContent = m.renderDetailsPane(m.notes[m.cursor], m.width, fullHeight)
		}
		if detailsContent != "" {
			b.WriteString(detailsContent)
			detailLines := strings.Count(detailsContent, "\n") + 1
			for i := detailLines; i < fullHeight; i++ {
				b.WriteString("\n")
			}
		} else {
			for i := 0; i < fullHeight; i++ {
				b.WriteString("\n")
			}
		}
		b.WriteString(m.renderStatusBar())
	} else if m.fullscreenPane && !m.detailsPaneFocused {
		// Fullscreen list pane: temporarily override listHeight
		origListHeight := m.listHeight
		m.listHeight = usable
		b.WriteString(m.renderListPane())
		m.listHeight = origListHeight
		b.WriteString(m.renderFooter())
		b.WriteString("\n")
		b.WriteString(m.renderStatusBar())
	} else {
		// Normal split view
		b.WriteString(m.renderListPane())
		b.WriteString(m.renderFooter())
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

		b.WriteString(m.renderDivider())
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

		b.WriteString(m.renderStatusBar())
	}
	return b.String()
}

func (m *NotesViewModel) renderListPane() string {
	if len(m.notes) == 0 {
		return MutedStyle.Render("No notes yet. Press 'n' to create one.") + "\n"
	}

	var b strings.Builder
	pinnedStyle := lipgloss.NewStyle().Foreground(ColorSuccess)

	listHeight := m.listHeight
	if listHeight < 1 {
		listHeight = 10
	}
	noteListHeight := listHeight - 1 // -1 for footer in list pane
	if noteListHeight < 1 {
		noteListHeight = 1
	}

	startIdx := 0
	if m.cursor >= noteListHeight {
		startIdx = m.cursor - noteListHeight + 1
	}
	endIdx := startIdx + noteListHeight
	if endIdx > len(m.notes) {
		endIdx = len(m.notes)
	}

	selectedStyle := TaskJumperSelectedStyle
	if m.detailsPaneFocused {
		selectedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	}

	for i := startIdx; i < endIdx; i++ {
		note := m.notes[i]
		isCurrent := i == m.cursor

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
			maxTitle = 36
		}
		if len(titleStr) > maxTitle {
			titleStr = titleStr[:maxTitle-3] + "..."
		}
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
	// Pad remaining list lines
	rendered := endIdx - startIdx
	for i := rendered; i < noteListHeight; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m *NotesViewModel) renderFooter() string {
	if m.renaming {
		return MutedStyle.Render("Enter: confirm  Esc: cancel")
	}
	if m.detailsPaneFocused {
		zHint := "Z:fullscreen"
		if m.fullscreenPane {
			zHint = "Z:exit fullscreen"
		}
		return MutedStyle.Render("j/k: scroll  g/G: top/bottom  Ctrl+d/u: half page  Tab/Esc: list pane  Enter: view  e: edit  " + zHint + "  q: close")
	}
	if m.multiSelectMode {
		return MutedStyle.Render("space: toggle  a: all  gX: clear  d: delete  t: tag  l: link  y: copy  V/Esc: exit multi-select")
	}
	zHint := "Z:fullscreen"
	if m.fullscreenPane {
		zHint = "Z:exit fullscreen"
	}
	footer := "j/k: nav  Tab: pane  e: edit  n: new  d: del  r: rename  s: sort  y: copy  " + zHint + "  /: search"
	if len(m.filterTags) > 0 {
		footer += "  C: clear filter"
	}
	return MutedStyle.Render(footer)
}

func (m *NotesViewModel) renderDivider() string {
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
	if m.detailsMaxScroll > 0 && len(m.notes) > 0 && m.cursor < len(m.notes) {
		indicator := fmt.Sprintf(" [%d/%d] ", m.detailsScroll+1, m.detailsMaxScroll+1)
		if len(indicator) < dividerWidth {
			dividerLine = strings.Repeat("─", dividerWidth-len(indicator)) + indicator
		}
	}
	return dividerStyle.Render(dividerLine)
}

func (m *NotesViewModel) renderStatusBar() string {
	if m.renaming {
		return "Rename: " + m.renameInput.View()
	}
	if m.confirmDelete {
		if m.pendingDeleteIdx < len(m.notes) {
			return lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("Press d again to delete: %s", m.notes[m.pendingDeleteIdx].Title))
		}
		return ""
	}
	if m.statusMessage != "" {
		if m.statusIsError {
			return lipgloss.NewStyle().Foreground(ColorWarning).Render(m.statusMessage)
		}
		return SuccessStyle.Render(m.statusMessage)
	}
	return ""
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

	// Linked tasks
	if len(note.LinkedTasks) > 0 {
		for _, taskID := range note.LinkedTasks {
			taskTitle := "[deleted]"
			if task := m.project.GetTaskByID(taskID); task != nil {
				taskTitle = task.Title
			}
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  → %s", taskTitle)))
		}
	}

	lines = append(lines, "")

	// Body: wrap lines to width
	if note.Body != "" {
		bodyLines := strings.Split(note.Body, "\n")
		for _, bl := range bodyLines {
			if len(bl) <= width && width > 0 {
				lines = append(lines, bl)
			} else if width > 0 {
				// Word wrap; forces break when no space found (e.g. URLs)
				for len(bl) > width {
					cut := width
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

	configEditor := ""
	if m.config != nil {
		configEditor = m.config.GetEditor()
	}
	editor := external.ResolveEditor(configEditor)
	path := storage.NoteFilePath(m.project, note.Filename)
	c := external.EditorCommand(editor, path)

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
	return func() tea.Msg { return NotesRefreshMsg{} }
}

// batchSaveNotes saves a slice of notes and returns the count of failures.
func (m *NotesViewModel) batchSaveNotes(notes []*models.Note) int {
	var failed int
	for _, note := range notes {
		if err := storage.SaveNote(m.project, note); err != nil {
			failed++
		}
	}
	return failed
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	// Priority: delete > tag > link
	if m.lastBulkDeleted != nil {
		count := len(m.lastBulkDeleted)
		failed := m.batchSaveNotes(m.lastBulkDeleted)
		m.lastBulkDeleted = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Restored %d notes (%d failed)", count-failed, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Restored %d notes", count)
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
		notes := make([]*models.Note, len(m.lastTagUndo))
		for i, entry := range m.lastTagUndo {
			entry.Note.Tags = entry.OldTags
			entry.Note.ModifiedAt = time.Now()
			notes[i] = entry.Note
		}
		count := len(m.lastTagUndo)
		failed := m.batchSaveNotes(notes)
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
		notes := make([]*models.Note, len(m.lastLinkUndo))
		for i, entry := range m.lastLinkUndo {
			entry.Note.LinkedTasks = entry.OldLinks
			entry.Note.ModifiedAt = time.Now()
			notes[i] = entry.Note
		}
		count := len(m.lastLinkUndo)
		failed := m.batchSaveNotes(notes)
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
	return func() tea.Msg { return NotesRefreshMsg{} }
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
			m.statusMessage = "Rename canceled: empty title"
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
			m.statusMessage = "Rename canceled: same title"
			return m, nil
		}
		_, err := storage.RenameNote(m.project, note.Filename, newTitle)
		if err != nil {
			m.statusMessage = "Rename failed: " + err.Error()
			m.renaming = false
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("Renamed: %s → %s", oldTitle, newTitle)
		m.loadNotes()
		m.renaming = false
		return m, func() tea.Msg { return NotesRefreshMsg{} }
	case "esc":
		m.renaming = false
		m.statusMessage = "Rename canceled"
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// copyNoteContent is a generic helper for clipboard copy operations. It handles
// both single-note and bulk (multi-select) copying, reducing duplication across
// the three copy commands.
func (m *NotesViewModel) copyNoteContent(
	singleFn func(*models.Note) (string, error),
	bulkFn func([]*models.Note) (string, error),
	singleMsg, bulkMsg string,
) tea.Cmd {
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := bulkFn(selected)
		if err != nil {
			m.statusMessage = "Copy failed: " + err.Error()
			m.statusIsError = true
		} else {
			m.statusMessage = fmt.Sprintf(bulkMsg, len(selected))
			m.statusIsError = false
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := singleFn(m.notes[m.cursor])
	if err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		m.statusIsError = true
	} else {
		m.statusMessage = singleMsg
		m.statusIsError = false
	}
	return nil
}

func (m *NotesViewModel) copyNoteTitle() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteTitle, external.CopyNoteTitles,
		"Copied title to clipboard", "Copied %d titles to clipboard",
	)
}

func (m *NotesViewModel) copyNoteBody() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteBody, external.CopyNoteBodies,
		"Copied body to clipboard", "Copied %d note bodies to clipboard",
	)
}

func (m *NotesViewModel) copyNoteTitleAndBody() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteTitleAndBody, external.CopyNoteTitlesAndBodies,
		"Copied title and body to clipboard", "Copied %d notes to clipboard",
	)
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
		{Key: "Z", Description: "Toggle fullscreen pane"},
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
