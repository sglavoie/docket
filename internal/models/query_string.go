package models

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// String renders the canonical clause string: every clause in canonical field
// order, rendered in the sigil vocabulary, joined by single spaces.
//
// The three axes are deliberately absent. Mode, Scope, and Sort persist beside
// the string rather than inside it, which is what lets a stored query be
// compared, deduplicated, and shown as one line without a reader having to
// separate a predicate from a presentation choice.
func (q Query) String() string {
	canonical := q.Canonical()
	parts := make([]string, 0, len(canonical.Clauses))
	for _, clause := range canonical.Clauses {
		parts = append(parts, clause.String())
	}
	return strings.Join(parts, " ")
}

// String renders one clause. Negation is always the leading '-', on every field,
// so the canonical form has exactly one spelling for "not this" and a reader
// never has to know which fields carry a negative word instead.
func (c QueryClause) String() string {
	token := c.token()
	if c.Polarity == QueryPolarityNegative {
		return "-" + token
	}
	return token
}

func (c QueryClause) token() string {
	switch c.Field {
	case QueryFieldText:
		return strconv.Quote(c.Value)
	case QueryFieldDependency:
		return "~" + c.Value
	case QueryFieldPriority:
		return "!" + querySuffixedValue(c, queryClearValue, "+")
	case QueryFieldTag:
		return "#" + querySuffixedValue(c, "", "*")
	case QueryFieldContext:
		return "##" + querySuffixedValue(c, queryClearValue, "*")
	case QueryFieldDue:
		if c.Operator == QueryOperatorNone {
			return "@" + queryClearValue
		}
		return "@" + c.Value
	case QueryFieldDefer:
		return "@@" + c.Value
	case QueryFieldState:
		return "$" + c.Value
	case QueryFieldPinned:
		return "*pin"
	case QueryFieldReview:
		return "=review"
	case QueryFieldCompleted:
		if c.Operator == QueryOperatorAtLeast {
			return "?" + string(QueryFieldCompleted) + ":" + c.Value
		}
		return "?" + string(QueryFieldCompleted)
	case QueryFieldStale, QueryFieldDeferred, QueryFieldParent, QueryFieldChild, QueryFieldLeaf,
		QueryFieldBlocked, QueryFieldBlocking:
		return "?" + string(c.Field)
	case QueryFieldProjectTag:
		if c.Operator == QueryOperatorNone {
			return "+#" + queryClearValue
		}
		return "+#" + c.Value
	case QueryFieldParked, QueryFieldArchived:
		if c.Value == QueryPopulationOnly {
			return "+" + string(c.Field) + ":" + QueryPopulationOnly
		}
		return "+" + string(c.Field)
	default:
		return ""
	}
}

// querySuffixedValue renders the three shapes a value-carrying field shares: the
// reserved unset token, a bare value, and a value wearing the suffix that marks
// its widened operator.
func querySuffixedValue(clause QueryClause, clearToken, suffix string) string {
	switch clause.Operator {
	case QueryOperatorNone:
		return clearToken
	case QueryOperatorAtLeast, QueryOperatorPrefix:
		return clause.Value + suffix
	default:
		return clause.Value
	}
}

// ParseQuery reads a canonical clause string back into a Query. It is strict by
// design: it accepts exactly what String emits and nothing more.
//
// This is not the user-facing grammar. Lenient input, diagnostics, completion,
// and the sigil surfaces belong to internal/textgrammar. What lives here is the
// closure that makes the durable string trustworthy: a value that round-trips
// through storage parses back to the query it was rendered from, with no second
// implementation of the vocabulary between them.
//
// The returned query carries clauses only. Mode, Scope, and Sort are stored
// beside the string and are the caller's to restore.
func ParseQuery(input string) (Query, error) {
	tokens, err := tokenizeQuery(input)
	if err != nil {
		return Query{}, err
	}
	clauses := make([]QueryClause, 0, len(tokens))
	for _, token := range tokens {
		clause, parseErr := parseQueryToken(token)
		if parseErr != nil {
			return Query{}, parseErr
		}
		clauses = append(clauses, clause)
	}
	return Query{Clauses: clauses}.Canonical(), nil
}

