package external

import (
	"fmt"
	"strings"
	"unicode"
)

// CLISection represents a parsed section of the CLI reference.
type CLISection struct {
	Heading  string
	Body     string
	Keywords []string
}

// ParseCLIReference splits the CLI reference markdown into per-command sections.
func ParseCLIReference(raw string) []CLISection {
	lines := strings.Split(raw, "\n")
	var sections []CLISection
	var current *CLISection

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.Body = strings.TrimSpace(current.Body)
				current.Keywords = extractKeywords(*current)
				sections = append(sections, *current)
			}
			current = &CLISection{Heading: strings.TrimPrefix(line, "## ")}
		} else if current != nil {
			current.Body += line + "\n"
		}
	}
	if current != nil {
		current.Body = strings.TrimSpace(current.Body)
		current.Keywords = extractKeywords(*current)
		sections = append(sections, *current)
	}

	return sections
}

// extractKeywords builds a keyword list from heading, first line, and flag names.
func extractKeywords(s CLISection) []string {
	seen := make(map[string]bool)
	var keywords []string

	add := func(word string) {
		w := strings.ToLower(word)
		if len(w) > 1 && !seen[w] {
			seen[w] = true
			keywords = append(keywords, w)
		}
	}

	// Heading words (e.g. "tt task add" -> "tt", "task", "add")
	for _, w := range tokenize(s.Heading) {
		add(w)
	}

	// First non-empty line of body (the short description)
	for line := range strings.SplitSeq(s.Body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			for _, w := range tokenize(line) {
				add(w)
			}
			break
		}
	}

	// Flag names from lines like "  --tags stringSlice" or "  --parent, -p string"
	for line := range strings.SplitSeq(s.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "--"); ok {
			// Extract the flag name
			name := after
			// Take up to first space or comma
			for i, ch := range name {
				if ch == ' ' || ch == ',' {
					name = name[:i]
					break
				}
			}
			add(name)
		}
	}

	return keywords
}

// tokenize splits text into lowercase words, stripping punctuation.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// SelectRelevantSections scores sections against query tokens and returns matching ones.
func SelectRelevantSections(sections []CLISection, query string) string {
	queryTokens := tokenize(query)
	queryTokens = discardShortQueryTokens(queryTokens)
	if len(queryTokens) == 0 {
		return buildIndex(sections)
	}

	type scored struct {
		section CLISection
		score   int
	}

	var matches []scored
	for _, s := range sections {
		score := 0
		for _, qt := range queryTokens {
			for _, kw := range s.Keywords {
				if kw == qt || strings.Contains(kw, qt) || strings.Contains(qt, kw) {
					score++
					break
				}
			}
		}
		if score >= 1 {
			matches = append(matches, scored{s, score})
		}
	}

	// Sort by score descending (simple insertion sort, small N)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].score > matches[j-1].score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	// Cap at 10
	if len(matches) > 10 {
		matches = matches[:10]
	}

	var b strings.Builder
	b.WriteString(buildIndex(sections))
	b.WriteString("\n\n")

	for i, m := range matches {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(m.section.Heading)
		b.WriteString("\n\n")
		b.WriteString(m.section.Body)
	}

	return b.String()
}

func discardShortQueryTokens(tokens []string) []string {
	var out []string
	for _, token := range tokens {
		if len(token) <= 1 {
			continue
		}
		out = append(out, token)
	}
	return out
}

// buildIndex creates a compact one-line-per-command index.
func buildIndex(sections []CLISection) string {
	var b strings.Builder
	b.WriteString("# Command Index\n")
	for _, s := range sections {
		desc := ""
		for line := range strings.SplitSeq(s.Body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				desc = line
				break
			}
		}
		fmt.Fprintf(&b, "- %s — %s\n", s.Heading, desc)
	}
	return strings.TrimRight(b.String(), "\n")
}

// BuildCLIContext is a convenience wrapper that parses the reference, selects
// relevant sections for the query, and returns the final string for the system prompt.
func BuildCLIContext(cliReference, query string) string {
	sections := ParseCLIReference(cliReference)
	if len(sections) == 0 {
		return ""
	}
	return "# tt CLI Reference\n\n" + SelectRelevantSections(sections, query)
}
