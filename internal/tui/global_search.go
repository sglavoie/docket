package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// filteredSearchResult holds a search result with match positions for highlighting.
type filteredSearchResult struct {
	result         models.SearchResult
	titlePositions []int // Match positions within the title for highlighting
}

// GlobalSearchModel provides global search across all projects.
type GlobalSearchModel struct {
	config   *config.Config
	projects []*models.Project
	allTasks []models.SearchResult  // flattened list of all tasks with project refs
	filtered []filteredSearchResult // tasks matching current query with highlight positions
	cursor   int

	// Input for filtering
	searchInput textinput.Model

	// Parsed filters from current query
	parsedFilters *ParsedSearchQuery

	// Autocomplete state
	acMode       int          // Current autocomplete mode (acSearchMode*)
	suggestions  []suggestion // Current autocomplete suggestions
	acCursor     int          // Selected suggestion index
	acTriggerPos int          // Position of trigger character in input

	// Caches for autocomplete
	tagCache     []string // All tags across projects
	sectionCache []string // All sections across projects

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int
}

// NewGlobalSearchModel creates a new global search model.
func NewGlobalSearchModel(cfg *config.Config) *GlobalSearchModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search all tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	projects := cfg.GetActiveProjects()

	// Load all tasks from all projects
	var allTasks []models.SearchResult
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			allTasks = append(allTasks, models.SearchResult{
				Task:    task,
				Project: project,
			})
		}
	}

	m := &GlobalSearchModel{
		config:      cfg,
		projects:    projects,
		allTasks:    allTasks,
		searchInput: searchInput,
		cursor:      0,
		acMode:      acSearchModeNone,
	}

	// Build caches for autocomplete
	m.buildCaches()

	// Start with no results (empty query shows nothing)
	m.filtered = nil
	return m
}

// Autocomplete mode constants for global search
const (
	acSearchModeNone    = iota // No autocomplete active
	acSearchModeTag            // Completing #tag
	acSearchModeSection        // Completing /section
	acSearchModeStatus         // Completing $status
	acSearchModeType           // Completing ^type
	acSearchModeDueDate        // Completing @duedate
)

// buildCaches builds tag and section caches from all loaded projects.
func (m *GlobalSearchModel) buildCaches() {
	tagSet := make(map[string]bool)
	sectionSet := make(map[string]bool)

	for _, project := range m.projects {
		// Collect tags from all tasks
		for _, task := range project.Tasks {
			for _, tag := range task.Tags {
				tagSet[tag] = true
			}
		}
		// Collect sections
		for _, section := range project.GetSections() {
			sectionSet[section] = true
		}
	}

	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}

	m.sectionCache = make([]string, 0, len(sectionSet))
	for section := range sectionSet {
		m.sectionCache = append(m.sectionCache, section)
	}
}

// HasActiveSearch returns true (search input is always active).
func (m *GlobalSearchModel) HasActiveSearch() bool {
	return true
}

// SetSize sets the view dimensions.
func (m *GlobalSearchModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := width - 10
	if innerWidth < 60 {
		innerWidth = 60
	}

	// Calculate split pane widths: 50/50 based on inner width
	m.listWidth = innerWidth * 50 / 100
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	m.previewWidth = innerWidth - m.listWidth - 1 // -1 for separator

	m.searchInput.Width = innerWidth - 4
}

