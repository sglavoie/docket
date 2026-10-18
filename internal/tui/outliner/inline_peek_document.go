package outliner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// inlinePeekPositionRows is the position row beneath the body. It is always
// drawn, for the same reason the body is always padded: a row whose counts
// appear and disappear would move the list under the cursor. It is the one fixed
// row in Peek's geometry — everything else about the window's size is decided by
// peekDockGeometry and reaches a document stamped onto it.
const inlinePeekPositionRows = 1

// inlinePeekDocument is the complete detail document behind Peek, separated
// from the page of it that is visible. Rendering and viewport measurement both
// consume it through Lines, so the height that is reserved and the height that
// is drawn are the same number by construction rather than by two agreeing
// calculations.
type inlinePeekDocument struct {
	// Body is every detail row the subject has, wrapped to the pane and in
	// content order, the title and the ancestor chain included. It is unbounded
	// and nothing in it sits outside the window: the window decides only how much
	// of it is on screen, so a detail Peek holds is always a scroll away rather
	// than sometimes pinned and sometimes elided.
	Body     []string
	sections []inlinePeekSection
	// Fingerprint identifies the content the body was built from, independent of
	// the width it was wrapped to. It is what lets a stored offset survive a
	// re-render without resolving its content anchor again. Changed detail or
	// geometry resolves that anchor against the new section layout.
	Fingerprint string

	indent string
	// width is the content width beside the indent, which is what every row in
	// the document was already clamped to.
	width int
	// window is the frame's dock geometry, stamped onto the document when it was
	// built. Content and window travel together from there, so Paged, MaxOffset,
	// ClampOffset and Lines cannot be answered against two different frames — and
	// a document built with no window draws nothing rather than falling back to a
	// height nobody measured.
	window peekDockGeometry
}

// Empty reports whether the document has nothing to draw at all. A row whose
// subject could not be resolved produces one, and it is what tells the four
// scrolling keys that no pane is on screen to consume them.
func (doc inlinePeekDocument) Empty() bool {
	return len(doc.Body) == 0
}

// Paged reports whether the body outgrows the window, which is what the
// scrolling keys and their hints read. A document that fits has the single
// offset zero and nothing to travel to.
func (doc inlinePeekDocument) Paged() bool {
	return len(doc.Body) > doc.window.BodyRows()
}

// MaxOffset is the last content row the window may start on. The window is
// always full rather than short at the end of the document, so the final offset
// is pinned to the body's end and an unpaged document has the single offset
// zero.
func (doc inlinePeekDocument) MaxOffset() int {
	return max(len(doc.Body)-doc.window.BodyRows(), 0)
}

// ClampOffset bounds a content-row offset to the document. Every reader goes
// through it, so an offset remembered against a longer body narrows to the end
// here rather than slicing past it.
func (doc inlinePeekDocument) ClampOffset(offset int) int {
	return min(max(offset, 0), doc.MaxOffset())
}

// Lines renders the window at one content-row offset: the content rows visible
// there, blank-padded when the document is shorter than the window, and the
// position row that says exactly how much sits off screen in each direction —
// left blank when nothing does, since a document that fits has nothing to count
// and the row is reserved for the height alone. It is the only way to either the drawn window or its height, so a render and a
// measurement cannot disagree about what is on screen. The slice is always the
// window's own height, which is what keeps the dock the same height over a
// one-line document and a fifty-line one.
func (doc inlinePeekDocument) Lines(offset int) []string {
	body := doc.window.BodyRows()
	if doc.Empty() || body < 1 {
		return nil
	}
	start := doc.ClampOffset(offset)
	end := min(start+body, len(doc.Body))
	lines := make([]string, 0, doc.window.WindowRows())
	lines = append(lines, doc.Body[start:end]...)
	for len(lines) < body {
		lines = append(lines, "")
	}
	if start == 0 && end == len(doc.Body) {
		return append(lines, doc.indent)
	}
	return append(lines, doc.positionLine(start, len(doc.Body)-end))
}

