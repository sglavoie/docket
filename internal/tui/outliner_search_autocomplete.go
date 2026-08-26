package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// outlinerSearchSuggestionLimit is the dropdown's total row budget, counting the
// overflow indicators RenderDropdownWithLimit spends rows on. The pane sits
// between the operational title and the first task row, so every row it takes is
// a row the result list loses.
const outlinerSearchSuggestionLimit = 5

// outlinerSearchDropdownIndent aligns the suggestion rows under the title's
// search segment without claiming a column of its own.
const outlinerSearchDropdownIndent = "  "

// outlinerSearchAutocomplete is the Outliner-only suggestion state for the slash
// search input. It is session-only presentation state: the vocabulary, the
// cardinality, and every static value are projections of
// textgrammar.SurfaceSearch, and the dynamic values are recomputed from the live
// candidate set on every refresh, so nothing here can go stale behind a mutation.
//
// Project Hub and Notes deliberately have no counterpart: they stay ordinary
// fuzzy text searches.
type outlinerSearchAutocomplete struct {
	ac AutocompleteState
	// triggerText is the accepted token's sigil, retained because a two-character
	// trigger (-# and ##, @@) cannot be recovered from the mode alone once the
	// suggestion list is built. The grammar family itself is not retained beside
	// it: singleton suppression and repeatable exclusion are decided from the
	// definition the refresh is already holding, and a second copy here would be
	// a field nothing reads.
	triggerText string
}

func (a *outlinerSearchAutocomplete) active() bool {
	return a != nil && a.ac.IsActive()
}

func (a *outlinerSearchAutocomplete) clear() {
	if a == nil {
		return
	}
	a.ac.Clear()
	a.triggerText = ""
}