func (m *GlobalSearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *GlobalSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
			switch key {
			case "tab", "enter":
				m.acceptAutocompleteSuggestion()
				m.filterTasks()
				return m, nil
			case "esc":
				// Dismiss autocomplete
				m.acMode = acSearchModeNone
				m.suggestions = nil
				m.acCursor = 0
				return m, nil
			case "up", "ctrl+p":
				if m.acCursor > 0 {
					m.acCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.acCursor < len(m.suggestions)-1 {
					m.acCursor++
				}
				return m, nil
			}
		}

		switch key {
		case "esc":
			return m, func() tea.Msg {
				return CloseGlobalSearchMsg{}
			}
		case "enter":
			// Only select task if not in autocomplete mode
			if m.acMode == acSearchModeNone && len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				result := m.filtered[m.cursor].result
				return m, func() tea.Msg {
					return GlobalSearchSelectMsg{
						Task:    result.Task,
						Project: result.Project,
					}
				}
			}
			return m, nil
		case "up", "ctrl+k":
			if m.acMode == acSearchModeNone && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.acMode == acSearchModeNone && m.cursor < len(m.filtered)-1 {
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
			// Update search input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterTasks()
			m.detectAutocomplete()
			return m, cmd
		}
	}

	return m, nil
}

// filterTasks filters tasks based on the current search query.
func (m *GlobalSearchModel) filterTasks() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Parse structured filters from query
	m.parsedFilters = ParseSearchQuery(query)

	// Empty query shows no results (unless filters are active)
	if query == "" {
		m.filtered = nil
		m.cursor = 0
		return
	}

	// Score each task and collect matches with title positions
	type scoredResult struct {
		result         models.SearchResult
		score          int
		titlePositions []int
	}
	var scored []scoredResult

	for _, result := range m.allTasks {
		// First apply structured filters
		if !TaskMatchesFilters(result.Task, m.parsedFilters) {
			continue
		}

		// If there's a text query, apply fuzzy scoring
		textQuery := m.parsedFilters.TextQuery
		if textQuery != "" {
			score, titlePositions := m.scoreTask(result.Task, textQuery)
			if score > 0 {
				scored = append(scored, scoredResult{
					result:         result,
					score:          score,
					titlePositions: titlePositions,
				})
			}
		} else {
			// No text query but filters matched - include with base score
			scored = append(scored, scoredResult{
				result:         result,
				score:          1,
				titlePositions: nil,
			})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results with positions
	m.filtered = make([]filteredSearchResult, len(scored))
	for i, s := range scored {
		m.filtered[i] = filteredSearchResult{
			result:         s.result,
			titlePositions: s.titlePositions,
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreTask returns a fuzzy match score for a task against the query,
// along with match positions within the title for highlighting.
// Returns 0 and nil if no match, higher scores indicate better matches.
func (m *GlobalSearchModel) scoreTask(task *models.Task, query string) (int, []int) {
	// Score against title (weighted higher)
	titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	if titleScore > 0 {
		titleScore *= 2 // Title matches are more important
	}

	// Score against description
	descScore := fuzzy.ScoreTerms(task.Description, query)

	// Return the best score with title positions (for highlighting)
	if titleScore > descScore {
		return titleScore, titlePositions
	}
	return descScore, nil // No title highlighting if description matched better
}

func (m *GlobalSearchModel) View() string {
	var content strings.Builder

	// Search input
	content.WriteString(m.searchInput.View())
	content.WriteString("\n")

	// Show autocomplete suggestions if active
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		for i, s := range m.suggestions {
			if i == m.acCursor {
				content.WriteString(SelectedStyle.Render("> " + s.display))
			} else {
				content.WriteString(MutedStyle.Render("  " + s.display))
			}
			content.WriteString("\n")
		}
	}
	content.WriteString("\n")

	// Show active filters as badges
	if m.parsedFilters != nil && m.parsedFilters.HasFilters() {
		content.WriteString(m.renderFilterBadges())
		content.WriteString("\n\n")
	}

	// Handle edge cases
	if len(m.allTasks) == 0 {
		content.WriteString(MutedStyle.Render("No tasks found in any project."))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: "0 tasks",
			Centered: false,
		})
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		content.WriteString(MutedStyle.Render("Type to search across all tasks..."))
		content.WriteString("\n\n")
		content.WriteString(MutedStyle.Render("Filters: !priority #tag @due $status ^type /section *active"))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: fmt.Sprintf("%d tasks", len(m.allTasks)),
			Centered: false,
		})
	}

	if len(m.filtered) == 0 {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("No tasks match \"%s\".", query)))
		content.WriteString("\n\n")
		content.WriteString(m.renderHelpText())

		return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Global Search",
			Subtitle: fmt.Sprintf("0/%d matching", len(m.allTasks)),
			Centered: false,
		})
	}

	// Calculate pane height for split view
	paneHeight := m.height - 10 // Account for card border, title, input, footer
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Adjust for autocomplete and filter badges
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		paneHeight -= len(m.suggestions)
	}
	if m.parsedFilters != nil && m.parsedFilters.HasFilters() {
		paneHeight -= 2
	}

	// Split pane: list on left, preview on right
	listContent := m.renderTaskList()
	previewContent := m.renderPreview()

	splitPane := renderOverlaySplitPane(listContent, previewContent, m.listWidth, m.previewWidth, paneHeight)
	content.WriteString(splitPane)
	content.WriteString("\n")

	// Footer
	content.WriteString(m.renderHelpText())

	// Build subtitle showing count
	subtitle := fmt.Sprintf("%d/%d matching", len(m.filtered), len(m.allTasks))

	return renderOverlayCard(content.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Global Search",
		Subtitle: subtitle,
		Centered: false,
	})
}

