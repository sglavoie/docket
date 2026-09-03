package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/textfit"
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
	ac grammar.AutocompleteState
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
func outlinerSearchTriggers() []grammar.ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceSearch)
	triggers := make([]grammar.ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		mode := grammar.AutocompleteModeForField(definition.Field)
		if mode == grammar.ACModeNone {
			continue
		}
		triggers = append(triggers, grammar.ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

// outlinerSearchDefinitionForMode is the reverse of the projection above. It
// returns the full definition rather than the field alone, so cardinality,
// values, and clears are read from the same entry the trigger came from.
func outlinerSearchDefinitionForMode(mode int) (textgrammar.Definition, bool) {
	if mode == grammar.ACModeNone {
		return textgrammar.Definition{}, false
	}
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		if grammar.AutocompleteModeForField(definition.Field) == mode {
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
	mode, triggerPos, partial := grammar.DetectTrigger(value[:cursorByte], outlinerSearchTriggers())
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
		items = grammar.FilterUsedTagSuggestions(items, prefix.TokensFor(definition.Field))
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
) []grammar.ACSuggestion {
	// A negation is completed from its twin's vocabulary, so each pair shares one
	// arm: -@ offers the due buckets @ offers, and the only difference is the
	// trigger each suggestion is displayed and accepted with, which comes from the
	// definition the caller already resolved.
	switch definition.Field { //nolint:exhaustive // Only the fields SurfaceSearch admits can reach a trigger here.
	case textgrammar.FieldPriority, textgrammar.FieldRemovePriority:
		return searchPrioritySuggestions(definition, partial)
	case textgrammar.FieldTag, textgrammar.FieldRemoveTag:
		return searchDynamicSuggestions(definition, m.searchCandidateTagNames(), partial)
	case textgrammar.FieldContext, textgrammar.FieldRemoveContext:
		return searchDynamicSuggestions(definition, m.searchCandidateContextNames(), partial)
	case textgrammar.FieldDue, textgrammar.FieldRemoveDue:
		return searchStaticSuggestions(definition, searchDueVocabulary(definition), partial)
	case textgrammar.FieldDefer, textgrammar.FieldRemoveDefer:
		return searchStaticSuggestions(definition, searchDeferVocabulary(definition), partial)
	case textgrammar.FieldDestination, textgrammar.FieldRemoveDestination:
		return searchDestinationSuggestions(definition, partial)
	case textgrammar.FieldPinned, textgrammar.FieldRemovePinned,
		textgrammar.FieldReview, textgrammar.FieldRemoveReview:
		return searchBooleanAxisSuggestions(definition, partial)
	case textgrammar.FieldProject, textgrammar.FieldRemoveProject:
		return m.searchProjectSuggestions(definition, partial)
	case textgrammar.FieldDependency, textgrammar.FieldRemoveDependency:
		return m.searchDependencySuggestions(definition, partial)
	case textgrammar.FieldDerived, textgrammar.FieldRemoveDerived:
		return searchStaticSuggestions(definition, searchDerivedVocabulary(definition), partial)
	default:
		return nil
	}
}

func (m *OutlinerModel) searchAutocompleteCandidateTasks() []*models.Task {
	if m == nil {
		return nil
	}
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	bounded := m.searchCandidateBoundary(projects)
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
// Everything else falls through on purpose, Enter above all: it still reaches the
// ordinary result landing rather than becoming a second acceptance.
//
// The two axes are deliberately split. Up/Down and Ctrl+N/P move the suggestion
// cursor, and j/k move the result cursor behind the pane, so the query can be
// completed and the results walked without either one borrowing the other's keys.
// j and k are ordinary characters, so this claim lasts exactly as long as the
// pane does: one Esc closes it and gives both letters back to the query.
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
	case "j", "k":
		m.moveSearchResultCursor(key)
		return true
	default:
		return false
	}
}

// moveSearchResultCursor steps the result cursor under an open suggestion pane.
// It reuses the shared list navigation and the Outliner's own non-navigable-row
// skip, so a group header is stepped over here exactly as it is outside search,
// and it touches neither the query nor the suggestion cursor. j and k move one
// row, which is the one navigation case handleListNavigation arms no flash for,
// so there is no command to lose on the way back through the key handler.
func (m *OutlinerModel) moveSearchResultCursor(key string) {
	previous := m.cursor
	handled, _ := handleListNavigation(key, &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
	if !handled {
		return
	}
	m.crossSkipNonNavigableRows(previous, nil)
	m.updateRangeSelection()
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
		lines[i] = textfit.TruncateToVisualWidth(line, max(width, 1))
	}
	return strings.Join(lines, "\n")
}

// searchAutocompleteGuidance describes the suggestion controls while the pane is
// open. It is additive: Enter and Esc keep saying what they do for the query, so
// closing the pane does not read as losing the search.
func (m *OutlinerModel) searchAutocompleteGuidance() chrome.GuidanceSpec {
	return chrome.GuidanceSpec{
		Hints: append(
			chrome.RequiredGuidanceHints("Tab accept", "Esc close suggestions"),
			chrome.OptionalGuidanceHints(10, "↑/↓ or Ctrl+N/P select", "j/k results", "Enter apply search")...,
		),
		Rows: 2,
	}
}
