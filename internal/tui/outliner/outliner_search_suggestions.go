package outliner

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// outlinerSearchSuggestionLimit is the dropdown's total row budget, counting the
// overflow indicators RenderDropdownWithLimit spends rows on. The pane sits
// between the operational title and the first task row, so every row it takes is
// a row the result list loses, which is why outlinerSearchResultFloorRows
// shrinks it on a short terminal before the list gives up its floor.
const outlinerSearchSuggestionLimit = 10

// outlinerSearchResultFloorRows is what the pane always leaves the result list:
// the pane is the one that shortens when the frame cannot hold both whole.
const outlinerSearchResultFloorRows = 5

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
	Ac grammar.AutocompleteState
	// triggerText is the accepted token's sigil, retained because a two-character
	// trigger (-# and ##, @@) cannot be recovered from the mode alone once the
	// suggestion list is built. The grammar family itself is not retained beside
	// it: singleton suppression and repeatable exclusion are decided from the
	// definition the refresh is already holding, and a second copy here would be
	// a field nothing reads.
	triggerText string
}

func (a *outlinerSearchAutocomplete) Active() bool {
	return a != nil && a.Ac.IsActive()
}

func (a *outlinerSearchAutocomplete) clear() {
	if a == nil {
		return
	}
	a.Ac.Clear()
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
func (m *OutlinerModel) RefreshSearchAutocomplete() {
	if m == nil {
		return
	}
	if !m.IsSearchMode() || m.Search.historyPicker != nil {
		m.Search.Autocomplete.clear()
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
		m.Search.Autocomplete.clear()
		return
	}

	// The settled query prefix is what the grammar has already accepted. A
	// singleton field is suppressed once it appears there, and a repeatable one
	// keeps every value it has not used yet. Cardinality is read from the
	// definition rather than from a parallel field list.
	prefix := textgrammar.Parse(value[:triggerPos], textgrammar.SurfaceSearch)
	if definition.Cardinality == textgrammar.CardinalitySingleton &&
		len(prefix.TokensFor(definition.Field)) > 0 {
		m.Search.Autocomplete.clear()
		return
	}

	items := m.searchAutocompleteSuggestions(definition, partial)
	if definition.Cardinality == textgrammar.CardinalityRepeatable {
		items = grammar.FilterUsedTagSuggestions(items, prefix.TokensFor(definition.Field))
	}

	m.Search.Autocomplete.triggerText = definition.Trigger
	m.Search.Autocomplete.Ac.UpdateSuggestions(mode, triggerPos, items)
	if !m.Search.Autocomplete.Ac.IsActive() {
		// The active token no longer has anything to offer, so the retained
		// trigger would outlive the list it described.
		m.Search.Autocomplete.clear()
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
	case textgrammar.FieldAttachment, textgrammar.FieldRemoveAttachment:
		// Presence is the whole search vocabulary, so the capture locator
		// examples are never offered here.
		return searchStaticSuggestions(definition, textgrammar.SearchPresetValues(definition.Field), partial)
	default:
		return nil
	}
}

func (m *OutlinerModel) searchAutocompleteCandidateTasks() []*models.Task {
	if m == nil {
		return nil
	}
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	bounded := m.SearchCandidateBoundary(projects)
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
// Up/Down, Ctrl+N/P, and Ctrl+J/K move the suggestion cursor, the same family
// capture's pane answers to. No printable key is claimed: the pane opens exactly
// while a sigil value is being typed, which is when j and k are most likely to be
// letters of it, so a claim here would turn "#junk" into "#unk".
func (m *OutlinerModel) HandleSearchAutocompleteKey(key string) bool {
	if m == nil || !m.Search.Autocomplete.Active() {
		return false
	}
	switch key {
	case "tab":
		m.acceptSearchSuggestion()
		return true
	case "esc":
		// The first Esc closes the suggestions; the next one reaches the search
		// cascade below and clears or restores the projection.
		m.Search.Autocomplete.clear()
		return true
	case "up", "ctrl+p", "ctrl+k", "down", "ctrl+n", "ctrl+j":
		return m.Search.Autocomplete.Ac.HandleKey(key, nil)
	default:
		return false
	}
}

// acceptSearchSuggestion replaces the token under the caret and preserves
// everything after it, so completing a sigil in the middle of a settled query
// never discards the tokens that follow.
func (m *OutlinerModel) acceptSearchSuggestion() {
	state := &m.Search.Autocomplete
	if !state.Ac.IsActive() {
		return
	}
	value := m.SearchInputValue()
	cursorByte := m.searchAutocompleteCursorByte()
	selected := state.Ac.Selected()
	accepted := state.Ac.BuildAcceptedInputAt(value, cursorByte, state.triggerText)
	inserted := value[:state.Ac.TriggerPos] + state.triggerText + selected.Value + " "

	m.SetSearchInputValue(accepted)
	m.SetSearchInputCursor(utf8.RuneCountInString(inserted))
	state.clear()
	m.SetSearchQuery(accepted)
	// The caret now sits after the inserted token's trailing space, so the refresh
	// finds no active token and the pane stays closed until the next sigil.
	m.RefreshSearchAutocomplete()
}

// searchAutocompleteRowBudget is the pane's total row allowance: the shared
// limit when the frame has room, else whatever the result floor leaves.
func (m *OutlinerModel) searchAutocompleteRowBudget() int {
	return min(outlinerSearchSuggestionLimit, max(m.ContentHeight()-2-outlinerSearchResultFloorRows, 1))
}

// renderSearchAutocompleteDropdown draws the bounded pane. The caller measures
// the returned string rather than predicting its height, because
// RenderDropdownWithLimit decides how many of its rows go to overflow hints.
func (m *OutlinerModel) renderSearchAutocompleteDropdown(width int) string {
	if m != nil && m.IsSearchMode() && m.Search.historyPicker != nil {
		return m.renderSearchHistoryPicker(width)
	}
	if m == nil || !m.Search.Autocomplete.Active() {
		return ""
	}
	budget := m.searchAutocompleteRowBudget()
	rendered := m.Search.Autocomplete.Ac.RenderDropdownWithLimit(outlinerSearchDropdownIndent, budget)
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
			chrome.OptionalGuidanceHints(10, "↑/↓ or Ctrl+J/K select", "Enter apply search")...,
		),
		Rows: 2,
	}
}