// renderFilterBadges renders active filters as styled badges.
func (m *GlobalSearchModel) renderFilterBadges() string {
	if m.parsedFilters == nil {
		return ""
	}

	var badges []string

	// Priority badges
	if len(m.parsedFilters.Priorities) > 0 {
		var pStrs []string
		for _, p := range m.parsedFilters.Priorities {
			pStrs = append(pStrs, fmt.Sprintf("%d", p))
		}
		badges = append(badges, fmt.Sprintf("!%s", strings.Join(pStrs, ",")))
	}

	// Tag badges
	for _, tag := range m.parsedFilters.Tags {
		badges = append(badges, "#"+tag)
	}

	// Status badges
	for _, status := range m.parsedFilters.Statuses {
		badges = append(badges, "$"+string(status))
	}

	// Type badges
	for _, taskType := range m.parsedFilters.Types {
		badges = append(badges, "^"+string(taskType))
	}

	// Due date badge
	if m.parsedFilters.DueDateFilter != "" {
		badges = append(badges, "@"+m.parsedFilters.DueDateFilter)
	}

	// Section badge
	if m.parsedFilters.Section != "" {
		badges = append(badges, "/"+m.parsedFilters.Section)
	}

	// Active badge
	if m.parsedFilters.ActiveOnly {
		badges = append(badges, "*active")
	}

	if len(badges) == 0 {
		return ""
	}

	// Style each badge
	var styled []string
	for _, badge := range badges {
		styled = append(styled, FilterBadgeStyle.Render(badge))
	}

	return "Filters: " + strings.Join(styled, " ")
}

// renderHelpText renders context-appropriate help text.
func (m *GlobalSearchModel) renderHelpText() string {
	if m.acMode != acSearchModeNone && len(m.suggestions) > 0 {
		return HelpStyle.Render("Tab: accept • ↑↓: navigate • Esc: dismiss")
	}
	return HelpStyle.Render(fmt.Sprintf("↑↓ nav • Enter • Esc • %d results", len(m.filtered)))
}

