package textgrammar

import (
	"regexp"
	"strings"
)

// Token is one recognized inline metadata token.
type Token struct {
	Field  Field
	Raw    string
	Value  string
	Form   string
	Legacy bool
}

// Result contains recognized tokens and the normalized free-text remainder.
type Result struct {
	Text   string
	Tokens []Token
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
	re      *regexp.Regexp
	field   Field
	form    string
	legacy  bool
	surface Surface
}

var tokenPatterns = []tokenPattern{
	{regexp.MustCompile(`(?i)(?:^|\s)type:(\S+)`), FieldTaskType, "type:", false, SurfaceCapture},
	{regexp.MustCompile(`(?:^|\s)\^(\S+)`), FieldTaskType, "^", false, SurfaceCapture | SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)(?:pri|priority):(\S+)`), FieldPriority, "pri:", false, SurfaceCapture | SurfaceEdit},
	{regexp.MustCompile(`(?:^|\s)!([^\s]+)`), FieldPriority, "!", false, SurfaceCapture | SurfaceEdit | SurfaceSearch},
	{regexp.MustCompile(`(?:^|\s)-#([\w][\w.-]*)`), FieldRemoveTag, "-#", false, SurfaceEdit},
	{regexp.MustCompile(`(?i)(?:^|\s)tag:([\w][\w.-]*)`), FieldTag, "tag:", false, SurfaceCapture | SurfaceEdit},
	{regexp.MustCompile(`(?:^|\s)#([\w][\w.-]*)`), FieldTag, "#", false, SurfaceCapture | SurfaceEdit | SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)due:(next\s+\w+|in\s+\d+\s+\w+|\S+)`), FieldDue, "due:", false, SurfaceCapture | SurfaceEdit},
	{regexp.MustCompile(`(?i)(?:^|\s)@(next\s+\w+|in\s+\d+\s+\w+|\S+)`), FieldDue, "@", false, SurfaceCapture | SurfaceEdit},
	{regexp.MustCompile(`(?i)(?:^|\s)@(today|tomorrow|week|overdue|none)(?:\s|$)`), FieldDue, "@", false, SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)recur:(\S+)`), FieldRecurrence, "recur:", false, SurfaceCapture},
	{regexp.MustCompile(`(?:^|\s)~(\S+)`), FieldRecurrence, "~", false, SurfaceCapture},
	{regexp.MustCompile(`(?i)(?:^|\s)(?:p|proj|project):(\S+)`), FieldProject, "p:", false, SurfaceCapture | SurfaceSearch},
	{regexp.MustCompile(`(?:^|\s)%(\S+)`), FieldProject, "%", false, SurfaceCapture},
	{regexp.MustCompile(`(?i)(?:^|\s)parent:(\S+)`), FieldParent, "parent:", false, SurfaceCapture},
	{regexp.MustCompile(`(?:^|\s)>(\S+)`), FieldParent, ">", false, SurfaceCapture},
	{regexp.MustCompile(`(?i)(?:^|\s)state:(\w+)`), FieldState, "state:", false, SurfaceCapture | SurfaceEdit | SurfaceSearch},
	{regexp.MustCompile(`(?:^|\s)\$(\w+)`), FieldState, "$", false, SurfaceCapture | SurfaceEdit | SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)has:(\w+)`), FieldHas, "has:", false, SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)no:(\w+)`), FieldNo, "no:", false, SurfaceSearch},
	{regexp.MustCompile(`(?i)(?:^|\s)is:(\w+)`), FieldIs, "is:", false, SurfaceSearch},
}

// Parse extracts every token enabled for surface using the one shared grammar.
func Parse(input string, surface Surface) Result {
	result := Result{}
	remaining := input
	for _, pattern := range tokenPatterns {
		if pattern.surface&surface == 0 {
			continue
		}
		var tokens []Token
		remaining, tokens = extractTokens(remaining, pattern)
		result.Tokens = append(result.Tokens, tokens...)
	}
	result.Text = NormalizeText(remaining)
	return result
}

func extractTokens(input string, pattern tokenPattern) (string, []Token) {
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
		rebuilt.WriteByte(' ')
		raw := strings.TrimSpace(input[match[0]:match[1]])
		tokens = append(tokens, Token{
			Field:  pattern.field,
			Raw:    raw,
			Value:  input[match[2]:match[3]],
			Form:   pattern.form,
			Legacy: pattern.legacy,
		})
		last = match[1]
	}
	rebuilt.WriteString(input[last:])
	return rebuilt.String(), tokens
}

// NormalizeText trims and collapses whitespace in the free-text remainder.
func NormalizeText(input string) string {
	return whitespaceRE.ReplaceAllString(strings.TrimSpace(input), " ")
}