// outlinerSearchTriggers projects the canonical search definitions into the
// autocomplete engine's presentation modes. The registry owns which triggers
// exist and in what parse order; this projection owns nothing but the mode
// number attached to each field, which is why a sigil added to SurfaceSearch
// reaches completion without a second list being edited.
func outlinerSearchTriggers() []ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceSearch)
	triggers := make([]ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		mode := autocompleteModeForField(definition.Field)
		if mode == acModeNone {
			continue
		}
		triggers = append(triggers, ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

// outlinerSearchDefinitionForMode is the reverse of the projection above. It
// returns the full definition rather than the field alone, so cardinality,
// values, and clears are read from the same entry the trigger came from.
func outlinerSearchDefinitionForMode(mode int) (textgrammar.Definition, bool) {
	if mode == acModeNone {
		return textgrammar.Definition{}, false
	}
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		if autocompleteModeForField(definition.Field) == mode {
			return definition, true
		}
	}
	return textgrammar.Definition{}, false
}

// searchAutocompleteCursorByte converts the input's rune caret to a byte offset,
// which is the unit DetectTrigger and BuildAcceptedInputAt both work in.
func (m *OutlinerModel) searchAutocompleteCursorByte() int {
	runes := []rune(m.SearchInputValue())
	position := max(0, min(m.SearchInputCursor(), len(runes)))
	return len(string(runes[:position]))
}

// refreshSearchAutocomplete recomputes the suggestion list from the token under
// the caret. It is the single entry point: search closing, the caret moving, the
// query changing, and a suggestion being accepted all land here, and each one
// either produces a fresh list or clears the state.
func (m *OutlinerModel) refreshSearchAutocomplete() {
	if m == nil {
		return
	}
	if !m.IsSearchMode() {
		m.searchAutocomplete.clear()
		return
	}

	value := m.SearchInputValue()
	cursorByte := m.searchAutocompleteCursorByte()
	// Only the text before the caret can name the active token; the longest
	// matching trigger wins there, which is what keeps ## out of #, @@ out of @,
	// and -# out of #.
	mode, triggerPos, partial := DetectTrigger(value[:cursorByte], outlinerSearchTriggers())
	definition, ok := outlinerSearchDefinitionForMode(mode)
	if !ok {
		m.searchAutocomplete.clear()
		return
	}

	// The settled query prefix is what the grammar has already accepted. A
	// singleton field is suppressed once it appears there, and a repeatable one
	// keeps every value it has not used yet. Cardinality is read from the
	// definition rather than from a parallel field list.
	prefix := textgrammar.Parse(value[:triggerPos], textgrammar.SurfaceSearch)
	if definition.Cardinality == textgrammar.CardinalitySingleton &&
		len(prefix.TokensFor(definition.Field)) > 0 {
		m.searchAutocomplete.clear()
		return
	}

	items := m.searchAutocompleteSuggestions(definition, partial)
	if definition.Cardinality == textgrammar.CardinalityRepeatable {
		items = filterUsedTagSuggestions(items, prefix.TokensFor(definition.Field))
	}

	m.searchAutocomplete.triggerText = definition.Trigger
	m.searchAutocomplete.ac.UpdateSuggestions(mode, triggerPos, items)
	if !m.searchAutocomplete.ac.IsActive() {
		// The active token no longer has anything to offer, so the retained
		// trigger would outlive the list it described.
		m.searchAutocomplete.clear()
	}
}

func (m *OutlinerModel) searchAutocompleteSuggestions(
	definition textgrammar.Definition,
	partial string,
) []ACSuggestion {
	switch definition.Field { //nolint:exhaustive // Only the fields SurfaceSearch admits can reach a trigger here.
	case textgrammar.FieldPriority:
		return searchPrioritySuggestions(definition, partial)
	case textgrammar.FieldTag, textgrammar.FieldRemoveTag:
		return searchDynamicSuggestions(definition, m.searchCandidateTagNames(), partial)
	case textgrammar.FieldContext:
		return searchDynamicSuggestions(definition, m.searchCandidateContextNames(), partial)
	case textgrammar.FieldDue:
		return searchStaticSuggestions(definition, searchDueVocabulary(definition), partial)
	case textgrammar.FieldDefer:
		return searchStaticSuggestions(definition, searchDeferVocabulary(definition), partial)
	case textgrammar.FieldDestination:
		return searchDestinationSuggestions(definition, partial)
	case textgrammar.FieldPinned:
		return searchPinSuggestions(definition, partial)
	case textgrammar.FieldProject:
		return m.searchProjectSuggestions(definition)
	default:
		return nil
	}
}

// searchDueVocabulary is the relational bucket set plus the one parametric form
// no fixed name can express. The shared capture values are deliberately absent:
// a due search asks which bucket a task falls in, and an exact calendar day is
// typed rather than picked.
func searchDueVocabulary(definition textgrammar.Definition) []string {
	values := textgrammar.SearchPresetValues(definition.Field)
	return appendUnprefixedValues(values, definition, textgrammar.SearchExampleValues(definition.Field))
}

// searchDeferVocabulary carries both halves of the deferral vocabulary: the two
// population splits only search understands, then the shared date and hour forms
// a deferral instant is actually written in.
func searchDeferVocabulary(definition textgrammar.Definition) []string {
	values := textgrammar.SearchPresetValues(definition.Field)
	values = appendUnprefixedValues(values, definition, definition.Values)
	values = appendUnprefixedValues(values, definition, textgrammar.SearchExampleValues(definition.Field))
	return appendUnprefixedValues(values, definition, definition.Examples)
}

// appendUnprefixedValues folds a projection into the vocabulary, dropping the
// sigil an Examples entry carries and skipping anything already listed. Order is
// the order the projections were folded in, which is the order they are offered.
func appendUnprefixedValues(values []string, definition textgrammar.Definition, incoming []string) []string {
	for _, value := range incoming {
		value = strings.TrimPrefix(value, definition.Trigger)
		if value == "" || sliceContainsFold(values, value) {
			continue
		}
		values = append(values, value)
	}
	return values
}

func sliceContainsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func searchPrioritySuggestions(definition textgrammar.Definition, partial string) []ACSuggestion {
	result := make([]ACSuggestion, 0, len(definition.Values))
	for _, value := range definition.Values {
		priorities, ok := textgrammar.ParsePriority(value, false)
		if !ok || len(priorities) != 1 {
			continue
		}
		if partial != "" && !strings.HasPrefix(value, partial) {
			continue
		}
		result = append(result, ACSuggestion{
			Display: fmt.Sprintf("%s%s (%s)", definition.Trigger, value, models.PriorityName(priorities[0])),
			Value:   value,
		})
	}
	return result
}

func searchStaticSuggestions(definition textgrammar.Definition, values []string, partial string) []ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]ACSuggestion, 0, len(values))
	for _, value := range values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	return result
}