// renderTaskList renders the filtered task list.
func (m *GlobalSearchModel) renderTaskList() string {
	if len(m.filtered) == 0 {
		return MutedStyle.Render("No matching tasks.")
	}

	var b strings.Builder
	maxVisible := m.height - 7 // Account for chrome

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(m.filtered) && i < scrollOffset+maxVisible; i++ {
		fr := m.filtered[i]
		result := fr.result
		task := result.Task

		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Type icon with badge style
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

		// Status icon with badge style
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

		// Project name (truncated to 8 chars to save space)
		projectSlug := util.Truncate(result.Project.Slug, 8)
		projectName := MutedStyle.Render(fmt.Sprintf("[%s]", projectSlug))

		// Short ID
		shortID := MutedStyle.Render(fmt.Sprintf("#%-4d", task.ShortID))

		// Build prefix to measure its actual visual width
		prefix := fmt.Sprintf("%s%s %s %s %s ", cursor, typeIcon, statusIcon, shortID, projectName)
		prefixWidth := lipgloss.Width(prefix)

		// Title (truncated) - use measured prefix width for accuracy
		// Subtract extra buffer to ensure no wrapping
		maxTitleLen := m.listWidth - prefixWidth - 2
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		title := util.Truncate(task.Title, maxTitleLen)

		// Adjust positions for truncated title and apply highlighting
		var titleRendered string
		if len(fr.titlePositions) > 0 {
			// Filter positions to only include those within the truncated title
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

		// Build line and verify total width doesn't exceed listWidth
		line := prefix + titleRendered
		lineWidth := lipgloss.Width(line)
		if lineWidth > m.listWidth {
			// Truncate title further if still too wide (recalculate without highlighting)
			excess := lineWidth - m.listWidth + 3
			if len(title) > excess {
				title = util.Truncate(task.Title, maxTitleLen-excess)
				// Re-apply highlighting with new truncated title
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

// renderPreview renders the preview pane for the selected task.
func (m *GlobalSearchModel) renderPreview() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return MutedStyle.Render("No task selected.")
	}

	result := m.filtered[m.cursor].result
	task := result.Task
	project := result.Project
	var b strings.Builder

	// Project header
	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(project.Name))
	b.WriteString("\n\n")

	// Header with type and title (truncate to fit preview width)
	typeStyle := GetTypeStyle(string(task.Type))
	titlePrefix := fmt.Sprintf("%s #%d: ", task.Type.HeadingPrefix(), task.ShortID)
	maxTitleLen := m.previewWidth - len(titlePrefix) - 4 // -4 for padding/margin
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		util.Truncate(task.Title, maxTitleLen))
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(header))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	b.WriteString(fmt.Sprintf("%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status))))

	// Priority
	priorityStyle := NormalStyle
	if task.Priority <= 2 {
		priorityStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	}
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority))))

	// Type
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Type:"),
		typeStyle.Render(task.Type.HeadingPrefix())))

	// Tags
	if len(task.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = tagStyle.Render(tag)
		}
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", ")))
	}

	// Due date
	if task.DueDate != nil {
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Due:"),
			task.DueDate.Format(util.DateFormat)))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(util.Truncate(parentRef, m.previewWidth-10))))
	}

	// Description (truncated)
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")

		// Truncate description to fit preview
		maxLines := m.height - 20
		if maxLines < 3 {
			maxLines = 3
		}
		lines := strings.Split(task.Description, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, "...")
		}
		// Truncate each line to fit preview width
		for i, line := range lines {
			lines[i] = util.Truncate(line, m.previewWidth-2)
		}
		b.WriteString(strings.Join(lines, "\n"))
	}

	// Children summary
	if len(task.Children) > 0 {
		b.WriteString("\n\n")
		completed := 0
		for _, child := range task.Children {
			if child.IsComplete() {
				completed++
			}
		}
		b.WriteString(fmt.Sprintf("%s %d/%d completed\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children)))
	}

	return b.String()
}

// detectAutocomplete checks if the cursor is after a filter trigger character
// and populates suggestions accordingly.
func (m *GlobalSearchModel) detectAutocomplete() {
	input := m.searchInput.Value()
	if len(input) == 0 {
		m.acMode = acSearchModeNone
		m.suggestions = nil
		return
	}

	// Scan backward from end to find most recent trigger
	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			// Hit whitespace before finding trigger
			break
		}

		partial := input[i+1:]

		switch ch {
		case '#':
			m.acMode = acSearchModeTag
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getTagSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '/':
			m.acMode = acSearchModeSection
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getSectionSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '$':
			m.acMode = acSearchModeStatus
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getStatusSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '^':
			m.acMode = acSearchModeType
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getTypeSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '@':
			m.acMode = acSearchModeDueDate
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getDueDateSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		}
	}

	m.acMode = acSearchModeNone
	m.suggestions = nil
}

