package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

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
	projects := m.loadedProjects
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
	if scope := m.workModeScopeProjects(); len(scope) == 1 {
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
// value and the human-readable name, so a project the dropdown's five rows would
// otherwise hide is reachable by typing either one.
func (m *OutlinerModel) searchProjectSuggestions(
	definition textgrammar.Definition,
	partial string,
) []grammar.ACSuggestion {
	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
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
