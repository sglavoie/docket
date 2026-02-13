package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// boardAddResult holds parsed metadata from focus board quick-add input.
type boardAddResult struct {
	title       string
	source      string
	url         string
	priority    int
	tags        []string
	projectSlug string
}

// detectBoardAutocompleteTrigger scans the add input for prefix triggers.
// Only supports %project, !priority, and #tag for focus board items.
func (m *FocusBoardModel) detectBoardAutocompleteTrigger() (int, int, string) {
	input := m.addInput.Value()
	if len(input) == 0 {
		return acModeNone, 0, ""
	}

	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			break
		}
		atStartOrAfterSpace := i == 0 || input[i-1] == ' ' || input[i-1] == '\n'
		if !atStartOrAfterSpace {
			continue
		}
		partial := input[i+1:]
		switch ch {
		case '%':
			return acModeProject, i, partial
		case '!':
			return acModePriority, i, partial
		case '#':
			return acModeTag, i, partial
		}
	}
	return acModeNone, 0, ""
}

// getBoardProjectSuggestions returns fuzzy-matched project suggestions.
func (m *FocusBoardModel) getBoardProjectSuggestions(partial string) []suggestion {
	type scoredProject struct {
		project *models.Project
		score   int
	}

	var scored []scoredProject
	for _, project := range m.acProjectCache {
		searchText := fmt.Sprintf("%s %s", project.Name, project.Context)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredProject{project: project, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		name := project.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}
		suggestions = append(suggestions, suggestion{
			display: fmt.Sprintf("%%%s [%s]", name, project.Context),
			value:   project.Slug,
		})
	}
	return suggestions
}

// getBoardPrioritySuggestions returns matching priority options.
func (m *FocusBoardModel) getBoardPrioritySuggestions(partial string) []suggestion {
	priorities := []struct {
		value string
		desc  string
	}{
		{"1", "Urgent"},
		{"2", "High"},
		{"3", "Normal"},
		{"4", "Low"},
		{"5", "Lowest"},
	}

	var suggestions []suggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				value:   p.value,
			})
		}
	}
	return suggestions
}

// getBoardTagSuggestions returns fuzzy-matched tag suggestions.
func (m *FocusBoardModel) getBoardTagSuggestions(partial string) []suggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range m.acTagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, suggestion{
			display: "#" + s.tag,
			value:   s.tag,
		})
	}
	return suggestions
}

// refreshBoardAutocomplete detects triggers and updates the suggestion list.
func (m *FocusBoardModel) refreshBoardAutocomplete() {
	mode, pos, partial := m.detectBoardAutocompleteTrigger()
	if mode == acModeNone {
		m.acMode = acModeNone
		m.acSuggestions = nil
		return
	}

	if mode != m.acMode || pos != m.acTriggerPos {
		m.acCursor = 0
		m.acViewportTop = 0
	}
	m.acMode = mode
	m.acTriggerPos = pos

	switch mode {
	case acModeProject:
		m.acSuggestions = m.getBoardProjectSuggestions(partial)
	case acModePriority:
		m.acSuggestions = m.getBoardPrioritySuggestions(partial)
	case acModeTag:
		m.acSuggestions = m.getBoardTagSuggestions(partial)
	}

	if m.acCursor >= len(m.acSuggestions) {
		m.acCursor = max(0, len(m.acSuggestions)-1)
	}
	if m.acCursor < m.acViewportTop {
		m.acViewportTop = m.acCursor
	} else if m.acCursor >= m.acViewportTop+acVisibleCount {
		m.acViewportTop = m.acCursor - acVisibleCount + 1
	}
	if len(m.acSuggestions) == 0 {
		m.acMode = acModeNone
	}
}

