package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
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
	searchInput.Width = 40
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

// ShouldCaptureEsc returns false so app can close the view on esc.
func (m *NotesSearchModel) ShouldCaptureEsc() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *NotesSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	innerWidth := width - 10
	if innerWidth < 60 {
		innerWidth = 60
	}

	m.listWidth = innerWidth * 50 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	m.previewWidth = innerWidth - m.listWidth - 1

	m.searchInput.Width = innerWidth - 4
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

	case tea.KeyMsg:
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
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			return m, nil
		case "ctrl+d":
			m.cursor += 10
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
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
		m.filtered = nil
		m.cursor = 0
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

		bestScore := titleScore
		if bodyScore > bestScore {
			bestScore = bodyScore
		}
		if tagScore > bestScore {
			bestScore = tagScore
		}

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
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
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
		idx := strings.Index(lowerBody, term)
		if idx < 0 {
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
		start := idx - 30
		if start < 0 {
			start = 0
		}
		end := idx + len(term) + 50
		if end > len(body) {
			end = len(body)
		}
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
				for ri := 0; ri < len(termRunes); ri++ {
					positions = append(positions, runeStart+ri)
				}
				offset += idx + len(term)
			}
		}
	}

	return result.String(), positions
}

// View renders the notes search view.
func (m *NotesSearchModel) View() string {
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
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Notes Search - %s", m.project.Name), subtitle))
	b.WriteString("\n\n")

	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	if query == "" {
		b.WriteString(MutedStyle.Render("Type to search note titles, tags, and body content..."))
		b.WriteString("\n")
		left := HelpStyle.Render("Esc close")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return b.String()
	}

	if len(m.filtered) == 0 {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("No notes match \"%s\".", query)))
		b.WriteString("\n")
		left := HelpStyle.Render("Esc close")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return b.String()
	}

	paneHeight := m.height - 8
	if paneHeight < 5 {
		paneHeight = 5
	}

	listContent := m.renderNoteList()
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	b.WriteString(splitPane)
	b.WriteString("\n")

	// Footer
	left := HelpStyle.Render(fmt.Sprintf("up/down nav | Enter select | Esc close | %d results", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, "", m.width))

	return b.String()
}

// renderNoteList renders the filtered note list.
func (m *NotesSearchModel) renderNoteList() string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching notes.")
	}

	var b strings.Builder
	maxVisible := m.height - 7

	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

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

		maxTitleLen := m.listWidth - prefixWidth - 2
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := util.Truncate(note.Title, maxTitleLen)

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

	b.WriteString(TaskJumperPreviewHeaderStyle.Render(note.Title))
	b.WriteString("\n\n")

	if len(note.Tags) > 0 {
		b.WriteString(fmt.Sprintf("%s %s\n", MutedStyle.Render("Tags:"), strings.Join(note.Tags, ", ")))
	}
	if len(note.LinkedTasks) > 0 {
		b.WriteString(MutedStyle.Render("Linked tasks:"))
		b.WriteString("\n")
		for _, taskID := range note.LinkedTasks {
			taskTitle := "[deleted]"
			if task := m.project.GetTaskByID(taskID); task != nil {
				taskTitle = task.Title
			}
			b.WriteString(fmt.Sprintf("  → %s\n", taskTitle))
		}
	}
	if note.Pinned {
		b.WriteString(fmt.Sprintf("%s Yes\n", MutedStyle.Render("Pinned:")))
	}

	b.WriteString(fmt.Sprintf("%s %s\n", MutedStyle.Render("Modified:"), formatNoteAge(note.ModifiedAt)))

	if fr.bodySnippet != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Match"))
		b.WriteString("\n")
		// Truncate lines to fit preview width
		lines := strings.Split(fr.bodySnippet, "\n")
		for i, line := range lines {
			lines[i] = util.Truncate(line, m.previewWidth-2)
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
		body := note.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		lines := strings.Split(body, "\n")
		maxLines := m.height - 15
		if maxLines < 3 {
			maxLines = 3
		}
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		for i, line := range lines {
			lines[i] = util.Truncate(line, m.previewWidth-2)
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	return b.String()
}
