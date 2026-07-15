package tui

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// tutorColorRule defines a regex-based colorization rule for tutor content.
type tutorColorRule struct {
	re    *regexp.Regexp
	style lipgloss.Style
	group int // 0 = whole match, >0 = submatch group index to style
}

var tutorColorRules []tutorColorRule

func init() {
	type rule struct {
		pattern string
		style   lipgloss.Style
		group   int
	}

	primaryStyle := makeColorStyle(ColorPrimary)
	keybindStyle := makeColorStyle(ColorWarning)

	rules := make([]rule, 0, len(models.AllWorkflowStates())+32)
	for _, state := range models.AllWorkflowStates() {
		rules = append(rules, rule{
			pattern: `\b` + regexp.QuoteMeta(state.String()) + `\b`,
			style:   GetStateStyle(state),
		})
	}

	rules = append(rules, []rule{

		// Task types (singular and plural)
		{`\bEpics?\b`, EpicStyle, 0},
		{`\bStories\b`, StoryStyle, 0},
		{`\bStory\b`, StoryStyle, 0},
		{`\bTasks?\b`, TaskStyle, 0},
		{`\bBugs?\b`, BugStyle, 0},
		{`\bIdeas?\b`, IdeaStyle, 0},

		// "tt" as standalone word
		{`\btt\b`, primaryStyle.Bold(true), 0},

		// Priority patterns: "Priority 1", "!1" through "!5"
		{`Priority 1|!1`, TitlePriority1Style, 0},
		{`Priority 2|!2`, TitlePriority2Style, 0},
		{`Priority 3|!3`, TitlePriority3Style, 0},
		{`Priority 4|!4`, TitlePriority4Style, 0},
		{`Priority 5|!5`, TitlePriority5Style, 0},

		// Section names (colors derived from ViewMode constants in styles.go)
		{`\bActive\b`, makeColorStyle(ColorViewActive), 0},
		{`\bNext\b`, makeColorStyle(ColorViewNext), 0},
		{`\bInbox\b`, makeColorStyle(ColorSecondary), 0},
		{`\bWaiting\b`, keybindStyle, 0},
		{`\bStandby\b`, makeColorStyle(ColorViewStandby), 0},
		{`\bSomeday\b`, makeColorStyle(ColorViewSomeday), 0},
		{`\bReview\b`, ReviewStyle, 0},

		// Tags: #word
		{`#[a-zA-Z][a-zA-Z0-9_-]*`, primaryStyle, 0},

		// Contexts
		{`"work"`, keybindStyle, 0},
		{`"personal"`, keybindStyle, 0},
		{`"side-projects"`, keybindStyle, 0},

		// Keybinding highlights (yellow)
		{`Ctrl\+K`, keybindStyle, 0},
		{`\bEnter\b`, keybindStyle, 0},
		{`\bEsc\b`, keybindStyle, 0},
		{`\bTab\b`, keybindStyle, 0},
		{`\bj/k\b`, keybindStyle, 0},
		{`\bJ/K\b`, keybindStyle, 0},
		// Comma-prefix bindings at start of line: ,s ,d ,b ,o ,x ,S
		{`^(,[sdboxrSm])\s`, keybindStyle, 1},
		// "using , followed" — color the comma
		{`using (,) followed`, keybindStyle, 1},
		// "Press X" patterns for single-char keys
		{`[Pp]ress ([qQ?:@fnva]) `, keybindStyle, 1},
		// "using the s prefix"
		{`the (s) prefix`, keybindStyle, 1},
		// "Use !" pattern
		{`Use (!) followed`, keybindStyle, 1},
		// "d for days, w for weeks"
		{`(d) for days`, keybindStyle, 1},
		{`(w) for weeks`, keybindStyle, 1},
		// "set with !", "with #", "with @"
		{`(?:set |tags )with ([!#@])`, keybindStyle, 1},
		{`assignment with (@)`, keybindStyle, 1},
		// Command palette commands
		{`type (ask ai) to`, keybindStyle, 1},
		// Score badge patterns
		{`\[S:\d+\]`, makeColorStyle(ColorSuccess), 0},
		{`\[S:~\d+\]`, makeColorStyle(ColorOrange), 0},
		// Shift+S keybinding
		{`Shift\+S`, keybindStyle, 0},
		// FS prefix
		{`\bFS\b`, keybindStyle, 0},
		// Fv prefix and sub-keys (Fv, Fvs, Fvl, Fvd)
		{`\bFv[sld]?\b`, keybindStyle, 0},
		// :sv palette prefix
		{`:sv\b`, keybindStyle, 0},
		// "by s", "then l", "then d" — color the single key after by/then
		{`(?:by|then) ([sld]) `, keybindStyle, 1},
		// Repeat-last-action key
		{`Press (\.) `, keybindStyle, 1},
	}...)

	for _, r := range rules {
		tutorColorRules = append(tutorColorRules, tutorColorRule{
			re:    regexp.MustCompile(r.pattern),
			style: r.style,
			group: r.group,
		})
	}
}