// searchDerivedVocabulary is the closed set of computed properties plus the one
// parametric form the fixed names cannot express: ?completed carries an optional
// day threshold, which is advertised through the definition's own examples rather
// than a second list maintained here.
func searchDerivedVocabulary(definition textgrammar.Definition) []string {
	return appendUnprefixedValues(slices.Clone(definition.Values), definition, definition.Examples)
}

// searchDependencySuggestions offers exactly the targets the candidate population
// already depends on, in first-referenced order.
//
// The vocabulary is built from stored references rather than from the tasks a
// dependency could name, because that is the set this predicate can match: an
// open task nobody depends on would complete into a query with no results.
// Terminal and unloaded targets are included for the same reason — a stored
// reference is matchable however closed or unreachable its target has become,
// and an unresolvable one still completes to the durable locator the gate
// compares.
func (m *OutlinerModel) searchDependencySuggestions(
	definition textgrammar.Definition,
	partial string,
) []grammar.ACSuggestion {
	return grammar.DependencySuggestions(
		m.searchDependencyCandidates(),
		partial,
		m.searchDependencyEffectiveProject(),
		definition.Trigger,
		max(m.Width()-25, 10),
	)
}

// searchDependencyCandidates collects the distinct targets the live candidates
// reference. Resolution runs over the whole loaded population rather than the
// search scope, so a scoped query still names a cross-project target by its
// qualified alias instead of falling back to a bare locator.
func (m *OutlinerModel) searchDependencyCandidates() []grammar.DependencyCandidate {
	projects := m.Population.Projects
	seen := make(map[app.TaskLocator]bool)
	candidates := make([]grammar.DependencyCandidate, 0)
	for _, task := range m.searchAutocompleteCandidateTasks() {
		if task == nil {
			continue
		}
		for _, reference := range task.Dependencies {
			locator := app.LocatorForReference(reference)
			if seen[locator] {
				continue
			}
			seen[locator] = true
			candidate := grammar.DependencyCandidate{Locator: locator}
			if project := grammar.ProjectByStableID(reference.ProjectID, projects); project != nil {
				candidate.Project = project
				candidate.Task = project.GetTaskByID(reference.TaskID)
			}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// searchDependencyEffectiveProject is the project a completed alias may stay
// compact in: the scope project when the search is scoped to exactly one, and
// nothing otherwise, which is what makes a global query complete to qualified
// aliases the global gate can resolve unambiguously.
func (m *OutlinerModel) searchDependencyEffectiveProject() *models.Project {
	if scope := m.WorkModeScopeProjects(); len(scope) == 1 {
		return scope[0]
	}
	return nil
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

func searchPrioritySuggestions(definition textgrammar.Definition, partial string) []grammar.ACSuggestion {
	result := make([]grammar.ACSuggestion, 0, len(definition.Values))
	for _, value := range definition.Values {
		priorities, ok := textgrammar.ParsePriority(value, false)
		if !ok || len(priorities) != 1 {
			continue
		}
		if partial != "" && !strings.HasPrefix(value, partial) {
			continue
		}
		result = append(result, grammar.ACSuggestion{
			Display: fmt.Sprintf("%s%s (%s)", definition.Trigger, value, models.PriorityName(priorities[0])),
			Value:   value,
		})
	}
	// !0 is the positive's search clear, read from its entry so -!0 offers it
	// too: a negation completes from its twin's vocabulary. It is named like the
	// levels, by the shared priority name the unset value carries.
	if twin, ok := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldPriority); ok {
		for _, clear := range twin.Clears {
			if partial != "" && !strings.HasPrefix(clear.Value, partial) {
				continue
			}
			result = append(result, grammar.ACSuggestion{
				Display: fmt.Sprintf("%s%s (%s)", definition.Trigger, clear.Value, models.PriorityName(models.PriorityUnset)),
				Value:   clear.Value,
			})
		}
	}
	return result
}

func searchStaticSuggestions(definition textgrammar.Definition, values []string, partial string) []grammar.ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]grammar.ACSuggestion, 0, len(values))
	for _, value := range values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, grammar.ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	return result
}

