package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// screenshotLikeDescription reproduces the reported-bug content shape: reviewer
// prose, a malformed/ragged Markdown table mixing long financial "raw ingredient"
// identifiers with a "spoken output" column, quoted/comma-laden and backticked
// cells, a degenerate row, and follow-up JSON plus trailing prose. This is the
// class of description that overflowed the task-detail screen before the shared
// width contract (sessions 1 & 2) bounded renderer and layout output to width.
const screenshotLikeDescription = `Kris left feedback on the pricing summary: the spoken output must match the raw
ingredients table exactly, and the follow-up JSON has to round-trip without
dropping the notional.

| Raw ingredients                                                     | Spoken output |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| ` + "`INTERESTRATE:IRSWAP:OIS` + `USD SOFR OIS COMPOUND` + `10Y`" + `         | "ten-year SOFR OIS" |
| ` + "`INTERESTRATE:IRSWAP:FIXEDFLOAT` + `EUR EURIBOR` + `5Y`" + `             | "five-year Euribor fixed-float swap" |
| ` + "`INTERESTRATE:CROSSCURRENCY:BASIS` + `EUR USD` + `10Y`" + `              | "ten-year euro-dollar cross-currency basis swap" |
| ` + "`FOREIGNEXCHANGE:NDF` + `BRL USD` + `1M`" + `                            | "one-month real-dollar NDF" |
| ` + "`CREDIT:INDEX:CDX:CDXIG` + `CDX.NA.IG S45`" + `                          | "CDX North America IG series forty-five"
| "a quoted, comma-laden phrase with 'nested' quotes"                | 'nested spoken phrase' |
| code                                                               | ` + "`github.com/sglavoie/tt/internal/tui.renderMarkdown`" + ` |
| broken row without trailing pipe and extra | | pipes | | scattered
|||

Follow-up payload:

` + "```json" + `
{"ingredient": "1,250,000.00 USD", "spoken": "one point two five million", "confirmed": false}
` + "```" + `

Kris also asked that this trailing prose wrap cleanly on every viewport width.`

// TestTaskDetailNarrowMarkdownTableStaysWithinWidth is the integration regression
// for the reported bug: a task whose Description holds a wide Markdown table
// overflowed the task-detail screen on a narrow terminal. It drives the full
// TaskDetailModel.View() path (description → shared renderMarkdown → card → scroll
// accounting) and asserts every rendered line fits the model width. If the shared
// renderer ever stops bounding its output to width, this line-width check fires.
func TestTaskDetailNarrowMarkdownTableStaysWithinWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "Narrow", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	// Screenshot-like wide, multi-column table plus surrounding prose. On a narrow
	// terminal the cells must wrap vertically and stay bounded rather than overflow.
	description := strings.Join([]string{
		"Investigate the layout regression.",
		"",
		"| Column One Heading | Column Two Heading | Column Three Heading |",
		"| --- | --- | --- |",
		"| a really long cell value here | another long value in this cell | yet more overflowing content here |",
		"| short | short | short |",
		"",
		"Follow-up notes with a fairly long trailing sentence that also needs to wrap on narrow widths.",
	}, "\n")

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Fix narrow markdown overflow")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Description = description
	project.Tasks = []*models.Task{task}

	m := &TaskDetailModel{task: task, project: project}

	// Narrow width and a viewport the wrapped content overflows, so the manual
	// vertical scroll accounting (totalLines vs viewHeight) is exercised with real
	// wrapped rows rather than short-circuited by a viewport that fits everything.
	const width = 34
	m.SetSize(width, 20)

	rendered := m.View().Content
	for line := range strings.SplitSeq(rendered, "\n") {
		stripped := ansi.Strip(line)
		if w := lipgloss.Width(stripped); w > width {
			t.Fatalf("task detail line exceeds width %d: %q (width=%d)\n\nfull output:\n%s",
				width, stripped, w, ansi.Strip(rendered))
		}
	}

	// Scroll accounting must be driven by the explicit wrapped line count, not by
	// implicit terminal wrapping: a wide table on a narrow screen produces more
	// content lines than the viewport can show.
	if m.totalLines <= m.viewHeight() {
		t.Fatalf("expected wrapped table to overflow viewport: totalLines=%d viewHeight=%d",
			m.totalLines, m.viewHeight())
	}

	// Important cell text must survive vertical wrapping (not be truncated away).
	// Inspect the full pre-scroll detail body because the wrapped footer reduces
	// the first viewport's body rows at narrow widths.
	stripped := ansi.Strip(strings.Join(m.detailSections(m.detailContentWidth()), "\n"))
	for _, word := range []string{"really", "another", "content"} {
		if !strings.Contains(stripped, word) {
			t.Fatalf("expected wide-table cell word %q preserved in task detail, got:\n%s", word, stripped)
		}
	}
}

