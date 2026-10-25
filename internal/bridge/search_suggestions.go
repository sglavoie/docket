package bridge

import (
	"slices"
	"strings"
	"unicode"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

type searchSuggestion struct {
	Token  string `json:"token"`
	Search string `json:"search"`
}

type searchSuggestionsResult struct {
	Suggestions []searchSuggestion `json:"suggestions"`
}

// querySearchSuggestions completes the last token in the mobile search box.
// The active token is excluded from the candidate query, so typing an unfinished
// tag cannot remove the very tasks that supply its completion vocabulary.
func (b *Bridge) querySearchSuggestions(request Request) (any, error) {
	var payload tasksPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	result := searchSuggestionsResult{Suggestions: []searchSuggestion{}}
	start := 0
	for offset, r := range payload.Search {
		if unicode.IsSpace(r) {
			start = offset + len(string(r))
		}
	}
	prefix, active := payload.Search[:start], payload.Search[start:]
	if active == "" {
		return result, nil
	}
	var definition textgrammar.Definition
	for _, candidate := range textgrammar.Definitions(textgrammar.SurfaceSearch) {
		if strings.HasPrefix(active, candidate.Trigger) && len(candidate.Trigger) > len(definition.Trigger) {
			definition = candidate
		}
	}
	if definition.Trigger == "" {
		return result, nil
	}
	partial := strings.ToLower(strings.TrimPrefix(active, definition.Trigger))
	payload.Search = prefix
	query, err := b.tasksQuery(payload)
	if err != nil {
		return nil, err
	}
	values := slices.Clone(definition.Values)
	values = append(values, textgrammar.SearchPresetValues(definition.Field)...)
	values = append(values, textgrammar.SearchExampleValues(definition.Field)...)
	for _, clear := range definition.Clears {
		if clear.Surfaces&textgrammar.SurfaceSearch != 0 {
			values = append(values, clear.Value)
		}
	}
	if definition.Field == textgrammar.FieldTag || definition.Field == textgrammar.FieldRemoveTag ||
		definition.Field == textgrammar.FieldContext || definition.Field == textgrammar.FieldRemoveContext {
		partial = models.NormalizeTag(partial)
	}
	dynamic, err := b.searchSuggestionValues(query, definition.Field)
	if err != nil {
		return nil, err
	}
	values = append(values, dynamic...)

	slices.Sort(values)
	values = slices.Compact(values)
	used := textgrammar.ParseSearch(prefix).TokensFor(definition.Field)
	for _, value := range values {
		token := definition.Trigger + value
		if !strings.HasPrefix(strings.ToLower(value), partial) || strings.EqualFold(token, active) {
			continue
		}
		if slices.ContainsFunc(used, func(t textgrammar.Token) bool { return strings.EqualFold(t.Value, value) }) {
			continue
		}
		// Suggest only values the bridge can evaluate as a clause; the grammar
		// also teaches richer desktop forms that its durable query cannot read.
		if _, err := models.ParseQuery(searchAlias(token)); err != nil {
			continue
		}
		search := prefix + token + " "
		result.Suggestions = append(result.Suggestions, searchSuggestion{Token: token, Search: search})
	}
	return result, nil
}

func (b *Bridge) searchSuggestionValues(query models.Query, field textgrammar.Field) ([]string, error) {
	var values []string
	switch field { //nolint:exhaustive // Dynamic vocabulary comes from the current list, excluding the unfinished token.
	case textgrammar.FieldTag, textgrammar.FieldRemoveTag, textgrammar.FieldContext, textgrammar.FieldRemoveContext,
		textgrammar.FieldProject, textgrammar.FieldRemoveProject,
		textgrammar.FieldDependency, textgrammar.FieldRemoveDependency,
		textgrammar.FieldDue, textgrammar.FieldRemoveDue,
		textgrammar.FieldDefer, textgrammar.FieldRemoveDefer:
		answer, queryErr := b.workspace.runtime.Query(query)
		if queryErr != nil {
			return nil, queryErr
		}
		for _, item := range answer.Items {
			switch field { //nolint:exhaustive // Only dynamic fields reach this loop.
			case textgrammar.FieldTag, textgrammar.FieldRemoveTag:
				values = append(values, item.Task.Tags...)
			case textgrammar.FieldContext, textgrammar.FieldRemoveContext:
				if item.Task.Context != "" {
					values = append(values, item.Task.Context)
				}
			case textgrammar.FieldProject, textgrammar.FieldRemoveProject:
				values = append(values, item.Project.Slug)
			case textgrammar.FieldDependency, textgrammar.FieldRemoveDependency:
				for _, reference := range item.Task.Dependencies {
					values = append(values, reference.Locator())
				}
			case textgrammar.FieldDue, textgrammar.FieldRemoveDue:
				if item.Task.DueDate != nil {
					values = append(values, item.Task.DueDate.Format(models.QueryDateLayout))
				}
			case textgrammar.FieldDefer, textgrammar.FieldRemoveDefer:
				if item.Task.DeferredUntil != nil {
					values = append(values, item.Task.DeferredUntil.In(b.workspace.runtime.Now().Location()).Format(models.QueryDateLayout))
				}
			}
		}
	}
	return values, nil
}
