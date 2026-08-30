package textgrammar

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// searchDuePresetNames are the relational due values only search understands.
// They are deliberately not FieldDue.Values, which is surface-blind and would
// advertise @overdue while capturing.
var searchDuePresetNames = []string{"overdue", "today", "week", "none"}

// searchDeferPresetNames are the relational deferral values only search
// understands. "any" and "none" split the population the deferral instant
// itself cannot express.
var searchDeferPresetNames = []string{"any", "none"}

// searchDueExampleValues are the parametric search-only due forms that no fixed
// preset name can express. The "+Nh" hour window is illustrated with one
// concrete value so `?` help can advertise it the way the preset buckets are,
// without it ever reaching the shared capture Examples where a due date refuses
// the hour offset.
var searchDueExampleValues = []string{"+4h"}

// searchPriorityExampleValues illustrates the threshold suffix the fixed value
// list cannot name. Values holds 1, 2, and 3 because those are the priorities a
// task can carry; "and above" is a question, so it is advertised here.
var searchPriorityExampleValues = []string{"2+"}

// SearchPresetValues returns the relational values only search understands for a
// field, and nil for a field that has none. It exists so a surface teaching the
// search vocabulary reads the same list ParseSearch matches against rather than
// hand-authoring a second copy that can drift.
func SearchPresetValues(field Field) []string {
	switch negatedField(field) { //nolint:exhaustive // Only the two relational fields have search-only values.
	case FieldDue:
		return slices.Clone(searchDuePresetNames)
	case FieldDefer:
		return slices.Clone(searchDeferPresetNames)
	default:
		return nil
	}
}

// SearchExampleValues returns concrete, search-only example values for a field
// whose vocabulary includes a parametric form the fixed preset list cannot
// name. It is the discoverability companion to SearchPresetValues: the "+Nh"
// due window is search-only, so it is advertised here rather than in the shared
// capture Examples.
func SearchExampleValues(field Field) []string {
	switch negatedField(field) { //nolint:exhaustive // Only the two parametric search-only forms are advertised here.
	case FieldDue:
		return slices.Clone(searchDueExampleValues)
	case FieldPriority:
		return slices.Clone(searchPriorityExampleValues)
	default:
		return nil
	}
}

// ParseSearch parses one live search-box value on SurfaceSearch and adds the
// diagnostics only search can raise: the reserved context name and the
// placement sigils, which are grammar everywhere else and refusals here. It
// never returns an error, because search runs per keystroke.
func ParseSearch(input string) Result {
	result := Parse(input, SurfaceSearch)
	result.Diagnostics = append(result.Diagnostics, searchDiagnostics(result)...)
	return result
}

// searchDiagnostics names every token search recognized but cannot honor. It
// switches on the negated field rather than the token's own, because a negation
// carries no vocabulary of its own: -$ready is wrong for exactly the reason
// $ready is, and one arm should say so. The reported field stays the token's, so
// a consumer can still tell which spelling raised the message, and the message
// itself interpolates token.Form.
func searchDiagnostics(result Result) []Diagnostic {
	var diagnostics []Diagnostic
	for _, token := range result.Tokens {
		switch negatedField(token.Field) { //nolint:exhaustive // Only the fields whose values can be recognized and wrong raise a diagnostic.
		case FieldPriority:
			// The threshold reader, not ParsePriority, decides here: !2+ is a legal
			// search and would otherwise be refused as an invalid priority.
			if _, ok := ParseSearchPriority(token.Value); !ok {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   token.Field,
					Message: InvalidPriorityError(token.Value).Error(),
				})
			}
		case FieldDestination:
			if _, ok := ParseDestination(token.Value); !ok {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   token.Field,
					Message: InvalidDestinationError(token.Value).Error(),
				})
			}
		case FieldContext:
			if token.Value == clearValue {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   token.Field,
					Message: fmt.Sprintf("%s%s clears a context and matches nothing", token.Form, clearValue),
				})
			}
		case FieldDependency:
			// ~0 is a real form on edit and outline, where a stored set exists to
			// empty. Search holds no set, so the value is named rather than silently
			// compiled into a locator that could never match.
			if token.Value == clearValue {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   token.Field,
					Message: fmt.Sprintf("%s%s clears dependencies with e or the outline buffer; search has no set to clear", token.Form, clearValue),
				})
			}
		case FieldDerived:
			if _, err := ParseDerived(token.Value); err != nil {
				diagnostics = append(diagnostics, Diagnostic{
					Field:   token.Field,
					Message: err.Error(),
				})
			}
		}
	}
	return append(diagnostics, placementDiagnostics(result.Text)...)
}

// refusedSearchField pairs one field search refuses by name with the sentence
// that says where the sigil does belong. The wording is per field rather than one
// shared string because the fields are refused for different reasons: placement
// is an edit act, while an attachment and a comment are creation-time or panel acts.
type refusedSearchField struct {
	field       Field
	replacement string
}

var refusedSearchFields = []refusedSearchField{
	{FieldParent, "placement is set with e"},
	{FieldChild, "placement is set with e"},
	{FieldAttachment, "attachments are added at capture or in the task's Attachments panel"},
	{FieldComment, "comments are added at capture or in the task's Comments panel"},
}

// placementDiagnostics re-scans the remainder with each refused field's own
// registry pattern. None of them reaches the search surface, so Parse leaves them
// in the text and this is the only place they can be named.
func placementDiagnostics(text string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, refused := range refusedSearchFields {
		spec, ok := registrySpecFor(refused.field)
		if !ok {
			continue
		}
		pattern, ok := registryPatternFor(refused.field)
		if !ok {
			continue
		}
		// The value is re-validated on the field's own capture surface so the
		// refusal names only what capture would have honored. Placement accepts any
		// value and so refuses on the sigil alone; an attachment does not, which is
		// what keeps a search for "&nbsp;" an ordinary query rather than a blocked one.
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 || !validTokenValue(spec, tokenValue(spec, match[1]), SurfaceCapture) {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Field: refused.field,
			Message: fmt.Sprintf(
				"%s is not a search predicate; %s",
				spec.forms[0].Text,
				refused.replacement,
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
