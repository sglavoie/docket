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

// boardTriggers defines the trigger characters for focus board autocomplete.
var boardTriggers = []ACTrigger{
	{'%', acModeProject},
	{'!', acModePriority},
	{'#', acModeTag},
}

// getBoardProjectSuggestions returns fuzzy-matched project suggestions.
func (m *FocusBoardModel) getBoardProjectSuggestions(partial string) []ACSuggestion {
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

	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		name := truncateToVisualWidth(project.Name, 25)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%%%s [%s]", name, project.Context),
			Value:   project.Slug,
		})
	}
	return suggestions
}

// getBoardPrioritySuggestions returns matching priority options.
func (m *FocusBoardModel) getBoardPrioritySuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				Value:   p.value,
			})
		}
	}
	return suggestions
}

// getBoardTagSuggestions returns fuzzy-matched tag suggestions.
func (m *FocusBoardModel) getBoardTagSuggestions(partial string) []ACSuggestion {
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

	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: "#" + s.tag,
			Value:   s.tag,
		})
	}
	return suggestions
}

// refreshBoardAutocomplete detects triggers and updates the suggestion list.
func (m *FocusBoardModel) refreshBoardAutocomplete() {
	mode, pos, partial := DetectTrigger(m.addInput.Value(), boardTriggers)
	if mode == acModeNone {
		m.ac.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeProject:
		items = m.getBoardProjectSuggestions(partial)
	case acModePriority:
		items = m.getBoardPrioritySuggestions(partial)
	case acModeTag:
		items = m.getBoardTagSuggestions(partial)
	}
	m.ac.UpdateSuggestions(mode, pos, items)
}

// handleBoardAutocompleteKey handles keys when autocomplete is active.
// Returns true if the key was consumed.
func (m *FocusBoardModel) handleBoardAutocompleteKey(key string) bool {
	return m.ac.HandleKey(key, func() {
		m.acceptBoardSuggestion()
	})
}

// acceptBoardSuggestion inserts the selected suggestion into the input.
func (m *FocusBoardModel) acceptBoardSuggestion() {
	if !m.ac.IsActive() {
		return
	}

	triggerChar := TriggerCharForMode(m.ac.Mode, boardTriggers)
	newInput := m.ac.BuildAcceptedInput(m.addInput.Value(), triggerChar)

	m.addInput.SetValue(newInput)
	m.addInput.SetCursor(len(newInput))

	m.ac.Clear()
}

// initBoardCaches builds tag and project caches for autocomplete.
func (m *FocusBoardModel) initBoardCaches() {
	// Cache tags from all rows (allRows includes all non-complete/non-snoozed)
	tagSet := make(map[string]bool)
	for _, r := range m.allRows {
		for _, tag := range r.Task.Tags {
			tagSet[tag] = true
		}
	}
	for _, r := range m.closedRows {
		for _, tag := range r.Task.Tags {
			tagSet[tag] = true
		}
	}
	m.acTagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.acTagCache = append(m.acTagCache, tag)
	}
	sort.Strings(m.acTagCache)

	// Cache non-archived, non-sandbox projects (including internal for %project targeting)
	m.acProjectCache = m.config.GetAllLoadableProjects()
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
	return m.ac.IsActive()
}
