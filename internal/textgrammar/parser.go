package textgrammar

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
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
	default:
		return false
	}
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
