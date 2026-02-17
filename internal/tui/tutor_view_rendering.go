package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	rules := []rule{
		// Statuses (longest first to match in_progress before partial)
		{`\bin_progress\b`, InProgressStyle, 0},
		{`\bcanceled\b`, GetStatusStyle("canceled"), 0},
		{`\bblocked\b`, BlockedStyle, 0},
		{`\breview\b`, ReviewStyle, 0},
		{`\btodo\b`, TodoStyle, 0},
		{`\bdone\b`, DoneStyle, 0},

		// Task types (singular and plural)
		{`\bEpics?\b`, EpicStyle, 0},
		{`\bStories\b`, StoryStyle, 0},
		{`\bStory\b`, StoryStyle, 0},
		{`\bTasks?\b`, TaskStyle, 0},
		{`\bBugs?\b`, BugStyle, 0},
		{`\bIdeas?\b`, IdeaStyle, 0},

		// "tt" as standalone word
		{`\btt\b`, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true), 0},

		// Priority patterns: "Priority 1", "!1" through "!5"
		{`Priority 1|!1`, Priority1Style, 0},
		{`Priority 2|!2`, Priority2Style, 0},
		{`Priority 3|!3`, Priority3Style, 0},
		{`Priority 4|!4`, Priority4Style, 0},
		{`Priority 5|!5`, Priority5Style, 0},

		// Section names
		{`\bActive\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("34")), 0},
		{`\bNext\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("61")), 0},
		{`\bInbox\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("33")), 0},
		{`\bWaiting\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("214")), 0},
		{`\bSomeday\b`, lipgloss.NewStyle().Foreground(lipgloss.Color("135")), 0},
		{`\bReview\b`, ReviewStyle, 0},

		// Tags: #word
		{`#[a-zA-Z][a-zA-Z0-9_-]*`, lipgloss.NewStyle().Foreground(ColorPrimary), 0},

		// Contexts
		{`"work"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`"personal"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`"side-projects"`, lipgloss.NewStyle().Foreground(ColorWarning), 0},

		// Keybinding highlights (yellow)
		{`Ctrl\+K`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bEnter\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bEsc\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bTab\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		{`\bj/k\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// Comma-prefix bindings at start of line: ,s ,d ,b ,o ,x
		{`^(,[sdboxr])\s`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "using , followed" — color the comma
		{`using (,) followed`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "Press X" patterns for single-char keys
		{`Press ([qQ?:@f]) `, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "using the s prefix"
		{`the (s) prefix`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "Use !" pattern
		{`Use (!) followed`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "d for days, w for weeks"
		{`(d) for days`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		{`(w) for weeks`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// "set with !", "with #", "with @"
		{`(?:set |tags )with ([!#@])`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		{`assignment with (@)`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// Command palette commands
		{`type (ask ai) to`, lipgloss.NewStyle().Foreground(ColorWarning), 1},
		// Score badge patterns
		{`\[S:\d+\]`, lipgloss.NewStyle().Foreground(lipgloss.Color("42")), 0},
		{`\[S:~\d+\]`, lipgloss.NewStyle().Foreground(lipgloss.Color("208")), 0},
		// Shift+S keybinding
		{`Shift\+S`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// FS prefix
		{`\bFS\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// Fv prefix and sub-keys (Fv, Fvs, Fvl, Fvd)
		{`\bFv[sld]?\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// :sv palette prefix
		{`:sv\b`, lipgloss.NewStyle().Foreground(ColorWarning), 0},
		// "by s", "then l", "then d" — color the single key after by/then
		{`(?:by|then) ([sld]) `, lipgloss.NewStyle().Foreground(ColorWarning), 1},
	}

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

func renderTutorContent(topic TutorTopic, width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	headingStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	proseStyle := lipgloss.NewStyle()

	var b strings.Builder
	b.WriteString(titleStyle.Render(colorizeTutorLine(topic.Title)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(topic.Category))
	b.WriteString("\n\n")

	for _, line := range strings.Split(topic.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString(headingStyle.Render(colorizeTutorLine(trimmed[2:])))
		case strings.HasPrefix(line, "  ") && (strings.HasPrefix(trimmed, "tt ") || strings.HasPrefix(trimmed, "$ ")):
			b.WriteString("  " + codeStyle.Render(colorizeTutorLine(trimmed)))
		case trimmed == "":
			// blank line
		default:
			wrapped := wrapTutorText(trimmed, width)
			colorized := make([]string, 0)
			for _, wl := range strings.Split(wrapped, "\n") {
				colorized = append(colorized, colorizeTutorLine(wl))
			}
			b.WriteString(proseStyle.Render(strings.Join(colorized, "\n")))
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
