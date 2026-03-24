package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ShowNotesSearchMsg opens the notes search view.
type ShowNotesSearchMsg struct {
	Project *models.Project
}

// CloseNotesSearchMsg closes the notes search view.
type CloseNotesSearchMsg struct{}

// NotesSearchSelectMsg selects a note from the search results.
type NotesSearchSelectMsg struct {
	Note    *models.Note
	Project *models.Project
}

// notesSearchResult holds a note with match positions for highlighting.
type notesSearchResult struct {
	note           *models.Note
	score          int
	titlePositions []int
	bodySnippet    string // snippet of matched body content
	bodyPositions  []int  // match positions within bodySnippet for highlighting
}

// NotesSearchModel provides search across notes in a project.
type NotesSearchModel struct {
	project     *models.Project
	tasksLoaded bool
	notes       []*models.Note

	filtered []notesSearchResult
	cursor   int

	searchInput textinput.Model

	width        int
	height       int
	listWidth    int
	previewWidth int

	stale bool // set true when notes are mutated; reloads on next activation
}

// NewNotesSearchModel creates a new notes search model.
func NewNotesSearchModel(project *models.Project) *NotesSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search notes..."
	searchInput.CharLimit = 100
	searchInput.SetWidth(40)
	searchInput.Focus()

	notes, _ := storage.ListNotes(project)

	m := &NotesSearchModel{
		project:     project,
		notes:       notes,
		searchInput: searchInput,
	}
	// Load project tasks for displaying linked task titles
	if loadErr := storage.LoadProject(project); loadErr == nil {
		m.tasksLoaded = true
	}
	m.filterNotes()
	return m
}

// MarkStale flags the search index for reload on next activation.
func (m *NotesSearchModel) MarkStale() {
	m.stale = true
}

// RefreshIfStale reloads notes from storage if the index is stale.
func (m *NotesSearchModel) RefreshIfStale() {
	if !m.stale {
		return
	}
	m.stale = false
	m.notes, _ = storage.ListNotes(m.project)
	m.filterNotes()
}

// HasTextInput returns true because search always has input focus.
func (m *NotesSearchModel) HasTextInput() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *NotesSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	innerWidth := max(width-10, 1)

	m.listWidth = max(innerWidth*50/100, 1)
	m.previewWidth = max(innerWidth-m.listWidth-1, 1)

	m.searchInput.SetWidth(max(innerWidth-4, 1))
}

// Init initializes the model.
func (m *NotesSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m *NotesSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Reload notes if they were mutated while this view was inactive
	m.RefreshIfStale()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseNotesSearchMsg{} }
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				note := m.filtered[m.cursor].note
				return m, func() tea.Msg {
					return NotesSearchSelectMsg{
						Note:    note,
						Project: m.project,
					}
				}
			}
			return m, nil
		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+u":
			jumpCursor(&m.cursor, len(m.filtered), -10)
			return m, nil
		case "ctrl+d":
			jumpCursor(&m.cursor, len(m.filtered), 10)
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterNotes()
			return m, cmd
		}
	}

	return m, nil
}