func searchDestinationSuggestions(definition textgrammar.Definition, partial string) []grammar.ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]grammar.ACSuggestion, 0, len(definition.Values))
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		destination, ok := textgrammar.ParseDestination(value)
		if !ok {
			continue
		}
		result = append(result, grammar.ACSuggestion{
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

// searchBooleanAxisSuggestions offers both halves of a stored boolean axis. The
// set value comes from Values and the unset one from the search-surface clear,
// which is the only place *unpin and =unreview are declared.
func searchBooleanAxisSuggestions(definition textgrammar.Definition, partial string) []grammar.ACSuggestion {
	partial = strings.ToLower(partial)
	result := make([]grammar.ACSuggestion, 0, len(definition.Values)+len(definition.Clears))
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, grammar.ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	for _, clear := range definition.Clears {
		if partial != "" && !strings.Contains(strings.ToLower(clear.Value), partial) {
			continue
		}
		display := definition.Trigger + clear.Value
		if clear.Description != "" {
			display += " (" + clear.Description + ")"
		}
		result = append(result, grammar.ACSuggestion{Display: display, Value: clear.Value})
	}
	return result
}

// searchProjectSuggestions offers the projects already loaded in the current
// search scope and nothing else: completion must never load a wider catalog than
// the query it is completing can match against. Within that scope the partial
// ranks the same way every other dynamic value does, over both the accepted
// value and the human-readable name, so a project the dropdown's bounded rows —
// at most outlinerSearchSuggestionLimit, fewer once searchAutocompleteRowBudget
// shrinks the pane on a short terminal — would otherwise hide is reachable by
// typing either one.
func (m *OutlinerModel) searchProjectSuggestions(
	definition textgrammar.Definition,
	partial string,
) []grammar.ACSuggestion {
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	loaded := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		loaded = append(loaded, project)
	}
	ranked := searchRankByFuzzyScore(loaded, partial, func(project *models.Project) []string {
		return []string{project.Slug, project.Name}
	})
	result := make([]grammar.ACSuggestion, 0, len(ranked))
	for _, project := range ranked {
		result = append(result, grammar.ACSuggestion{
			Display: definition.Trigger + project.Slug + " " + project.Name,
			Value:   project.Slug,
		})
	}
	return result
}

// searchDynamicSuggestions ranks live values with the shared fuzzy score and
// keeps source order for ties, so an empty partial — which scores every value
// identically — offers a list that does not reshuffle between keystrokes.
func searchDynamicSuggestions(definition textgrammar.Definition, values []string, partial string) []grammar.ACSuggestion {
	ranked := searchRankByFuzzyScore(values, partial, func(value string) []string {
		return []string{value}
	})
	result := make([]grammar.ACSuggestion, 0, len(ranked))
	for _, value := range ranked {
		result = append(result, grammar.ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	return result
}

// searchRankByFuzzyScore is the one ranking rule the dynamic suggestion sources
// share: a nonmatching candidate is removed, the rest are ordered by the shared
// fuzzy score, and equal scores keep the order the candidates were collected in.
// A candidate offering several match texts is scored by its best one, so the
// text that found it decides nothing but whether it survives.
func searchRankByFuzzyScore[T any](candidates []T, partial string, texts func(T) []string) []T {
	type scored struct {
		candidate T
		score     int
	}
	items := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		best := 0
		for _, text := range texts(candidate) {
			if score := fuzzy.Score(text, partial); score > best {
				best = score
			}
		}
		if partial != "" && best == 0 {
			continue
		}
		items = append(items, scored{candidate: candidate, score: best})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]T, 0, len(items))
	for _, item := range items {
		result = append(result, item.candidate)
	}
	return result
}

// searchAutocompleteCandidateTasks returns exactly the tasks search itself can
// inspect: the loaded scope, narrowed to the same candidate boundary the
// projection is built from — the Focus branch and the active facets. Completion
// reads the same authority, so it can never offer a value no admissible result
// carries.

// searchProjectionMatchesWithin applies an optional authoritative candidate
// boundary. A present empty entry means that project contributes no matches;
// a missing entry retains the scope-wide behavior used outside Focus.
func searchProjectionMatchesWithin(
	projects []*models.Project,
	query string,
	candidates map[string][]*models.Task,
	matcher fuzzy.Matcher,
	predicates *query.SearchPredicates,
) []outlinerSearchMatch {
	if predicates.Invalid() {
		return nil
	}
	if projectRef, shortID, exact := parseProjectShortIDQuery(query); exact {
		return exactProjectShortIDMatchesWithin(projects, projectRef, shortID, candidates, predicates)
	}

	matches := make([]outlinerSearchMatch, 0)
	sourceIndex := 0
	for _, project := range projects {
		if project == nil || !predicates.AdmitsProject(project) {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for _, task := range tasks {
			if task == nil || !predicates.AdmitsPopulation(task) {
				sourceIndex++
				continue
			}
			if !predicates.Admits(task, project) {
				sourceIndex++
				continue
			}
			fieldRank, score, source, ok := scoreSearchProjectionTask(task, project, query, matcher)
			if ok {
				matches = append(matches, outlinerSearchMatch{
					task: task, project: project, fieldRank: fieldRank,
					fuzzyScore: score, source: source, sourceIndex: sourceIndex,
				})
			}
			sourceIndex++
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		if matches[i].fuzzyScore != matches[j].fuzzyScore {
			return matches[i].fuzzyScore > matches[j].fuzzyScore
		}
		return matches[i].sourceIndex < matches[j].sourceIndex
	})
	return matches
}

// scoreSearchProjectionTask ranks one task against the free-text remainder in
// the declared field tiers. Each tier is scored on its own text, so a hit is
// attributable to exactly one source; only the last tier, details, scores the
// concatenation, and it exists solely for multi-term queries whose terms are
// satisfied across fields. Because rank sorts before score, a details hit can
// never outrank a single-field one.
func scoreSearchProjectionTask(
	task *models.Task,
	project *models.Project,
	query string,
	matcher fuzzy.Matcher,
) (int, int, MatchSource, bool) {
	if query == "" {
		return searchFieldRankTitle, 1, MatchSourceNone, true
	}

	checklistText := searchChecklistText(task)
	commentText := searchCommentText(task)
	tagText := strings.Join(task.Tags, " ")
	attachmentText := searchAttachmentText(task)
	projectText := searchProjectText(task, project)

	if score := matcher.ScoreTerms(task.Title, query); score > 0 {
		return searchFieldRankTitle, score, MatchSourceTitle, true
	}
	if score := matcher.ScoreTerms(task.Description, query); score > 0 {
		return searchFieldRankDescription, score, MatchSourceDescription, true
	}
	if score := matcher.ScoreTerms(task.ResumeNote, query); score > 0 {
		return searchFieldRankResume, score, MatchSourceResume, true
	}
	if score := matcher.ScoreTerms(checklistText, query); score > 0 {
		return searchFieldRankChecklist, score, MatchSourceChecklist, true
	}
	if score := matcher.ScoreTerms(commentText, query); score > 0 {
		return searchFieldRankComment, score, MatchSourceComment, true
	}
	// Tags and context share a rank, so the better score wins and an exact tie
	// falls to the declared field order: tags precede context.
	tagScore := matcher.ScoreTerms(tagText, query)
	contextScore := matcher.ScoreTerms(task.Context, query)
	if tagScore > 0 || contextScore > 0 {
		if tagScore >= contextScore {
			return searchFieldRankTagContext, tagScore, MatchSourceTag, true
		}
		return searchFieldRankTagContext, contextScore, MatchSourceContext, true
	}
	if score := matcher.ScoreTerms(attachmentText, query); score > 0 {
		return searchFieldRankAttachment, score, MatchSourceAttachment, true
	}
	if score := matcher.ScoreTerms(projectText, query); score > 0 {
		return searchFieldRankProject, score, MatchSourceProject, true
	}

	details := searchMatchText(task, project, MatchSourceDetails)
	if score := matcher.ScoreTerms(details, query); score > 0 {
		return searchFieldRankDetails, score, MatchSourceDetails, true
	}
	return searchFieldRankTitle, 0, MatchSourceNone, false
}

// searchChecklistText is the item text alone: completion state is presentation,
// never a term a text query can match or miss on.
func searchChecklistText(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, " ")
}

func searchCommentText(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Comments))
	for _, comment := range task.Comments {
		if comment != nil {
			parts = append(parts, comment.Text)
		}
	}
	return strings.Join(parts, " ")
}