func searchDestinationSuggestions(definition textgrammar.Definition, partial string) []ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]ACSuggestion, 0, len(definition.Values))
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		destination, ok := textgrammar.ParseDestination(value)
		if !ok {
			continue
		}
		result = append(result, ACSuggestion{
			Display: definition.Trigger + value + " " + searchDestinationDescription(destination),
			Value:   value,
		})
	}
	return result
}

// searchDestinationDescription names what the destination selects. Today and
// Next are planning collections rather than states, so the shared state
// description would report the backlog they both store tasks in.
func searchDestinationDescription(destination textgrammar.Destination) string {
	if destination.IsPlanning() {
		return "(planning list)"
	}
	return "(" + textgrammar.StateDescription(destination.WorkflowState()) + ")"
}

// searchPinSuggestions offers both halves of the pin axis. The set value comes
// from Values and the unset one from the search-surface clear, which is the only
// place *unpin is declared.
func searchPinSuggestions(definition textgrammar.Definition, partial string) []ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]ACSuggestion, 0, len(definition.Values)+len(definition.Clears))
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	for _, clear := range definition.Clears {
		if partial != "" && !strings.Contains(strings.ToLower(clear.Value), partial) {
			continue
		}
		display := definition.Trigger + clear.Value
		if clear.Description != "" {
			display += " (" + clear.Description + ")"
		}
		result = append(result, ACSuggestion{Display: display, Value: clear.Value})
	}
	return result
}

// searchProjectSuggestions offers the projects already loaded in the current
// search scope and nothing else: completion must never load a wider catalog than
// the query it is completing can match against. The scope is small enough that
// every project is listed, so the partial is left to the dropdown's own
// navigation rather than filtering the scope away.
func (m *OutlinerModel) searchProjectSuggestions(definition textgrammar.Definition) []ACSuggestion {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	result := make([]ACSuggestion, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		result = append(result, ACSuggestion{
			Display: definition.Trigger + project.Slug + " " + project.Name,
			Value:   project.Slug,
		})
	}
	return result
}

// searchDynamicSuggestions ranks live values with the shared fuzzy score and
// keeps source order for ties, so an empty partial — which scores every value
// identically — offers a list that does not reshuffle between keystrokes.
func searchDynamicSuggestions(definition textgrammar.Definition, values []string, partial string) []ACSuggestion {
	type candidate struct {
		value string
		score int
	}
	items := make([]candidate, 0, len(values))
	for _, value := range values {
		score := fuzzy.Score(value, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{value: value, score: score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, ACSuggestion{Display: definition.Trigger + item.value, Value: item.value})
	}
	return result
}

// searchAutocompleteCandidateTasks returns exactly the tasks search itself can
// inspect: the loaded scope, narrowed to the Focus boundary whenever one is
// active. Completion reads the same authority the projection does, so it can
// never offer a tag no result in this Focus carries.
func (m *OutlinerModel) searchAutocompleteCandidateTasks() []*models.Task {
	if m == nil {
		return nil
	}
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	bounded := m.searchFocusCandidates(projects)
	tasks := make([]*models.Task, 0)
	for _, project := range projects {
		if project == nil {
			continue
		}
		population := project.Tasks
		if focused, ok := bounded[project.ID]; ok {
			population = focused
		}
		tasks = append(tasks, population...)
	}
	return tasks
}

// searchCandidateTagNames collects the normalized tags in scope. Normalization
// is what makes the offered value the one the predicate compares against:
// #source:github is stored and matched as source-github.
func (m *OutlinerModel) searchCandidateTagNames() []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, task := range m.searchAutocompleteCandidateTasks() {
		if task == nil {
			continue
		}
		for _, raw := range task.Tags {
			tag, err := models.NormalizeTaskTag(raw)
			if err != nil || seen[tag] {
				continue
			}
			seen[tag] = true
			names = append(names, tag)
		}
	}
	return names
}

