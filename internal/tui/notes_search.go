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
}

// NotesSearchModel provides search across notes in a project.
type NotesSearchModel struct {
	project *models.Project
	notes   []*models.Note

	filtered []notesSearchResult
	cursor   int

	searchInput textinput.Model

	width        int
	height       int
	listWidth    int
	previewWidth int
}

// NewNotesSearchModel creates a new notes search model.
func NewNotesSearchModel(project *models.Project) *NotesSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search notes..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	notes, _ := storage.ListNotes(project)

	return &NotesSearchModel{
		project:     project,
		notes:       notes,
		searchInput: searchInput,
	}
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

		bodyScore := fuzzy.ScoreTerms(note.Body, query)

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
			if bodyScore > 0 {
				snippet = extractSnippet(note.Body, query)
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

// extractSnippet extracts a short snippet around the first match in body text.
func extractSnippet(body, query string) string {
	lowerBody := strings.ToLower(body)
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return ""
	}

	// Find the first occurrence of any term
	bestIdx := -1
	for _, term := range terms {
		idx := strings.Index(lowerBody, term)
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
		}
	}

	if bestIdx < 0 {
		// No exact substring match; return beginning of body
		if len(body) > 120 {
			return body[:120] + "..."
		}
		return body
	}

	// Extract context around the match
	start := bestIdx - 40
	if start < 0 {
		start = 0
	}
	end := bestIdx + 80
	if end > len(body) {
		end = len(body)
	}

	snippet := body[start:end]
	// Trim to word boundaries
	snippet = strings.TrimSpace(snippet)
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(body) {
		snippet = snippet + "..."
	}

	return snippet
}

// View renders the notes search view.
func (m *NotesSearchModel) View() string {
	var content strings.Builder

	content.WriteString(m.searchInput.View())
	content.WriteString("\n\n")

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		content.WriteString(MutedStyle.Render("Type to search note titles, tags, and body content..."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("Esc close"))

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    fmt.Sprintf("Notes Search - %s", m.project.Name),
			Subtitle: fmt.Sprintf("%d notes", len(m.notes)),
			Centered: false,
		})
	}

	if len(m.filtered) == 0 {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("No notes match \"%s\".", query)))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("Esc close"))

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    fmt.Sprintf("Notes Search - %s", m.project.Name),
			Subtitle: fmt.Sprintf("0/%d matching", len(m.notes)),
			Centered: false,
		})
	}

	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	listContent := m.renderNoteList()
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	content.WriteString(splitPane)
	content.WriteString("\n")
	content.WriteString(HelpStyle.Render(fmt.Sprintf("up/down nav | Enter select | Esc close | %d results", len(m.filtered))))

	subtitle := fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.notes))

	return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
		Title:    fmt.Sprintf("Notes Search - %s", m.project.Name),
		Subtitle: subtitle,
		Centered: false,
	})
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
		b.WriteString(fmt.Sprintf("%s %d\n", MutedStyle.Render("Linked tasks:"), len(note.LinkedTasks)))
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
		b.WriteString(strings.Join(lines, "\n"))
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