// searchAttachmentText covers both the displayed name (filename for files, the
// full URL for links) and the original path a code reference or local file was
// captured from.
func searchAttachmentText(task *models.Task) string {
	if len(task.Attachments) == 0 {
		return ""
	}
	parts := make([]string, 0, 2*len(task.Attachments))
	for _, attachment := range task.Attachments {
		parts = append(parts, attachment.Filename, attachment.OriginalPath)
	}
	return strings.Join(parts, " ")
}

// searchProjectText carries the owning project's metadata plus the qualified
// task reference, which is the fuzzy counterpart of the exact project#ID path.
func searchProjectText(task *models.Task, project *models.Project) string {
	return strings.Join([]string{
		project.Name,
		project.Slug,
		project.Description,
		fmt.Sprintf("%s#%d", project.Slug, task.ShortID),
	}, " ")
}

func parseProjectShortIDQuery(query string) (string, int, bool) {
	query = strings.TrimSpace(query)
	separator := strings.LastIndex(query, "#")
	if separator <= 0 || separator == len(query)-1 {
		return "", 0, false
	}
	shortID, err := strconv.Atoi(query[separator+1:])
	if err != nil || shortID <= 0 {
		return "", 0, false
	}
	projectRef := strings.TrimSpace(query[:separator])
	return projectRef, shortID, projectRef != ""
}

