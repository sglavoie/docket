package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTutorColorRulesIncludeAllWorkflowStates(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		stateStr := state.String()
		found := false
		for _, rule := range tutorColorRules {
			if rule.re.MatchString(stateStr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tutor color rules missing highlighting for workflow state %q", stateStr)
		}
	}
}

func TestColorizeTutorLineHighlightsPaused(t *testing.T) {
	input := "Tasks in paused stay out of the Focus Queue."
	output := colorizeTutorLine(input)
	if input == output {
		t.Error("expected colorizeTutorLine to apply styling to 'paused', but output was unchanged")
	}
}

func TestTutorColorRulesDoNotTreatRetiredStageKeysAsBindings(t *testing.T) {
	input := ".1 Set first stage"
	if output := colorizeTutorLine(input); output != input {
		t.Fatalf("retired stage binding was colorized: %q", output)
	}
}

// TestRenderTutorContentEscapesLeadingHash verifies that a prose line escaped
// with a leading backslash renders a literal "#" rather than being consumed as
// a heading. The "Your first task" topic carries the only such line.
func TestRenderTutorContentEscapesLeadingHash(t *testing.T) {
	var topic TutorTopic
	for _, tp := range tutorGettingStartedTopics() {
		if tp.Title == "Your first task" {
			topic = tp
			break
		}
	}
	if topic.Title == "" {
		t.Fatal("could not find 'Your first task' topic")
	}

	out := ansi.Strip(renderTutorContent(topic, 80, ""))
	if !strings.Contains(out, "# is for tags") {
		t.Errorf("expected literal '# is for tags' in rendered output, got:\n%s", out)
	}
}

// TestRenderTutorContentPreservesAlignedTable verifies that an indented line
// containing an internal column run keeps its leading indent and internal
// spacing verbatim (no reflow, no indent flattening).
func TestRenderTutorContentPreservesAlignedTable(t *testing.T) {
	topic := TutorTopic{
		Title:    "Aligned",
		Category: "Test",
		Content:  "# Head\n\n  @date       Due date column\n",
	}

	out := ansi.Strip(renderTutorContent(topic, 80, ""))
	if !strings.Contains(out, "  @date       Due date column") {
		t.Errorf("expected aligned row preserved verbatim, got:\n%s", out)
	}
}

// TestRenderTutorContentReflowsIndentedProse verifies that indented prose is
// reflowed to fit a narrow pane while retaining its indent.
func TestRenderTutorContentReflowsIndentedProse(t *testing.T) {
	const width = 20
	topic := TutorTopic{
		Title:    "Reflow",
		Category: "Test",
		Content:  "  alpha beta gamma delta epsilon zeta eta theta iota\n",
	}

	out := ansi.Strip(renderTutorContent(topic, width, ""))

	indentedLines := 0
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		indentedLines++
		if len(line) > width {
			t.Errorf("indented prose line exceeds width %d: %q (len %d)", width, line, len(line))
		}
	}
	if indentedLines < 2 {
		t.Errorf("expected indented prose to reflow into multiple indented lines, got %d", indentedLines)
	}
}

// renderedTutorBody renders a single-content-line topic and returns the ANSI-
// stripped, non-blank body lines, with the title and category header lines
// dropped. It lets escape/heading assertions compare the exact rendered text a
// content line produces instead of relying on substring presence.
func renderedTutorBody(t *testing.T, content string, width int) []string {
	t.Helper()
	topic := TutorTopic{Title: "T", Category: "C", Content: content}
	out := ansi.Strip(renderTutorContent(topic, width, ""))
	lines := strings.Split(out, "\n")
	var body []string
	for i, ln := range lines {
		if i < 2 { // line 0 = title, line 1 = category
			continue
		}
		if strings.TrimSpace(ln) == "" {
			continue
		}
		body = append(body, ln)
	}
	return body
}

// TestRenderTutorContentStripsExactlyOneBackslash pins the escape path to exact
// rendered output so zero, one, and two leading backslashes are mutually
// distinguishable — a substring check would let a two-backslash input pass while
// masquerading as one. The renderer must strip exactly one leading backslash.
func TestRenderTutorContentStripsExactlyOneBackslash(t *testing.T) {
	// One backslash: escaped column-0 markup renders literally as prose.
	one := renderedTutorBody(t, "\\# literal\n", 80)
	if len(one) != 1 || one[0] != "# literal" {
		t.Fatalf("one backslash: want exactly [\"# literal\"], got %q", one)
	}

	// Two backslashes: exactly one is stripped, leaving a literal backslash.
	two := renderedTutorBody(t, "\\\\# literal\n", 80)
	if len(two) != 1 || two[0] != "\\# literal" {
		t.Fatalf("two backslashes: want exactly [\"\\\\# literal\"], got %q", two)
	}

	// Zero backslashes: this is a heading, so "# " is consumed and no backslash
	// appears. Distinct from both escape outputs above.
	zero := renderedTutorBody(t, "# literal\n", 80)
	if len(zero) != 1 || zero[0] != "literal" {
		t.Fatalf("zero backslashes (heading): want exactly [\"literal\"], got %q", zero)
	}

	// The three outputs must all differ, proving leading-backslash count is
	// observable end to end.
	if one[0] == two[0] || one[0] == zero[0] || two[0] == zero[0] {
		t.Errorf("escape outputs are not mutually distinguishable: zero=%q one=%q two=%q",
			zero[0], one[0], two[0])
	}
}