// getTagSuggestions returns matching tags for the partial query.
func (m *GlobalSearchModel) getTagSuggestions(partial string) []suggestion {
	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, tag := range m.tagCache {
		// Show all if partial is empty, otherwise filter by prefix/contains
		if partial == "" || strings.Contains(strings.ToLower(tag), partialLower) {
			suggestions = append(suggestions, suggestion{
				display: "#" + tag,
				value:   tag,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getSectionSuggestions returns matching sections for the partial query.
func (m *GlobalSearchModel) getSectionSuggestions(partial string) []suggestion {
	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, suggestion{
				display: "/" + section,
				value:   section,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getStatusSuggestions returns matching status options for the partial query.
func (m *GlobalSearchModel) getStatusSuggestions(partial string) []suggestion {
	statuses := []struct {
		name string
		icon string
	}{
		{"todo", "○"},
		{"in_progress", "◐"},
		{"done", "●"},
		{"blocked", "◈"},
		{"cancelled", "✕"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("$%s %s", s.name, s.icon),
				value:   s.name,
			})
		}
	}
	return suggestions
}

// getTypeSuggestions returns matching type options for the partial query.
func (m *GlobalSearchModel) getTypeSuggestions(partial string) []suggestion {
	types := []struct {
		name string
		icon string
	}{
		{"epic", "◆"},
		{"story", "◇"},
		{"task", "□"},
		{"bug", "⚠"},
		{"idea", "💡"},
		{"note", "📝"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("^%s %s", t.name, t.icon),
				value:   t.name,
			})
		}
	}
	return suggestions
}

// getDueDateSuggestions returns due date filter options for the partial query.
func (m *GlobalSearchModel) getDueDateSuggestions(partial string) []suggestion {
	options := []struct {
		name string
		desc string
	}{
		{"today", "due today"},
		{"tomorrow", "due tomorrow"},
		{"week", "due this week"},
		{"overdue", "past due date"},
		{"none", "no due date"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("@%s (%s)", o.name, o.desc),
				value:   o.name,
			})
		}
	}
	return suggestions
}

// acceptAutocompleteSuggestion inserts the selected suggestion into the input.
func (m *GlobalSearchModel) acceptAutocompleteSuggestion() {
	if m.acMode == acSearchModeNone || len(m.suggestions) == 0 {
		return
	}

	selected := m.suggestions[m.acCursor]
	input := m.searchInput.Value()

	// Build the prefix character based on mode
	var prefix string
	switch m.acMode {
	case acSearchModeTag:
		prefix = "#"
	case acSearchModeSection:
		prefix = "/"
	case acSearchModeStatus:
		prefix = "$"
	case acSearchModeType:
		prefix = "^"
	case acSearchModeDueDate:
		prefix = "@"
	}

	// Replace from trigger position to end with completed value
	newInput := input[:m.acTriggerPos] + prefix + selected.value + " "
	m.searchInput.SetValue(newInput)
	m.searchInput.SetCursor(len(newInput))

	// Reset autocomplete state
	m.acMode = acSearchModeNone
	m.suggestions = nil
	m.acCursor = 0
}

// CloseGlobalSearchMsg is sent when the global search should close.
type CloseGlobalSearchMsg struct{}

// GlobalSearchSelectMsg is sent when a task is selected from global search.
type GlobalSearchSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowGlobalSearchMsg is sent to open the global search.
type ShowGlobalSearchMsg struct{}