func (m *OutlinerModel) searchCandidateContextNames() []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, task := range m.searchAutocompleteCandidateTasks() {
		if task == nil || task.Context == "" {
			continue
		}
		context, err := models.NormalizeTaskContext(task.Context)
		if err != nil || seen[context] {
			continue
		}
		seen[context] = true
		names = append(names, context)
	}
	return names
}

// handleSearchAutocompleteKey claims only the keys an open dropdown owns.
// Everything else falls through on purpose: Enter still reaches the ordinary
// result landing, and every printable key — j and k among them — still types
// into the query, so completion never makes a letter unreachable.
func (m *OutlinerModel) handleSearchAutocompleteKey(key string) bool {
	if m == nil || !m.searchAutocomplete.active() {
		return false
	}
	switch key {
	case "tab":
		m.acceptSearchSuggestion()
		return true
	case "esc":
		// The first Esc closes the suggestions; the next one reaches the search
		// cascade below and clears or restores the projection.
		m.searchAutocomplete.clear()
		return true
	case "up", "ctrl+p", "down", "ctrl+n":
		return m.searchAutocomplete.ac.HandleKey(key, nil)
	default:
		return false
	}
}

// acceptSearchSuggestion replaces the token under the caret and preserves
// everything after it, so completing a sigil in the middle of a settled query
// never discards the tokens that follow.
func (m *OutlinerModel) acceptSearchSuggestion() {
	state := &m.searchAutocomplete
	if !state.ac.IsActive() {
		return
	}
	value := m.SearchInputValue()
	cursorByte := m.searchAutocompleteCursorByte()
	selected := state.ac.Selected()
	accepted := state.ac.BuildAcceptedInputAt(value, cursorByte, state.triggerText)
	inserted := value[:state.ac.TriggerPos] + state.triggerText + selected.Value + " "

	m.SetSearchInputValue(accepted)
	m.SetSearchInputCursor(utf8.RuneCountInString(inserted))
	state.clear()
	m.SetSearchQuery(accepted)
	// The caret now sits after the inserted token's trailing space, so the refresh
	// finds no active token and the pane stays closed until the next sigil.
	m.refreshSearchAutocomplete()
}

// renderSearchAutocompleteDropdown draws the bounded pane. The caller measures
// the returned string rather than predicting its height, because
// RenderDropdownWithLimit decides how many of its rows go to overflow hints.
func (m *OutlinerModel) renderSearchAutocompleteDropdown(width int) string {
	if m == nil || !m.searchAutocomplete.active() {
		return ""
	}
	budget := min(outlinerSearchSuggestionLimit, max(m.contentHeight()-2, 1))
	rendered := m.searchAutocomplete.ac.RenderDropdownWithLimit(outlinerSearchDropdownIndent, budget)
	if rendered == "" {
		return ""
	}
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lines[i] = truncateToVisualWidth(line, max(width, 1))
	}
	return strings.Join(lines, "\n")
}

// searchAutocompleteGuidance describes the suggestion controls while the pane is
// open. It is additive: Enter and Esc keep saying what they do for the query, so
// closing the pane does not read as losing the search.
func (m *OutlinerModel) searchAutocompleteGuidance() GuidanceSpec {
	return GuidanceSpec{
		Hints: append(
			requiredGuidanceHints("Tab accept", "Esc close suggestions"),
			optionalGuidanceHints(10, "↑/↓ or Ctrl+N/P select", "Enter apply search")...,
		),
		Rows: 2,
	}
}