// TestTaskDetailStaysWithinWidthAcrossSizes exercises the shared task-detail
// content-width path (View → sections → cards → dividers → centering) at narrow,
// normal, and centered-wide model widths. Every rendered line — card borders,
// dividers, description content, title, and footer — must stay within the model
// width so wide terminals center the real card width without drift and narrow
// terminals never overflow.
func TestTaskDetailStaysWithinWidthAcrossSizes(t *testing.T) {
	description := strings.Join([]string{
		"Overview paragraph describing the task with enough words to wrap on narrow widths.",
		"",
		"| Column One Heading | Column Two Heading | Column Three Heading |",
		"| --- | --- | --- |",
		"| a really long cell value here | another long value in this cell | yet more overflowing content here |",
		"",
		"Trailing notes sentence.",
	}, "\n")

	sizes := []struct {
		name  string
		width int
	}{
		{"narrow", 34},
		{"normal", 80},
		{"centered-wide", 160},
	}

	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			project, err := models.NewProject("proj-1", "Sizes", t.TempDir())
			if err != nil {
				t.Fatalf("NewProject: %v", err)
			}

			task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Fix task detail width")
			if err != nil {
				t.Fatalf("NewTask: %v", err)
			}
			task.Description = description
			// Add comments and decisions so the History cluster divider renders and
			// its width is exercised alongside the section divider.
			task.AddComment("A comment that adds some history to the task detail.")
			task.AddDecision("A decision recorded for this task.", "Because it matters for the divider width path.", models.DecisionCategoryDesign)
			project.Tasks = []*models.Task{task}

			m := &TaskDetailModel{task: task, project: project}
			m.historyExpanded = true
			m.SetSize(tc.width, 40)

			rendered := m.View().Content
			for line := range strings.SplitSeq(rendered, "\n") {
				stripped := ansi.Strip(line)
				if w := lipgloss.Width(stripped); w > tc.width {
					t.Fatalf("task detail line exceeds width %d: %q (width=%d)\n\nfull output:\n%s",
						tc.width, stripped, w, ansi.Strip(rendered))
				}
			}
		})
	}
}

// TestTaskDetailDividersUseVisualWidth guards the divider width calculation: the
// section and history dividers must subtract the prefix's visual width (not its
// byte length), so multi-byte glyphs like "──" and "⏵" do not push the divider
// past the requested width.
func TestTaskDetailDividersUseVisualWidth(t *testing.T) {
	// Width is chosen wide enough that both divider prefixes (which contain
	// multi-byte glyphs like "──" and "⏵") fit, so the trailing rule fills the
	// remainder to exactly width. A byte-length calculation would under-fill.
	const width = 100

	section := renderDetailSectionDivider("Planning", width)
	if w := lipgloss.Width(ansi.Strip(section)); w != width {
		t.Fatalf("section divider width = %d, want %d: %q", w, width, ansi.Strip(section))
	}

	history := renderHistoryClusterDivider(3, 1, 2, false, width)
	if w := lipgloss.Width(ansi.Strip(history)); w != width {
		t.Fatalf("history divider width = %d, want %d: %q", w, width, ansi.Strip(history))
	}
}

// TestTaskDetailScreenshotRegressionAcrossWidths is the screenshot-level
// regression for the reported overflow bug. It drives the full
// TaskDetailModel.View() path with a description that mirrors the screenshots
// (reviewer prose, malformed financial table, spoken-output column, follow-up
// JSON) at 80/100/120 columns. At every width, no rendered line may exceed the
// model width; and at centered-wide widths (>MaxContentWidth) the card body —
// each line minus its centering pad — must stay no wider than MaxContentWidth,
// so wide terminals center the capped card instead of stretching it. Key text
// from every problem cell must survive wrapping rather than being truncated.
func TestTaskDetailScreenshotRegressionAcrossWidths(t *testing.T) {
	for _, width := range []int{80, 100, 120} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			project, err := models.NewProject("proj-1", "Screenshot", t.TempDir())
			if err != nil {
				t.Fatalf("NewProject: %v", err)
			}

			task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Fix pricing summary overflow")
			if err != nil {
				t.Fatalf("NewTask: %v", err)
			}
			task.Description = screenshotLikeDescription
			project.Tasks = []*models.Task{task}

			m := &TaskDetailModel{task: task, project: project}
			// Tall viewport so the whole description renders and every wrapped
			// line is width-checked (scroll accounting is exercised separately).
			m.SetSize(width, 200)

			rendered := m.View().Content

			// The full view (title bar, content, footer bars span the model width)
			// must never exceed the model width on any line.
			for _, line := range strings.Split(rendered, "\n") {
				stripped := ansi.Strip(line)
				if w := lipgloss.Width(stripped); w > width {
					t.Fatalf("task detail line exceeds width %d: %q (width=%d)\n\nfull output:\n%s",
						width, stripped, w, ansi.Strip(rendered))
				}
			}

			// The centered card body (the content region between the title and
			// footer bars) must stay no wider than MaxContentWidth. Render it
			// directly so full-width title/footer bars do not mask a stretched
			// card. Each content line minus its centering pad is the card body.
			contentWidth := min(width, MaxContentWidth)
			padLeft := 0
			if width > contentWidth {
				padLeft = (width - contentWidth) / 2
			}
			padStr := strings.Repeat(" ", padLeft)

			body := m.renderDetailContent(m.detailContentWidth())
			for _, line := range strings.Split(body, "\n") {
				stripped := strings.TrimPrefix(ansi.Strip(line), padStr)
				if w := lipgloss.Width(stripped); w > MaxContentWidth {
					t.Fatalf("card body exceeds MaxContentWidth %d at model width %d: %q (width=%d)",
						MaxContentWidth, width, stripped, w)
				}
			}

			// Important text from each problem cell (long identifiers, quoted and
			// spoken phrases, JSON payload, reviewer name) must survive wrapping —
			// it may be split across lines but must not be truncated away.
			flat := strings.ReplaceAll(ansi.Strip(rendered), "\n", " ")
			compact := strings.ReplaceAll(flat, " ", "")
			for _, word := range []string{"Kris", "SOFR", "Euribor", "quoted", "renderMar", "kdown", "confirmed"} {
				if !strings.Contains(flat, word) && !strings.Contains(compact, word) {
					t.Fatalf("expected screenshot word %q preserved at width %d, got:\n%s",
						word, width, ansi.Strip(rendered))
				}
			}
		})
	}
}