// handleBoardAutocompleteKey handles keys when autocomplete is active.
// Returns true if the key was consumed.
func (m *FocusBoardModel) handleBoardAutocompleteKey(key string) bool {
	if m.acMode == acModeNone || len(m.acSuggestions) == 0 {
		return false
	}

	switch key {
	case "tab", "enter":
		m.acceptBoardSuggestion()
		return true
	case "esc":
		m.resetBoardAutocomplete()
		return true
	case "up", "ctrl+p", "ctrl+k":
		if m.acCursor > 0 {
			m.acCursor--
			if m.acCursor < m.acViewportTop {
				m.acViewportTop = m.acCursor
			}
		}
		return true
	case "down", "ctrl+n", "ctrl+j":
		if m.acCursor < len(m.acSuggestions)-1 {
			m.acCursor++
			if m.acCursor >= m.acViewportTop+acVisibleCount {
				m.acViewportTop = m.acCursor - acVisibleCount + 1
			}
		}
		return true
	}
	return false
}

// acceptBoardSuggestion inserts the selected suggestion into the input.
func (m *FocusBoardModel) acceptBoardSuggestion() {
	if m.acMode == acModeNone || len(m.acSuggestions) == 0 {
		return
	}

	selected := m.acSuggestions[m.acCursor]
	input := m.addInput.Value()

	var newInput string
	switch m.acMode {
	case acModeProject:
		newInput = input[:m.acTriggerPos] + "%" + selected.value + " "
	case acModePriority:
		newInput = input[:m.acTriggerPos] + "!" + selected.value + " "
	case acModeTag:
		newInput = input[:m.acTriggerPos] + "#" + selected.value + " "
	}

	m.addInput.SetValue(newInput)
	m.addInput.SetCursor(len(newInput))

	m.resetBoardAutocomplete()
}

// resetBoardAutocomplete clears all autocomplete state.
func (m *FocusBoardModel) resetBoardAutocomplete() {
	m.acMode = acModeNone
	m.acSuggestions = nil
	m.acCursor = 0
	m.acViewportTop = 0
}

// initBoardCaches builds tag and project caches for autocomplete.
func (m *FocusBoardModel) initBoardCaches() {
	// Cache tags from all project tasks and board items
	tagSet := make(map[string]bool)
	for _, project := range m.config.GetActiveProjects() {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			for _, tag := range task.Tags {
				tagSet[tag] = true
			}
		}
	}
	for _, item := range m.boardItems {
		for _, tag := range item.Tags {
			tagSet[tag] = true
		}
	}
	m.acTagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.acTagCache = append(m.acTagCache, tag)
	}
	sort.Strings(m.acTagCache)

	// Cache non-archived, non-sandbox projects
	m.acProjectCache = m.config.GetActiveProjects()
}

// parseBoardAddInput extracts title and optional prefix tokens (%project, !N, #tag)
// alongside existing --source, --url, --priority flags.
func parseBoardAddInput(input string) boardAddResult {
	parts := strings.Fields(input)
	var result boardAddResult
	var titleParts []string

	for i := 0; i < len(parts); i++ {
		switch {
		case parts[i] == "--source" || parts[i] == "-s":
			if i+1 < len(parts) {
				result.source = parts[i+1]
				i++
			}
		case parts[i] == "--url" || parts[i] == "-u":
			if i+1 < len(parts) {
				result.url = parts[i+1]
				i++
			}
		case parts[i] == "--priority" || parts[i] == "-p":
			if i+1 < len(parts) {
				if p := parts[i+1]; len(p) == 1 && p[0] >= '1' && p[0] <= '5' {
					result.priority = int(p[0] - '0')
				}
				i++
			}
		case strings.HasPrefix(parts[i], "%") && len(parts[i]) > 1:
			result.projectSlug = parts[i][1:]
		case strings.HasPrefix(parts[i], "!") && len(parts[i]) == 2 && parts[i][1] >= '1' && parts[i][1] <= '5':
			result.priority = int(parts[i][1] - '0')
		case strings.HasPrefix(parts[i], "#") && len(parts[i]) > 1:
			result.tags = append(result.tags, parts[i][1:])
		default:
			titleParts = append(titleParts, parts[i])
		}
	}

	result.title = strings.Join(titleParts, " ")
	return result
}

// isAutocompleteActive returns whether the autocomplete dropdown is showing.
func (m *FocusBoardModel) isAutocompleteActive() bool {
	return m.acMode != acModeNone && len(m.acSuggestions) > 0
}
