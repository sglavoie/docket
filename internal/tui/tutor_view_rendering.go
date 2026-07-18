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

	// renderProse wraps text to width, colorizes each wrapped line, and styles
	// the block as prose. Used by the escape and default branches.
	renderProse := func(text string) string {
		wrapped := wrapTutorText(text, width)
		colorized := make([]string, 0)
		for wl := range strings.SplitSeq(wrapped, "\n") {
			colorized = append(colorized, colorizeTutorLine(wl))
		}
		return hl(proseStyle.Render(strings.Join(colorized, "\n")))
	}

	// renderIndentedProse reflows body to the space remaining after the indent,
	// then re-applies the indent to every wrapped line so the indent stays
	// visible without overflowing narrow panes.
	renderIndentedProse := func(indent, body string) string {
		wrapped := wrapTutorText(body, width-len(indent))
		colorized := make([]string, 0)
		for wl := range strings.SplitSeq(wrapped, "\n") {
			colorized = append(colorized, indent+colorizeTutorLine(wl))
		}
		return hl(proseStyle.Render(strings.Join(colorized, "\n")))
	}

	var b strings.Builder
	b.WriteString(hl(titleStyle.Render(colorizeTutorLine(topic.Title))))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(topic.Category))
	b.WriteString("\n\n")

	for line := range strings.SplitSeq(topic.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		body := line[len(indent):]
		isCommand := strings.HasPrefix(trimmed, "tt ") || strings.HasPrefix(trimmed, "$ ")
		switch {
		case trimmed == "":
			// blank line
		case strings.HasPrefix(line, "  ") && (isCommand || strings.Contains(trimmed, "  ")):
			// Aligned / preformatted: keep the leading whitespace and internal
			// column alignment verbatim when the row fits the pane. Rows wider
			// than the pane wrap with a hanging indent at the description column
			// so no content is clipped by the viewport while alignment survives.
			segs := splitAlignedRow(indent, body, width)
			parts := make([]string, len(segs))
			for i, seg := range segs {
				if isCommand {
					parts[i] = seg.lead + codeStyle.Render(colorizeTutorLine(seg.content))
				} else {
					parts[i] = hl(seg.lead + proseStyle.Render(colorizeTutorLine(seg.content)))
				}
			}
			b.WriteString(strings.Join(parts, "\n"))
		case strings.HasPrefix(line, "  "):
			// Indented prose: reflow to the pane but keep the indent.
			b.WriteString(renderIndentedProse(indent, body))
		case strings.HasPrefix(trimmed, "\\"):
			// Escape: strip exactly one leading backslash, render the remainder
			// as prose. Sits above the heading case so escaped column-0 markup
			// (e.g. "\# ...") renders literally instead of as a heading.
			b.WriteString(renderProse(trimmed[1:]))
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString(hl(headingStyle.Render(colorizeTutorLine(trimmed[2:]))))
		default:
			b.WriteString(renderProse(trimmed))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// alignedSegment is one physical line of a (possibly wrapped) aligned row:
// lead is the verbatim leading whitespace and content is the styleable text.
type alignedSegment struct {
	lead    string
	content string
}

// minAlignedDesc is the smallest description column width worth preserving when
// wrapping a wide aligned row. If honoring the description column would leave
// fewer than this many columns, the row falls back to wrapping at the base
// indent so the text stays readable instead of dribbling down a narrow gutter.
const minAlignedDesc = 16

// splitAlignedRow returns the physical lines for one aligned row. Rows that fit
// the pane are returned verbatim (single segment) so narrow tables keep their
// exact column alignment. Wider rows wrap at word boundaries: the label stays on
// the first line and the description reflows under a hanging indent aligned to
// the description column, so nothing is clipped by the viewport.
func splitAlignedRow(indent, body string, width int) []alignedSegment {
	if width <= 0 || lipgloss.Width(indent+body) <= width {
		return []alignedSegment{{lead: indent, content: body}}
	}

	baseAvail := max(width-len(indent), 1)

	// Locate the description column: the first run of 2+ spaces separating a
	// label from its description.
	hangCol := -1
	var label, desc string
	if gap := strings.Index(body, "  "); gap > 0 {
		gapEnd := gap
		for gapEnd < len(body) && body[gapEnd] == ' ' {
			gapEnd++
		}
		if gapEnd < len(body) { // there is text after the gap
			if hc := len(indent) + gapEnd; width-hc >= minAlignedDesc {
				hangCol = hc
				label = body[:gapEnd] // label plus its trailing gap
				desc = body[gapEnd:]
			}
		}
	}

	if hangCol < 0 {
		// No usable description column: wrap the whole body at the base indent.
		var segs []alignedSegment
		for wl := range strings.SplitSeq(wrapTutorText(body, baseAvail), "\n") {
			segs = append(segs, alignedSegment{lead: indent, content: wl})
		}
		return segs
	}

	descLines := strings.Split(wrapTutorText(desc, width-hangCol), "\n")
	segs := make([]alignedSegment, 0, len(descLines))
	segs = append(segs, alignedSegment{lead: indent, content: label + descLines[0]})
	hang := strings.Repeat(" ", hangCol)
	for _, dl := range descLines[1:] {
		segs = append(segs, alignedSegment{lead: hang, content: dl})
	}
	return segs
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