// colorizeTutorLine applies keyword colorization to a single line of tutor content.
func colorizeTutorLine(line string) string {
	type replacement struct {
		start, end int
		styled     string
	}

	var replacements []replacement
	occupied := make([]bool, len(line))

	for _, rule := range tutorColorRules {
		if rule.group == 0 {
			matches := rule.re.FindAllStringIndex(line, -1)
			for _, m := range matches {
				overlap := false
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) && occupied[i] {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) {
						occupied[i] = true
					}
				}
				replacements = append(replacements, replacement{
					start:  m[0],
					end:    m[1],
					styled: rule.style.Render(line[m[0]:m[1]]),
				})
			}
		} else {
			// Group-based: only style the specified submatch group
			allSubmatch := rule.re.FindAllStringSubmatchIndex(line, -1)
			for _, m := range allSubmatch {
				gi := rule.group * 2
				if gi+1 >= len(m) || m[gi] < 0 {
					continue
				}
				start, end := m[gi], m[gi+1]
				overlap := false
				for i := start; i < end; i++ {
					if i < len(occupied) && occupied[i] {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				// Mark the full match as occupied to prevent other rules matching context
				for i := m[0]; i < m[1]; i++ {
					if i < len(occupied) {
						occupied[i] = true
					}
				}
				replacements = append(replacements, replacement{
					start:  start,
					end:    end,
					styled: rule.style.Render(line[start:end]),
				})
			}
		}
	}

	if len(replacements) == 0 {
		return line
	}

	// Sort by start position (already mostly sorted since we iterate rules in order,
	// but matches within a rule are sorted)
	for i := 1; i < len(replacements); i++ {
		for j := i; j > 0 && replacements[j].start < replacements[j-1].start; j-- {
			replacements[j], replacements[j-1] = replacements[j-1], replacements[j]
		}
	}

	var result strings.Builder
	pos := 0
	for _, r := range replacements {
		result.WriteString(line[pos:r.start])
		result.WriteString(r.styled)
		pos = r.end
	}
	result.WriteString(line[pos:])
	return result.String()
}

func renderTutorContent(topic TutorTopic, width int, searchQuery string) string {
	titleStyle := TitleStyle
	headingStyle := SectionHeaderStyle
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	proseStyle := NormalStyle

	hl := func(s string) string {
		if searchQuery != "" {
			return HighlightStyledText(s, searchQuery)
		}
		return s
	}

	var b strings.Builder
	b.WriteString(hl(titleStyle.Render(colorizeTutorLine(topic.Title))))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(topic.Category))
	b.WriteString("\n\n")

	for line := range strings.SplitSeq(topic.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString(hl(headingStyle.Render(colorizeTutorLine(trimmed[2:]))))
		case strings.HasPrefix(line, "  ") && (strings.HasPrefix(trimmed, "tt ") || strings.HasPrefix(trimmed, "$ ")):
			b.WriteString("  " + codeStyle.Render(colorizeTutorLine(trimmed)))
		case trimmed == "":
			// blank line
		default:
			wrapped := wrapTutorText(trimmed, width)
			colorized := make([]string, 0)
			for wl := range strings.SplitSeq(wrapped, "\n") {
				colorized = append(colorized, colorizeTutorLine(wl))
			}
			b.WriteString(hl(proseStyle.Render(strings.Join(colorized, "\n"))))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// wrapTutorText wraps text to the given width.
func wrapTutorText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}

	var lines []string
	remaining := text
	for len(remaining) > width {
		bp := width
		for i := width; i > 0; i-- {
			if remaining[i-1] == ' ' {
				bp = i - 1
				break
			}
		}
		lines = append(lines, remaining[:bp])
		remaining = strings.TrimLeft(remaining[bp:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return strings.Join(lines, "\n")
}