// tokenizeQuery splits on whitespace outside a double-quoted span, so the one
// field whose value may hold spaces stays one token.
func tokenizeQuery(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	quoted := false
	escaped := false
	for _, symbol := range input {
		switch {
		case escaped:
			current.WriteRune(symbol)
			escaped = false
		case quoted && symbol == '\\':
			current.WriteRune(symbol)
			escaped = true
		case symbol == '"':
			current.WriteRune(symbol)
			quoted = !quoted
		case !quoted && unicode.IsSpace(symbol):
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(symbol)
		}
	}
	if quoted || escaped {
		return nil, fmt.Errorf("query text is missing a closing quote: %q", input)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func parseQueryToken(token string) (QueryClause, error) {
	polarity := QueryPolarityPositive
	if body, found := strings.CutPrefix(token, "-"); found {
		polarity = QueryPolarityNegative
		token = body
	}
	field, operator, value, err := splitQueryToken(token)
	if err != nil {
		return QueryClause{}, err
	}
	return NewQueryClause(field, polarity, operator, value)
}

// splitQueryToken maps one rendered token back to its field, operator, and
// value. The prefixes are tried longest first, which is the same reason the
// capture grammar peels ## before # and @@ before @.
func splitQueryToken(token string) (QueryField, QueryOperator, string, error) {
	switch {
	case strings.HasPrefix(token, `"`):
		value, err := strconv.Unquote(token)
		if err != nil {
			return "", "", "", fmt.Errorf("query text %s is not a quoted string", token)
		}
		return QueryFieldText, QueryOperatorContains, value, nil
	case strings.HasPrefix(token, "~"):
		return QueryFieldDependency, QueryOperatorIs, strings.TrimPrefix(token, "~"), nil
	case strings.HasPrefix(token, "!"):
		operator, value := splitSuffixedQueryValue(strings.TrimPrefix(token, "!"), "+", QueryOperatorAtLeast)
		return QueryFieldPriority, operator, value, nil
	case strings.HasPrefix(token, "##"):
		operator, value := splitSuffixedQueryValue(strings.TrimPrefix(token, "##"), "*", QueryOperatorPrefix)
		return QueryFieldContext, operator, value, nil
	case strings.HasPrefix(token, "#"):
		operator, value := splitSuffixedQueryValue(strings.TrimPrefix(token, "#"), "*", QueryOperatorPrefix)
		return QueryFieldTag, operator, value, nil
	case strings.HasPrefix(token, "@@"):
		return QueryFieldDefer, QueryOperatorIs, strings.TrimPrefix(token, "@@"), nil
	case strings.HasPrefix(token, "@"):
		operator, value := splitSuffixedQueryValue(strings.TrimPrefix(token, "@"), "", QueryOperatorIs)
		return QueryFieldDue, operator, value, nil
	case strings.HasPrefix(token, "$"):
		return QueryFieldState, QueryOperatorIs, strings.TrimPrefix(token, "$"), nil
	case token == "*pin":
		return QueryFieldPinned, QueryOperatorPresent, "", nil
	case token == "=review":
		return QueryFieldReview, QueryOperatorPresent, "", nil
	case strings.HasPrefix(token, "?"):
		return splitDerivedQueryToken(strings.TrimPrefix(token, "?"))
	case strings.HasPrefix(token, "+#"):
		operator, value := splitSuffixedQueryValue(strings.TrimPrefix(token, "+#"), "", QueryOperatorIs)
		return QueryFieldProjectTag, operator, value, nil
	case strings.HasPrefix(token, "+"):
		return splitPopulationQueryToken(strings.TrimPrefix(token, "+"))
	default:
		return "", "", "", fmt.Errorf("unknown query token %q", token)
	}
}

// splitSuffixedQueryValue reads the reserved unset token and the optional
// operator suffix off one value. An empty suffix means the field has no widened
// operator and only the unset reading to detect.
func splitSuffixedQueryValue(value, suffix string, suffixed QueryOperator) (QueryOperator, string) {
	if value == queryClearValue {
		return QueryOperatorNone, ""
	}
	if suffix != "" {
		if body, found := strings.CutSuffix(value, suffix); found {
			return suffixed, body
		}
	}
	return QueryOperatorIs, value
}

func splitDerivedQueryToken(body string) (QueryField, QueryOperator, string, error) {
	name, value, qualified := strings.Cut(body, ":")
	field := QueryField(name)
	switch field {
	case QueryFieldStale, QueryFieldDeferred, QueryFieldParent, QueryFieldChild, QueryFieldLeaf,
		QueryFieldBlocked, QueryFieldBlocking:
		if qualified {
			return "", "", "", fmt.Errorf("derived query token ?%s takes no value", name)
		}
		return field, QueryOperatorPresent, "", nil
	case QueryFieldCompleted:
		if qualified {
			return field, QueryOperatorAtLeast, value, nil
		}
		return field, QueryOperatorPresent, "", nil
	default:
		return "", "", "", fmt.Errorf("unknown derived query token ?%s", body)
	}
}

func splitPopulationQueryToken(body string) (QueryField, QueryOperator, string, error) {
	name, value, qualified := strings.Cut(body, ":")
	field := QueryField(name)
	if field != QueryFieldParked && field != QueryFieldArchived {
		return "", "", "", fmt.Errorf("unknown population query token +%s", body)
	}
	if !qualified {
		return field, QueryOperatorIs, QueryPopulationInclude, nil
	}
	return field, QueryOperatorIs, value, nil
}