// TestRenderTutorContentHeadingStyling asserts that a column-0 "# " line is an
// observable heading (rendered with SectionHeaderStyle) while an escaped "\# "
// line is prose and is never promoted to a heading. It checks the raw styled
// output, not just visible text, so a styling regression is caught.
func TestRenderTutorContentHeadingStyling(t *testing.T) {
	headingMarker := SectionHeaderStyle.Render("Head")

	// A real heading carries the heading style verbatim.
	raw := renderTutorContent(TutorTopic{Title: "T", Category: "C", Content: "# Head\n"}, 80, "")
	if !strings.Contains(raw, headingMarker) {
		t.Errorf("expected heading styled with SectionHeaderStyle %q, got raw:\n%q", headingMarker, raw)
	}

	// An escaped line must render as prose: the heading style must be absent for
	// both the bare word and the literal "# Head", and the literal markup must
	// survive in the visible text.
	rawEsc := renderTutorContent(TutorTopic{Title: "T", Category: "C", Content: "\\# Head\n"}, 80, "")
	if strings.Contains(rawEsc, headingMarker) || strings.Contains(rawEsc, SectionHeaderStyle.Render("# Head")) {
		t.Errorf("escaped line was promoted to a styled heading:\n%q", rawEsc)
	}
	body := renderedTutorBody(t, "\\# Head\n", 80)
	if len(body) != 1 || body[0] != "# Head" {
		t.Fatalf("escaped prose: want exactly [\"# Head\"], got %q", body)
	}
}

// tutorMinPaneWidth mirrors the production floor computed in
// (*TutorModel).renderTopicContent: max(contentWidth()-2, 20). It is the
// narrowest width renderTutorContent is ever invoked with, and therefore the
// binding case for prose reflow: any prose line that fits here fits every
// wider pane.
const tutorMinPaneWidth = 20

// TestRenderTutorContentCorpusGuardUsesRenderedOutput renders every tutor topic
// at the minimum production pane width and locks in the renderer contract by
// inspecting the ANSI-stripped output of renderTutorContent itself — not a
// re-derived classifier or a direct wrapTutorText call. That means a wrapping,
// indentation, or aligned-row regression that widens an emitted line is caught
// here even if the source-side heuristics still agree.
//
//   - Every emitted line must fit the pane, except the (unwrapped) title,
//     category, and column-0 headings, which are single-line by design.
//   - Every column-0 "# " heading survives as its own rendered line (no
//     unintended demotion; escaped column-0 markup is not promoted).
func TestRenderTutorContentCorpusGuardUsesRenderedOutput(t *testing.T) {
	const width = tutorMinPaneWidth
	for _, topic := range tutorTopics() {
		topic := topic
		t.Run(topic.Category+"/"+topic.Title, func(t *testing.T) {
			// The title and category are emitted unwrapped; headings are rendered
			// as single lines and may legitimately exceed a narrow pane. Collect
			// them so an over-width line that is none of these is a real defect.
			exempt := map[string]bool{topic.Title: true, topic.Category: true}
			var headings []string
			for _, line := range strings.Split(topic.Content, "\n") {
				if strings.HasPrefix(line, "  ") {
					continue // aligned / indented-prose branch, never a heading
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "# ") {
					h := trimmed[2:]
					exempt[h] = true
					headings = append(headings, h)
				}
			}

			rendered := ansi.Strip(renderTutorContent(topic, width, ""))
			renderedLines := strings.Split(rendered, "\n")

			// (1) No reflowed prose / escaped / indented / aligned line may exceed
			// the pane. Measure display columns (lipgloss.Width) so multibyte
			// separators are not mistaken for overflow — this matches how the
			// production viewport clips. Only exempt header and heading lines may
			// exceed the pane.
			for _, ln := range renderedLines {
				if w := lipgloss.Width(ln); w > width && !exempt[ln] {
					t.Errorf("rendered line exceeds pane width %d and is not a title/category/heading: %q (%d cols)",
						width, ln, w)
				}
			}

			// (2) Every column-0 heading survives as an exact standalone line.
			for _, h := range headings {
				found := false
				for _, ln := range renderedLines {
					if ln == h {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("heading %q missing as a standalone rendered line:\n%s", h, rendered)
				}
			}
		})
	}
}

// TestRenderTutorContentSampledAlignedRows verifies that a representative
// aligned row from each table-heavy topic keeps its leading indent and internal
// column run verbatim after rendering (no reflow, no indent flattening).
func TestRenderTutorContentSampledAlignedRows(t *testing.T) {
	samples := []struct {
		title string
		row   string
	}{
		{"Quick Capture Grammar", "    @date       Due date: @tomorrow @friday @+3d"},
		{"Quick Capture Grammar", "    ^type       Task type: ^epic ^story ^task ^bug ^idea"},
		{"Bulk Operations", "  ,s  Start all selected"},
		{"Pipeline Views", "  Plan:     ;q next, ;D due dates, ;d daily review, ;R roadmap"},
		{"The Task Pipeline", "  Capture   Get ideas and tasks in quickly"},
		{"Attachments", "  tt attach 5 src/main.go:42      # Code reference (opens in IDE)"},
	}

	byTitle := map[string]TutorTopic{}
	for _, topic := range tutorTopics() {
		byTitle[topic.Title] = topic
	}

	for _, s := range samples {
		topic, ok := byTitle[s.title]
		if !ok {
			t.Errorf("topic %q not found", s.title)
			continue
		}
		if !strings.Contains(topic.Content, s.row) {
			t.Errorf("sample row drifted from %q source; update the test: %q", s.title, s.row)
			continue
		}
		out := ansi.Strip(renderTutorContent(topic, 80, ""))
		if !strings.Contains(out, s.row) {
			t.Errorf("aligned row not preserved verbatim in %q:\nwant: %q\ngot:\n%s", s.title, s.row, out)
		}
	}
}
