package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// boardAddResult holds parsed metadata from focus board quick-add input.
type boardAddResult struct {
	title        string
	source       string
	url          string
	priority     int
	tags         []string
	tagsToRemove []string
	projectSlug  string
	dueDate      *time.Time
	dueDateClear bool
	section      string
	status       models.TaskStatus
	statusSet    bool
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

// parseBoardPriority parses a priority value from s, accepting both numeric ("1"-"5")
// and named forms (critical/crit/c=1, high/h=2, medium/med/m=3, low/l=4, minimal/min=5).
func parseBoardPriority(s string) (int, bool) {
	if len(s) == 1 && s[0] >= '1' && s[0] <= '5' {
		return int(s[0] - '0'), true
	}
	synonyms := map[string]int{
		"critical": 1, "crit": 1, "c": 1,
		"high": 2, "h": 2,
		"medium": 3, "med": 3, "m": 3,
		"low": 4, "l": 4,
		"minimal": 5, "min": 5,
	}
	if p, ok := synonyms[strings.ToLower(s)]; ok {
		return p, true
	}
	return 0, false
}

// parseBoardAddInput extracts title and optional prefix tokens (%project, !N, #tag,
// -#tag, @date, /section, $status) alongside --source, --url, --priority flags.
func parseBoardAddInput(input string) boardAddResult {
	var result boardAddResult

	input = extractBoardRemoveTags(input, &result)
	input = extractBoardDueDate(input, &result)
	input = extractBoardSection(input, &result)
	input = extractBoardStatus(input, &result)
	result.title = parseBoardAddTokens(input, &result)
	return result
}

func extractBoardRemoveTags(input string, result *boardAddResult) string {
	for _, match := range qeRemoveTagRe.FindAllStringSubmatch(input, -1) {
		if len(match) > 1 {
			result.tagsToRemove = append(result.tagsToRemove, match[1])
		}
	}
	return qeRemoveTagRe.ReplaceAllString(input, " ")
}

func extractBoardDueDate(input string, result *boardAddResult) string {
	match := qeDueDateRe.FindStringSubmatch(input)
	if len(match) <= 1 {
		return input
	}
	if match[1] == "0" {
		result.dueDateClear = true
	} else if date, err := ParseDate(match[1]); err == nil {
		result.dueDate = &date
	}
	return qeDueDateRe.ReplaceAllString(input, " ")
}

func extractBoardSection(input string, result *boardAddResult) string {
	match := qeSectionRe.FindStringSubmatch(input)
	if len(match) <= 1 {
		return input
	}
	if match[1] != "0" {
		result.section = match[1]
	}
	return qeSectionRe.ReplaceAllString(input, " ")
}

func extractBoardStatus(input string, result *boardAddResult) string {
	match := qeStatusRe.FindStringSubmatch(input)
	if len(match) <= 1 {
		return input
	}
	if status, ok := boardAddStatus(strings.ToLower(match[1])); ok {
		result.status = status
		result.statusSet = true
	}
	return qeStatusRe.ReplaceAllString(input, " ")
}

func boardAddStatus(status string) (models.TaskStatus, bool) {
	switch status {
	case "start", "started", "progress":
		return models.StatusInProgress, true
	case "blocked", "block":
		return models.StatusBlocked, true
	case "done", "complete", "completed":
		return models.StatusDone, true
	case "canceled", "cancel":
		return models.StatusCanceled, true
	case "todo", "open", "reopen":
		return models.StatusTodo, true
	case "review":
		return models.StatusReview, true
	case "standby", "hold":
		return models.StatusStandby, true
	case "maybe":
		return models.StatusMaybe, true
	default:
		return "", false
	}
}

func parseBoardAddTokens(input string, result *boardAddResult) string {
	parts := strings.Fields(input)
	var titleParts []string

	for i := 0; i < len(parts); i++ {
		if parseBoardFlag(parts, &i, result) {
			continue
		}
		if parseBoardInlineToken(parts[i], result) {
			continue
		}
		titleParts = append(titleParts, parts[i])
	}

	return strings.Join(titleParts, " ")
}

func parseBoardFlag(parts []string, idx *int, result *boardAddResult) bool {
	if *idx+1 >= len(parts) {
		return false
	}

	switch parts[*idx] {
	case "--source", "-s":
		result.source = parts[*idx+1]
	case "--url", "-u":
		result.url = parts[*idx+1]
	case "--priority", "-p":
		if p, ok := parseBoardPriority(parts[*idx+1]); ok {
			result.priority = p
		}
	default:
		return false
	}

	*idx = *idx + 1
	return true
}

func parseBoardInlineToken(part string, result *boardAddResult) bool {
	switch {
	case strings.HasPrefix(part, "%") && len(part) > 1:
		result.projectSlug = part[1:]
	case strings.HasPrefix(part, "!") && len(part) > 1:
		if p, ok := parseBoardPriority(part[1:]); ok {
			result.priority = p
		}
	case strings.HasPrefix(part, "#") && len(part) > 1:
		result.tags = append(result.tags, part[1:])
	default:
		return false
	}
	return true
}

// isAutocompleteActive returns whether the autocomplete dropdown is showing.
func (m *FocusBoardModel) isAutocompleteActive() bool {
	return m.ac.IsActive()
}