// positionLine spends the reserved row on exact counts rather than an ordinal:
// what a reader needs to know is how much detail is still unread in each
// direction, and a page number only answers that after arithmetic — the more so
// once a window can stop between two pages.
func (doc inlinePeekDocument) positionLine(above, below int) string {
	counts := fmt.Sprintf("%d above · %d below", above, below)
	return doc.indent + theme.MutedStyle.Render(textfit.TruncateIntentional(counts, doc.width, "…"))
}

type inlinePeekOptions struct {
	Task   *models.Task
	Indent string
	Width  int
	// Window is the frame's dock geometry, which the document keeps so that its
	// own paging arithmetic and the render agree about the height on screen. It
	// is the frame's answer and never the document's: a long body and a short one
	// are drawn in the same window.
	Window peekDockGeometry
	// Compact prioritizes a readable resume passage in a crowded first page.
	// The document remains complete; expansion restores the ordinary order.
	Compact     bool
	Breadcrumb  string
	ProjectSlug string
	// SearchExcerpt explains a non-title search hit without replacing the detail.
	SearchExcerpt    string
	SearchExcerptKey string
	// Planning is the task's planning membership, `today`, `next`, or empty. It
	// arrives resolved for the same reason the two reference lists do: membership
	// is an explicit project-qualified collection in the planning state and is
	// readable nowhere on the task itself.
	Planning string
	// Dependencies and Dependents are the two read-only reference details. They
	// arrive resolved because Peek is a pure renderer and neither direction can
	// be read off the task alone: an outgoing entry needs the target project to
	// name it, and an incoming one exists nowhere but in the other tasks.
	Dependencies []DependencyDetail
	Dependents   []DependencyDetail
	// DependencyCensus is the same reading the row's `~S/N` painted, carried in
	// so Peek's Dependencies label repeats those numbers rather than recounting
	// them against a population of its own.
	DependencyCensus dependencyCensus
}

// inlinePeekSection is one labeled block of the body together with the
// width-independent key that identifies its content. The lines say what is
// drawn at this width; the key says what the section is, so a resize rewraps
// the document without making it a different one.
type inlinePeekSection struct {
	key   string
	lines []string
}

// inlinePeekDocumentFor assembles the additive, cursor-following task preview.
// Peek is the task's complete document rather than a supplement to the row: the
// row keeps an abbreviated form of a few signals and drops the rest, and every
// one of them is spelled here in full. The whole title leads, then the ancestor
// chain and the two summary lines, then the resume note and description, then
// the longer supporting material, and the task's history last.
func inlinePeekDocumentFor(opts inlinePeekOptions) inlinePeekDocument {
	if opts.Task == nil || opts.Width <= 0 {
		return inlinePeekDocument{}
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	sections := inlinePeekBodySections(opts, available)
	body := make([]string, 0, len(sections))
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		body = append(body, section.lines...)
		keys = append(keys, section.key)
	}

	return inlinePeekDocument{
		Body:        body,
		sections:    sections,
		Fingerprint: strings.Join(keys, "\x1f"),
		indent:      opts.Indent,
		width:       available,
		window:      opts.Window,
	}
}