// filterNotes filters notes based on the current search query.
func (m *NotesSearchModel) filterNotes() {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		// Show all notes when no search query
		m.filtered = make([]notesSearchResult, len(m.notes))
		for i, note := range m.notes {
			m.filtered[i] = notesSearchResult{note: note, score: 1}
		}
		m.cursor = ClampCursor(m.cursor, len(m.filtered))
		return
	}

	var scored []notesSearchResult

	for _, note := range m.notes {
		titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(note.Title, query)
		if titleScore > 0 {
			titleScore *= 2 // title matches weigh more
		}

		bodyScore := scoreBodyTerms(note.Body, query)

		tagScore := 0
		for _, tag := range note.Tags {
			s := fuzzy.ScoreTerms(tag, query)
			if s > tagScore {
				tagScore = s
			}
		}

		bestScore := max(tagScore, max(bodyScore, titleScore))

		if bestScore > 0 {
			snippet := ""
			var bodyPos []int
			if bodyScore > 0 {
				snippet, bodyPos = extractSnippets(note.Body, query)
			}

			positions := titlePositions
			if titleScore <= 0 {
				positions = nil
			}

			scored = append(scored, notesSearchResult{
				note:           note,
				score:          bestScore,
				titlePositions: positions,
				bodySnippet:    snippet,
				bodyPositions:  bodyPos,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filtered = scored
	m.cursor = ClampCursor(m.cursor, len(m.filtered))
}

// scoreBodyTerms scores body text against a multi-term query using substring
// matching. The fuzzy scorer can reject valid body matches because its
// "too scattered" heuristic fires when early characters in a long text match
// before the actual contiguous occurrence. This function checks for case-
// insensitive substring containment of each term, requiring all terms to match.
func scoreBodyTerms(body, query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1
	}
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return 1
	}

	lowerBody := strings.ToLower(body)
	total := 0
	for _, term := range terms {
		found := strings.Contains(lowerBody, term)
		if !found {
			// Fall back to fuzzy for this term
			s := fuzzy.Score(body, term)
			if s == 0 {
				return 0
			}
			total += s
		} else {
			total += len(term) * 4 // base score proportional to term length
		}
	}
	return total
}

// extractSnippets extracts up to 3 snippet windows (one per distinct term match)
// and returns the joined snippet text along with rune-based highlight positions.
func extractSnippets(body, query string) (string, []int) {
	lowerBody := strings.ToLower(body)
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return "", nil
	}

	type window struct {
		start, end int
		termIdx    int // index of the matching term
	}

	// Find one window per distinct term (up to 3)
	seen := make(map[int]bool)
	var windows []window
	for ti, term := range terms {
		idx := strings.Index(lowerBody, term)
		if idx < 0 || seen[idx] {
			continue
		}
		seen[idx] = true
		start := max(idx-30, 0)
		end := min(idx+len(term)+50, len(body))
		windows = append(windows, window{start: start, end: end, termIdx: ti})
		if len(windows) >= 3 {
			break
		}
	}

	if len(windows) == 0 {
		// No exact substring match; return beginning of body
		if len(body) > 120 {
			return body[:120] + "...", nil
		}
		return body, nil
	}

	// Sort windows by position
	sort.Slice(windows, func(i, j int) bool {
		return windows[i].start < windows[j].start
	})

	// Merge overlapping windows
	merged := []window{windows[0]}
	for i := 1; i < len(windows); i++ {
		last := &merged[len(merged)-1]
		if windows[i].start <= last.end {
			if windows[i].end > last.end {
				last.end = windows[i].end
			}
		} else {
			merged = append(merged, windows[i])
		}
	}

	// Build combined snippet and compute highlight positions
	var result strings.Builder
	var positions []int

	for i, w := range merged {
		if i > 0 {
			result.WriteString(" … ")
		}

		segment := body[w.start:w.end]
		prefix := ""
		suffix := ""
		if w.start > 0 {
			prefix = "..."
		}
		if w.end < len(body) {
			suffix = "..."
		}

		segmentStart := result.Len() + len(prefix)
		result.WriteString(prefix)
		result.WriteString(segment)
		result.WriteString(suffix)

		// Find all term occurrences within this segment for highlighting
		lowerSegment := strings.ToLower(segment)
		for _, term := range terms {
			offset := 0
			for {
				idx := strings.Index(lowerSegment[offset:], term)
				if idx < 0 {
					break
				}
				absOffset := segmentStart + offset + idx
				segRunes := []rune(result.String()[:absOffset])
				runeStart := len(segRunes)
				termRunes := []rune(term)
				for ri := range termRunes {
					positions = append(positions, runeStart+ri)
				}
				offset += idx + len(term)
			}
		}
	}

	return result.String(), positions
}

// View renders the notes search view.
func (m *NotesSearchModel) View() tea.View {
	var b strings.Builder

	query := strings.TrimSpace(m.searchInput.Value())

	// Subtitle
	var subtitle string
	if query == "" {
		subtitle = fmt.Sprintf("%d notes", len(m.notes))
	} else if len(m.filtered) == 0 {
		subtitle = fmt.Sprintf("0/%d matching", len(m.notes))
	} else {
		subtitle = fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.notes))
	}

	// Title
	title := fmt.Sprintf("Notes Search - %s", m.project.Name)
	availableSubtitleWidth := max(m.width-lipgloss.Width(title)-2, 1)
	b.WriteString(renderFullscreenTitle(title, truncateToVisualWidth(subtitle, availableSubtitleWidth)))
	b.WriteString("\n\n")

	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		if query != "" {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("No notes match \"%s\".", query)))
		} else {
			b.WriteString(MutedStyle.Render("No notes found."))
		}
		b.WriteString("\n")
		left := HelpStyle.Render("Esc close")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return tea.NewView(b.String())
	}

	paneHeight := max(m.height-8, 5)

	listContent := m.renderNoteList(paneHeight)
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	b.WriteString(splitPane)
	b.WriteString("\n")

	// Footer
	left := HelpStyle.Render(fmt.Sprintf("up/down nav · Enter select · Esc close · %d results", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, "", m.width))

	return tea.NewView(b.String())
}