func exactProjectShortIDMatchesWithin(
	projects []*models.Project,
	projectRef string,
	shortID int,
	candidates map[string][]*models.Task,
	predicates *query.SearchPredicates,
) []outlinerSearchMatch {
	for _, project := range projects {
		if project == nil || !projectSearchRefMatches(project, projectRef) {
			continue
		}
		if !predicates.AdmitsProject(project) {
			return nil
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for sourceIndex, task := range tasks {
			if task == nil || task.ShortID != shortID {
				continue
			}
			if !predicates.AdmitsPopulation(task) {
				continue
			}
			if predicates.Admits(task, project) {
				// The qualified reference lives in the project tier, so an
				// exact hit is attributed there while keeping the exact
				// path's top rank.
				return []outlinerSearchMatch{{
					task: task, project: project, fieldRank: searchFieldRankTitle,
					fuzzyScore: 1, source: MatchSourceProject, sourceIndex: sourceIndex,
				}}
			}
		}
		return nil
	}
	return nil
}

func projectSearchRefMatches(project *models.Project, ref string) bool {
	return strings.EqualFold(ref, project.Slug) ||
		strings.EqualFold(ref, project.Name) ||
		strings.EqualFold(ref, project.ID)
}

// outlinerSearchHistoryLimit bounds the ring. It is the same size as the undo
// ledger for the same reason: far more than a working session reaches back for,
// and small enough that the oldest entries are never worth walking to.
const outlinerSearchHistoryLimit = config.SearchHistoryLimit

// outlinerSearchHistory is the bounded ring of submitted slash queries.
//
// entries is oldest first, so the newest query is the last element and Up walks
// backward from the end. index is the recall position and index == len(entries)
// means no walk is in progress, which makes the zero value inert: a
// struct-literal test model recalls nothing rather than panicking. draft holds
// the text the user had typed when the walk started, so stepping back down past
// the newest entry returns what they were writing rather than an empty input.
type outlinerSearchHistory struct {
	wired       bool
	persist     func([]string) error
	entries     []string
	index       int
	draft       string
	draftCursor int    // rune position to restore when returning to the draft
	notice      string // local recall feedback, never persisted or added to message history
}

// record appends a submitted query. An earlier identical entry is removed rather
// than left in place, so the ring holds each distinct query once and a query
// reused all afternoon stays one press away instead of filling the walk with
// copies of itself. Submitting also ends any walk in progress: the query now in
// the input is the newest one, so the next Up starts from it.
func (h *outlinerSearchHistory) record(query string) {
	if h == nil {
		return
	}
	h.resetRecall()
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	for i, entry := range h.entries {
		if entry == query {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	h.entries = append(h.entries, query)
	if len(h.entries) > outlinerSearchHistoryLimit {
		h.entries = append([]string(nil), h.entries[len(h.entries)-outlinerSearchHistoryLimit:]...)
	}
	h.index = len(h.entries)
}

// resetRecall ends a walk without touching the entries. Opening the input and
// typing into it both land here, so a walk lasts exactly as long as the user
// keeps pressing Up and Down.
func (h *outlinerSearchHistory) resetRecall() {
	if h == nil {
		return
	}
	h.index = len(h.entries)
	h.draft = ""
	h.draftCursor = 0
	h.notice = ""
}

// older steps one entry back and reports the query to install. The first step of
// a walk sets the caller's current text aside as both draft and smart-case prefix.
// Only entries beginning with that prefix are visited in either direction. The oldest entry is
// a floor: pressing Up there reports nothing rather than wrapping, because a
// wrap would silently hand back the newest query as if it were older.
func (h *outlinerSearchHistory) older(current string, cursor int) (string, bool) {
	if h == nil {
		return "", false
	}
	if len(h.entries) == 0 {
		h.notice = "No search history yet"
		return "", false
	}
	if h.index >= len(h.entries) {
		h.draft = current
		h.draftCursor = cursor
		h.index = len(h.entries)
	}
	for i := h.index - 1; i >= 0; i-- {
		if h.matchesPrefix(h.entries[i]) {
			h.index = i
			h.notice = h.positionLabel()
			return h.entries[i], true
		}
	}
	h.notice = "No history matches this prefix"
	if h.index < len(h.entries) {
		h.notice = "Oldest matching query"
	}
	return "", false
}

func (h *outlinerSearchHistory) positionLabel() string {
	position, total := 0, 0
	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.matchesPrefix(h.entries[i]) {
			total++
			if i == h.index {
				position = total
			}
		}
	}
	return fmt.Sprintf("History %d of %d", position, total)
}

func (h *outlinerSearchHistory) matchesPrefix(entry string) bool {
	if fuzzy.CaseSensitiveQuery(h.draft) {
		return strings.HasPrefix(entry, h.draft)
	}
	return strings.HasPrefix(strings.ToLower(entry), strings.ToLower(h.draft))
}

// newer steps one entry forward, ending on the draft the walk set aside. Down
// outside a walk reports nothing, so it never clears an input the user typed.
func (h *outlinerSearchHistory) newer() (string, bool) {
	if h == nil || h.index >= len(h.entries) {
		return "", false
	}
	for i := h.index + 1; i < len(h.entries); i++ {
		if h.matchesPrefix(h.entries[i]) {
			h.index = i
			h.notice = h.positionLabel()
			return h.entries[i], true
		}
	}
	draft := h.draft
	h.resetRecall()
	return draft, true
}

// recordSearchHistoryEntry retains a query the user submitted with Enter.
func (m *OutlinerModel) recordSearchHistoryEntry(query string) error {
	if m == nil {
		return nil
	}
	m.Search.history.record(query)
	if m.Search.history.persist != nil && strings.TrimSpace(query) != "" {
		return m.Search.history.persist(slices.Clone(m.Search.history.entries))
	}
	return nil
}

// ConfigureSearchHistory loads once, even if the shell rewires this view later.
// A nil persistence seam keeps standalone views usable without a runtime.
func (m *OutlinerModel) ConfigureSearchHistory(entries []string, persist func([]string) error) {
	h := &m.Search.history
	if h.wired {
		return
	}
	for _, query := range entries {
		h.record(query)
	}
	h.resetRecall()
	h.persist, h.wired = persist, true
}

// ClearSearchHistory publishes the clear before forgetting local entries. A
// failed write leaves recall intact, and neither path changes the active query.
func (m *OutlinerModel) ClearSearchHistory() error {
	h := &m.Search.history
	if h.persist != nil {
		if err := h.persist(nil); err != nil {
			return err
		}
	}
	h.entries = nil
	h.resetRecall()
	return nil
}

// recallSearchHistory claims Up and Down inside the slash input, and only there.
// It is called from the key cascade below the suggestion pane's own claim, so an
// open pane keeps both keys for its cursor and one Esc gives them back to the
// walk. Boundary presses are consumed without changing the input; every other
// key falls through to ordinary input handling.
func (m *OutlinerModel) recallSearchHistory(key string) bool {
	if m == nil || !m.IsSearchMode() {
		return false
	}
	var (
		value string
		ok    bool
	)
	draftCursor := m.Search.history.draftCursor
	switch key {
	case "up":
		value, ok = m.Search.history.older(m.SearchInputValue(), m.SearchInputCursor())
	case "down":
		value, ok = m.Search.history.newer()
	default:
		return false
	}
	if !ok {
		// Claim boundary presses too: they must not reset the walk or its notice
		// through the ordinary text-input path.
		return true
	}
	m.applyRecalledSearchQuery(value)
	if key == "down" && m.Search.history.index == len(m.Search.history.entries) {
		m.SetSearchInputCursor(draftCursor)
	}
	return true
}

// applyRecalledSearchQuery installs a recalled query exactly as typing it would:
// the same live query update and the same rebuild the ordinary input path makes,
// with the caret at the end so the next character extends it. The suggestion
// pane is cleared rather than refreshed, because a recalled query replaces the
// token the pane was describing whole, and leaving Up and Down with the pane
// would end the walk on its first step.
func (m *OutlinerModel) applyRecalledSearchQuery(value string) {
	m.SetSearchInputValue(value)
	m.SetSearchInputCursor(utf8.RuneCountInString(value))
	m.BaseSearchableView.SetSearchQuery(value)
	m.Search.Autocomplete.clear()
	RebuildOutlinerPreservingCursor(m)
}

// hasSearchHistory reports whether anything is there to recall, which is what
// decides if the search footer advertises the walk.
func (m *OutlinerModel) hasSearchHistory() bool {
	return m != nil && len(m.Search.history.entries) > 0
}

// searchMatchText uses the same field text as ranking, including its combined
// details tier when separate query terms occur in different fields.
func searchMatchText(task *models.Task, project *models.Project, source MatchSource) string {
	switch source {
	case MatchSourceTitle:
		return task.Title
	case MatchSourceDescription:
		return task.Description
	case MatchSourceResume:
		return task.ResumeNote
	case MatchSourceChecklist:
		return searchChecklistText(task)
	case MatchSourceComment:
		return searchCommentText(task)
	case MatchSourceTag:
		return strings.Join(task.Tags, " ")
	case MatchSourceContext:
		return task.Context
	case MatchSourceAttachment:
		return searchAttachmentText(task)
	case MatchSourceProject:
		if project != nil {
			return searchProjectText(task, project)
		}
		return ""
	case MatchSourceDetails:
		var fields []string
		for field := MatchSourceTitle; field < MatchSourceDetails; field++ {
			fields = append(fields, searchMatchText(task, project, field))
		}
		return strings.Join(fields, " ")
	default:
		return ""
	}
}

// The filter is separate from the live search input: canceling changes neither
// its text nor its caret, and choosing a query never submits or records it.
type searchHistoryPicker struct {
	input   textinput.Model
	choices grammar.AutocompleteState
}

func (m *OutlinerModel) openSearchHistoryPicker() tea.Cmd {
	p := &searchHistoryPicker{input: textinput.New()}
	p.input.Prompt = ""
	p.input.Placeholder = "Find a saved query…"
	p.input.CharLimit = 4000
	p.input.SetValue(m.SearchInputValue())
	m.Search.Autocomplete.clear()
	m.Search.historyPicker = p
	m.refreshSearchHistoryPicker()
	return p.input.Focus()
}

func (m *OutlinerModel) refreshSearchHistoryPicker() {
	p := m.Search.historyPicker
	terms := strings.Fields(strings.ToLower(p.input.Value()))
	var items []grammar.ACSuggestion
	for i := len(m.Search.history.entries) - 1; i >= 0; i-- {
		entry := m.Search.history.entries[i]
		match := true
		for _, term := range terms {
			if !strings.Contains(strings.ToLower(entry), term) {
				match = false
				break
			}
		}
		if match {
			items = append(items, grammar.ACSuggestion{Display: entry, Value: entry})
		}
	}
	p.choices.UpdateSuggestions(grammar.ACModeTag, 0, items)
}

// Forget the selected query only after its durable history has been saved.
// The picker filter, live search draft and accepted projection remain intact.
func (m *OutlinerModel) forgetSelectedSearchHistory() tea.Cmd {
	p := m.Search.historyPicker
	if !p.choices.IsActive() {
		return nil
	}
	value := p.choices.Selected().Value
	h := &m.Search.history
	entries := slices.DeleteFunc(slices.Clone(h.entries), func(entry string) bool { return entry == value })
	if h.persist != nil {
		if err := h.persist(entries); err != nil {
			return status.ErrorCmd("Search history could not be updated: %v", err)
		}
	}
	h.entries = entries
	h.resetRecall()
	m.refreshSearchHistoryPicker()
	return func() tea.Msg { return status.InfoStatusMsg("Query forgotten from search history") }
}

func (m *OutlinerModel) updateSearchHistoryPicker(msg tea.Msg) tea.Cmd {
	p := m.Search.historyPicker
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+x":
			return m.forgetSelectedSearchHistory()
		case "esc", "ctrl+r":
			m.Search.historyPicker = nil
			return nil
		case "enter", "tab":
			if p.choices.IsActive() {
				value := p.choices.Selected().Value
				m.Search.historyPicker = nil
				m.Search.history.resetRecall()
				m.applyRecalledSearchQuery(value)
			}
			return nil
		case "up", "down", "ctrl+k", "ctrl+j", "ctrl+p", "ctrl+n":
			p.choices.HandleKey(key.String(), nil)
			return nil
		}
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if before != p.input.Value() {
		m.refreshSearchHistoryPicker()
	}
	return cmd
}

func (m *OutlinerModel) renderSearchHistoryPicker(width int) string {
	p := m.Search.historyPicker
	if !p.choices.IsActive() {
		label := "No history matches"
		if !m.hasSearchHistory() {
			label = "No search history yet"
		}
		return textfit.TruncateToVisualWidth(theme.MutedStyle.Render(label), max(width, 1))
	}
	rendered := p.choices.RenderDropdownWithLimit(outlinerSearchDropdownIndent, m.searchAutocompleteRowBudget())
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lines[i] = textfit.TruncateToVisualWidth(line, max(width, 1))
	}
	return strings.Join(lines, "\n")
}
