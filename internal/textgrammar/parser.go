package textgrammar

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// Token is one recognized inline metadata token.
type Token struct {
	Field Field
	Value string
	Form  string
}

// Result contains recognized tokens and the normalized free-text remainder.
type Result struct {
	Text        string
	Tokens      []Token
	Diagnostics []Diagnostic
}

// Diagnostic describes a precise parser validation failure without coupling
// the grammar package to any one UI's error type.
type Diagnostic struct {
	Field   Field
	Message string
}

// TokensFor returns the recognized tokens for one field in input order.
func (r Result) TokensFor(field Field) []Token {
	result := make([]Token, 0)
	for _, token := range r.Tokens {
		if token.Field == field {
			result = append(result, token)
		}
	}
	return result
}

var whitespaceRE = regexp.MustCompile(`\s+`)

type tokenPattern struct {
	re   *regexp.Regexp
	spec definitionSpec
}

var tokenPatterns = buildTokenPatterns()

func buildTokenPatterns() []tokenPattern {
	result := make([]tokenPattern, len(registry))
	for i, spec := range registry {
		result[i] = tokenPattern{re: regexp.MustCompile(spec.pattern), spec: spec}
	}
	return result
}

// Parse extracts every token enabled for surface using the one shared grammar.
func Parse(input string, surface Surface) Result {
	result := Result{}
	remaining := input
	for _, pattern := range tokenPatterns {
		if pattern.spec.Surfaces&surface == 0 {
			continue
		}
		var tokens []Token
		remaining, tokens = extractTokens(remaining, pattern, surface)
		result.Tokens = append(result.Tokens, tokens...)
	}
	result.Text = NormalizeText(remaining)
	result.Diagnostics = duplicateSingletonDiagnostics(result.Tokens)
	return result
}

func extractTokens(input string, pattern tokenPattern, surface Surface) (string, []Token) {
	matches := pattern.re.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}
	var rebuilt strings.Builder
	tokens := make([]Token, 0, len(matches))
	last := 0
	for _, match := range matches {
		if len(match) < 4 || match[2] < 0 || match[3] < 0 {
			continue
		}
		rebuilt.WriteString(input[last:match[0]])
		value := input[match[2]:match[3]]
		if !validTokenValue(pattern.spec, value, surface) {
			rebuilt.WriteString(input[match[0]:match[1]])
			last = match[1]
			continue
		}
		rebuilt.WriteByte(' ')
		tokens = append(tokens, Token{
			Field: pattern.spec.Field,
			Value: value,
			Form:  pattern.spec.Trigger,
		})
		last = match[1]
	}
	rebuilt.WriteString(input[last:])
	return rebuilt.String(), tokens
}

func duplicateSingletonDiagnostics(tokens []Token) []Diagnostic {
	first := make(map[Field]Token)
	diagnosed := make(map[Field]bool)
	var result []Diagnostic
	for _, token := range tokens {
		spec, ok := registrySpecFor(token.Field)
		if !ok || spec.Cardinality != CardinalitySingleton {
			continue
		}
		previous, exists := first[token.Field]
		if !exists {
			first[token.Field] = token
			continue
		}
		if diagnosed[token.Field] {
			continue
		}
		diagnosed[token.Field] = true
		result = append(result, Diagnostic{
			Field: token.Field,
			Message: fmt.Sprintf(
				"duplicate %s: %s%s conflicts with earlier %s%s; only one %s value is allowed",
				token.Field,
				token.Form,
				token.Value,
				previous.Form,
				previous.Value,
				spec.forms[0].Text,
			),
		})
	}
	return result
}

func validTokenValue(spec definitionSpec, value string, surface Surface) bool {
	switch spec.validation {
	case validateAny:
		if surface == SurfaceSearch {
			return searchValueViable(spec, value)
		}
		return true
	case validatePriority:
		// Keep invalid priority-like tokens in the parsed token stream so every
		// consumer can return the registry-backed diagnostic instead of silently
		// folding a typo such as !9 into the task title.
		if slices.Contains(spec.Values, value) || slices.Contains(spec.RetiredValues, value) {
			return true
		}
		if value == clearValue && surface != SurfaceEdit {
			return false
		}
		// Search tokenizes any numeric priority so !9 can be diagnosed, and
		// leaves !important as prose.
		if surface == SurfaceSearch {
			return isAllDigits(value)
		}
		// Clear operations are enabled only on their exact surface. A combined
		// surface retains capture's stricter grammar.
		if surface == SurfaceEdit {
			for _, clear := range spec.Clears {
				if clear.Surfaces&surface != 0 && clear.Value == value {
					return true
				}
			}
		}
		return surface == SurfaceEdit && value != ""
	case validateTag:
		return validTagTokenValue(value)
	case validatePin:
		// The clear value tokenizes on every surface, including the ones that do
		// not advertise it: the consumer refuses it with a message, where the
		// tokenizer could only fold the word silently into the title.
		if slices.Contains(spec.Values, value) {
			return true
		}
		return slices.ContainsFunc(spec.Clears, func(clear ClearOperation) bool {
			return clear.Value == value
		})
	case validateAttachment:
		return validAttachmentTokenValue(value)
	default:
		return false
	}
}

// validAttachmentTokenValue decides lexically whether an & token names something
// attachable. It is deliberately lexical only: this package must not import
// internal/storage, and whether a path exists is the application layer's
// question. An unrecognized value is not consumed, the validatePin behavior
// rather than priority's, because & is ordinary prose ("R&D", "Q&A", "&nbsp;").
func validAttachmentTokenValue(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return true
	}
	if strings.Contains(value, "/") {
		return true
	}
	// A bare name:digits shape is a code reference only when the name part looks
	// like a file. Without that clause "&10:30 standup" would attach file 10 at
	// line 30, while "&main.go:42" must still land.
	if index := strings.LastIndex(value, ":"); index > 0 && isAllDigits(value[index+1:]) {
		return strings.Contains(value[:index], ".")
	}
	return false
}

// searchValueViable decides whether a free-form token shape is a search
// predicate at all. Search runs over prose the user may simply be looking for,
// so an unresolvable value stays in the text rather than becoming a gate that
// matches nothing.
func searchValueViable(spec definitionSpec, value string) bool {
	switch spec.Field {
	case FieldDestination:
		if _, ok := ParseDestination(value); ok {
			return true
		}
		return IsRetiredDestination(value)
	case FieldDue:
		if slices.Contains(searchDuePresetNames, strings.ToLower(strings.TrimSpace(value))) {
			return true
		}
		_, err := util.ParseCalendarDate(value)
		return err == nil
	case FieldDefer:
		if slices.Contains(searchDeferPresetNames, strings.ToLower(strings.TrimSpace(value))) {
			return true
		}
		_, err := util.ParseDeferInstant(value)
		return err == nil
	case FieldProject:
		return strings.TrimSpace(value) != ""
	default:
		return true
	}
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validTagTokenValue(value string) bool {
	for segment := range strings.SplitSeq(value, ":") {
		if segment == "" {
			return false
		}
		for i, r := range segment {
			if i == 0 && (r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				continue
			}
			if i > 0 && (r == '_' || r == '.' || r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				continue
			}
			return false
		}
	}
	return value != ""
}

// NormalizeText trims and collapses whitespace in the free-text remainder.
func NormalizeText(input string) string {
	return whitespaceRE.ReplaceAllString(strings.TrimSpace(input), " ")
}