// inlinePeekBodySections is the body's content order, one entry per section
// that has anything to say. Order is the whole of what this function decides:
// every section below renders itself in full and none of them competes with
// another for room, because room is the page's problem rather than theirs.
//
// One clock is read for the whole document, so the state age and the task clock
// beneath it cannot be measured against two instants that straddle a tick.
func inlinePeekBodySections(opts inlinePeekOptions, width int) []inlinePeekSection {
	task := opts.Task
	now := time.Now()
	sections := make([]inlinePeekSection, 0, 14)
	add := func(key string, lines []string) {
		if len(lines) == 0 {
			return
		}
		sections = append(sections, inlinePeekSection{key: key, lines: lines})
	}

	// The compact dock answers what this is and where to resume before spending
	// rows on project identity, ancestry, or scheduling detail. Every section
	// still belongs to the scrollable document at every size.
	title := inlinePeekSection{"title:" + task.Title, inlinePeekTitleLines(task, opts.Indent, width)}
	status := inlinePeekSection{"status:" + inlinePeekStatusKey(task, opts.Planning), inlinePeekStatusLines(task, opts.Planning, now, opts.Indent, width)}
	resume := inlinePeekSection{"resume:" + task.ResumeNote, inlinePeekResumeLines(task.ResumeNote, opts.Indent, width)}
	lead := []inlinePeekSection{title, status, resume}
	if opts.Compact && opts.Window.Displayed() && len(resume.lines) > 0 {
		// Two wrapped lines usually carry an instruction, rather than just its
		// preamble. Move metadata, then a long title, behind that passage without
		// eliding either or changing the dock's frame-owned height.
		passage := min(2, len(resume.lines))
		if len(title.lines)+passage > opts.Window.BodyRows() {
			lead = []inlinePeekSection{resume, title, status}
		} else if len(title.lines)+len(status.lines)+passage > opts.Window.BodyRows() {
			lead = []inlinePeekSection{title, resume, status}
		}
	}
	for _, section := range lead {
		add(section.key, section.lines)
	}
	add("project:"+opts.ProjectSlug, inlinePeekStyledLines(theme.MutedStyle.Render("Project: "), opts.ProjectSlug, opts.Indent, width))
	add("parent:"+opts.Breadcrumb, inlinePeekParentLines(opts.Breadcrumb, opts.Indent, width))
	add("match:"+opts.SearchExcerptKey, inlinePeekStyledLines(theme.MutedStyle.Render("Match: "), opts.SearchExcerpt, opts.Indent, width))
	add("schedule:"+inlinePeekScheduleKey(task), inlinePeekScheduleLines(task, now, opts.Indent, width))
	add("description:"+task.Description, inlinePeekDescriptionLines(task.Description, opts.Indent, width))

	// The three supporting counts sit under the task's own text, in the order a
	// reader reaches for them: the steps inside this task, the files beside it,
	// then the conversation about it.
	add("checklist:"+inlinePeekChecklistKey(task), inlinePeekChecklistLines(task, opts.Indent, width))
	add("attachments:"+inlinePeekAttachmentsKey(task), inlinePeekAttachmentsLines(task, opts.Indent, width))
	add("comments:"+strconv.Itoa(len(task.Comments)), inlinePeekCommentsLines(task, opts.Indent, width))

	// The two reference lists sit above tags because Peek is the only surface
	// that names a reference at all: the row carries `~S/N`, and there is no
	// panel behind it the way `;` and `&` stand behind comments and attachments.
	// Each label carries its own count — the outgoing one the row's very numbers
	// — so a reader sees the whole size of the set before reading it. No `?`
	// follows the fraction here: every entry below names its own state in full.
	dependencyLabel := "Dependencies: " + opts.DependencyCensus.counts() + " · "
	add(
		"dependencies:"+inlinePeekReferenceKey(opts.Dependencies),
		inlinePeekReferenceLines(dependencyLabel, opts.Dependencies, opts.Indent, width),
	)
	dependentLabel := fmt.Sprintf("Dependents: %d · ", len(opts.Dependents))
	add(
		"dependents:"+inlinePeekReferenceKey(opts.Dependents),
		inlinePeekReferenceLines(dependentLabel, opts.Dependents, opts.Indent, width),
	)

	// Context is its own section rather than a prefix on the tags line. Neither
	// reaches the row any more — a row is scanned for what to do next, and a
	// label is not that — so these two sections are the only place either is
	// readable, and both are rendered whole rather than truncated.
	add("context:"+task.Context, inlinePeekContextLines(task, opts.Indent, width))
	add("tags:"+strings.Join(listsurface.OrderedTaskTags(task.Tags), ","), inlinePeekTagsLines(task, opts.Indent, width))

	// History is last because nothing in it is owed. How old a task is, when it
	// was last touched or reviewed, and how often it has been pushed away are
	// what a reader consults after deciding what to do, so they sit below every
	// section that answers that question rather than between the schedule and
	// the task's own text.
	add("history:"+inlinePeekHistoryKey(task), inlinePeekHistoryLines(task, now, opts.Indent, width))
	return sections
}