// TestTaskDetailScreenshotScrollAccounting asserts scroll accounting is driven
// by the explicit wrapped line count, not by implicit terminal wrapping: with
// the screenshot-like description at a constrained height, the model's computed
// totalLines must exceed the visible viewHeight so the manual scroll path (and
// the footer percentage) engages rather than relying on the terminal to wrap.
func TestTaskDetailScreenshotScrollAccounting(t *testing.T) {
	project, err := models.NewProject("proj-1", "Screenshot", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Fix pricing summary overflow")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Description = screenshotLikeDescription
	project.Tasks = []*models.Task{task}

	m := &TaskDetailModel{task: task, project: project}
	// Constrained height so the wrapped screenshot content overflows the viewport.
	m.SetSize(80, 20)

	_ = m.View().Content
	if m.totalLines <= m.viewHeight() {
		t.Fatalf("expected screenshot description to overflow viewport: totalLines=%d viewHeight=%d",
			m.totalLines, m.viewHeight())
	}
}

func TestTaskDetailFooterWrapsAtMinimumWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "Footer", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Footer wrapping")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	const (
		width  = 60
		height = 20
	)
	m := &TaskDetailModel{task: task, project: project}
	m.SetSize(width, height)

	rendered := m.View().Content
	plain, issues := normalizeScreen(rendered, width, height)
	if len(issues) > 0 {
		t.Fatalf("task detail footer layout issues: %+v\n\n%s", issues, plain)
	}

	for _, want := range []string{"c: comments", "D: decisions", "H: toggle history", "Esc: close"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wrapped footer missing %q:\n%s", want, plain)
		}
	}
}

// TestTaskDetailFooterRecordsRequiredTruncation verifies that TaskDetail's custom
// wrapped footer routes its last-resort clipping through the render audit: at an
// extremely narrow width a help token can't be wrapped small enough, so the footer
// clips it to fit and records a required_text_truncated issue.
func TestTaskDetailFooterRecordsRequiredTruncation(t *testing.T) {
	// Width narrower than the shortest indivisible help token ("H: toggle history"),
	// so wrapping cannot avoid a last-resort clip of the over-wide help line.
	m := &TaskDetailModel{}
	m.SetSize(8, 20)

	_, issues := withRenderAudit(func() string {
		return m.renderWrappingFooter("")
	})

	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected a %q audit issue for the clipped task detail footer, got %+v", requiredTextTruncatedKind, issues)
	}
}

func TestTaskDetailWrappedFooterCountsTowardScrollHeight(t *testing.T) {
	project, err := models.NewProject("proj-1", "Scrollable Footer", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Scrollable footer wrapping")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Description = screenshotLikeDescription
	project.Tasks = []*models.Task{task}

	const (
		width  = 60
		height = 20
	)
	m := &TaskDetailModel{task: task, project: project}
	m.SetSize(width, height)

	rendered := m.View().Content
	plain, issues := normalizeScreen(rendered, width, height)
	if len(issues) > 0 {
		t.Fatalf("task detail scrollable footer layout issues: %+v\n\n%s", issues, plain)
	}
	if m.totalLines <= m.viewHeight() {
		t.Fatalf("expected scrollable detail content: totalLines=%d viewHeight=%d", m.totalLines, m.viewHeight())
	}
	if !strings.Contains(plain, "% (1/") {
		t.Fatalf("expected scroll indicator in wrapped footer:\n%s", plain)
	}
	if !strings.Contains(plain, "Esc: close") {
		t.Fatalf("wrapped footer should preserve final close hint:\n%s", plain)
	}
}