// renderNoteList renders the filtered note list.
func (m *NotesSearchModel) renderNoteList(paneHeight int) string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching notes.")
	}

	var b strings.Builder
	maxVisible := paneHeight

	scrollOffset := calcScrollOffset(m.cursor, maxVisible)

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		fr := m.filtered[i]
		note := fr.note

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		noteIcon := MutedStyle.Render("[N]")
		prefix := fmt.Sprintf("%s%s ", cursor, noteIcon)
		prefixWidth := lipgloss.Width(prefix)

		maxTitleLen := max(m.listWidth-prefixWidth-2, 1)
		title := truncateToVisualWidth(note.Title, maxTitleLen)

		var titleRendered string
		if len(fr.titlePositions) > 0 {
			titleRunes := []rune(title)
			var validPositions []int
			for _, p := range fr.titlePositions {
				if p < len(titleRunes) {
					validPositions = append(validPositions, p)
				}
			}
			titleRendered = HighlightText(title, validPositions, NormalStyle)
		} else {
			titleRendered = title
		}

		line := prefix + titleRendered
		if lipgloss.Width(line) > m.listWidth {
			excess := lipgloss.Width(line) - m.listWidth
			if lipgloss.Width(title) > excess {
				title = truncateToVisualWidth(note.Title, max(maxTitleLen-excess, 1))
				if len(fr.titlePositions) > 0 {
					titleRunes := []rune(title)
					var validPositions []int
					for _, p := range fr.titlePositions {
						if p < len(titleRunes) {
							validPositions = append(validPositions, p)
						}
					}
					titleRendered = HighlightText(title, validPositions, NormalStyle)
				} else {
					titleRendered = title
				}
				line = prefix + titleRendered
			}
		}
		if i == m.cursor {
			line = TaskJumperSelectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderPreview renders the preview pane for the selected note.
func (m *NotesSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No note selected.")
	}

	fr := m.filtered[m.cursor]
	note := fr.note

	var b strings.Builder

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(truncateToVisualWidth(note.Title, max(m.previewWidth-2, 1))))
	b.WriteString("\n\n")

	if len(note.Tags) > 0 {
		fmt.Fprintf(&b, "%s %s\n", MutedStyle.Render("Tags:"), strings.Join(note.Tags, ", "))
	}
	if len(note.LinkedTasks) > 0 {
		b.WriteString(MutedStyle.Render("Linked tasks:"))
		b.WriteString("\n")
		for _, taskID := range note.LinkedTasks {
			taskTitle := "[deleted]"
			if task := m.project.GetTaskByID(taskID); task != nil {
				taskTitle = task.Title
			}
			fmt.Fprintf(&b, "  → %s\n", taskTitle)
		}
	}
	if note.Pinned {
		fmt.Fprintf(&b, "%s Yes\n", MutedStyle.Render("Pinned:"))
	}

	fmt.Fprintf(&b, "%s %s\n", MutedStyle.Render("Modified:"), formatNoteAge(note.ModifiedAt))

	if fr.bodySnippet != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Match"))
		b.WriteString("\n")
		// Truncate lines to fit preview width
		lines := strings.Split(fr.bodySnippet, "\n")
		for i, line := range lines {
			lines[i] = truncateToVisualWidth(line, m.previewWidth-2)
		}
		snippetText := strings.Join(lines, "\n")
		if len(fr.bodyPositions) > 0 {
			snippetRunes := len([]rune(snippetText))
			var validPositions []int
			for _, p := range fr.bodyPositions {
				if p < snippetRunes {
					validPositions = append(validPositions, p)
				}
			}
			if len(validPositions) > 0 {
				snippetText = HighlightText(snippetText, validPositions, NormalStyle)
			}
		}
		b.WriteString(snippetText)
	} else if note.Body != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Content"))
		b.WriteString("\n")
		rendered := renderMarkdown(note.Body, m.previewWidth-2)
		lines := strings.Split(rendered, "\n")
		maxLines := max(m.height-15, 3)
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	return b.String()
}

func (m *NotesSearchModel) StatusBarLabel() string { return "Notes Search" }
