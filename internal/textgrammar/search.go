package textgrammar

import (
	"fmt"
	"regexp"
	"strings"
)

// searchDuePresetNames are the relational due values only search understands.
// They are deliberately not FieldDue.Values, which is surface-blind and would
// advertise @overdue while capturing.
var searchDuePresetNames = []string{"overdue", "today", "week", "none"}

// ParseSearch parses one live search-box value on SurfaceSearch and adds the
// diagnostics only search can raise: the reserved context name and the
// placement sigils, which are grammar everywhere else and refusals here. It
// never returns an error, because search runs per keystroke.
func ParseSearch(input string) Result {
	result := Parse(input, SurfaceSearch)
	result.Diagnostics = append(result.Diagnostics, searchDiagnostics(result)...)
	return result
}

func searchDiagnostics(result Result) []Diagnostic {
	var diagnostics []Diagnostic
	for _, token := range result.Tokens {
		switch token.Field { //nolint:exhaustive // Only the fields whose values can be recognized and wrong raise a diagnostic.
		case FieldPriority:
			if _, ok := ParsePriority(token.Value, false); !ok {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   FieldPriority,
					Message: InvalidPriorityError(token.Value).Error(),
				})
			}
		case FieldDestination:
			if _, ok := ParseDestination(token.Value); !ok {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   FieldDestination,
					Message: InvalidDestinationError(token.Value).Error(),
				})
			}
		case FieldContext:
			if token.Value == clearValue {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   FieldContext,
					Message: fmt.Sprintf("%s%s clears a context and matches nothing", token.Form, clearValue),
				})
			}
		}
	}
	return append(diagnostics, placementDiagnostics(result.Text)...)
}

// placementDiagnostics re-scans the remainder with the registry's own parent and
// child patterns. Neither field reaches the search surface, so Parse leaves both
// in the text and this is the only place they can be named.
func placementDiagnostics(text string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, field := range []Field{FieldParent, FieldChild} {
		spec, ok := registrySpecFor(field)
		if !ok {
			continue
		}
		pattern, ok := registryPatternFor(field)
		if !ok || !pattern.MatchString(text) {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Field: field,
			Message: fmt.Sprintf(
				"%s is not a search predicate; placement is set with e",
				spec.forms[0].Text,
			),
		})
	}
	return diagnostics
}

func registryPatternFor(field Field) (*regexp.Regexp, bool) {
	for _, pattern := range tokenPatterns {
		if pattern.spec.Field == field {
			return pattern.re, true
		}
	}
	return nil, false
}

// SearchGuidance returns the first diagnostic message a parsed search query
// raised, or the empty string when the query can run.
func SearchGuidance(result Result) string {
	for _, diagnostic := range result.Diagnostics {
		if strings.TrimSpace(diagnostic.Message) != "" {
			return diagnostic.Message
		}
	}
	return ""
}
