package outliner

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
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
	Body []string
	// Fingerprint identifies the content the body was built from, independent of
	// the width it was wrapped to. It is what lets a stored offset survive a
	// re-render and fall back to the top when the detail, the reference
	// population, or the task itself changed.
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
	Window     peekDockGeometry
	Breadcrumb string
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

	// Identity leads: the whole title, then the ancestor chain the row reduced to
	// a bare `‹`. A reader who opened Peek on a clipped row is looking for the
	// rest of that title, so nothing precedes it.
	add("title:"+task.Title, inlinePeekTitleLines(task, opts.Indent, width))
	add("parent:"+opts.Breadcrumb, inlinePeekParentLines(opts.Breadcrumb, opts.Indent, width))
	add("match:"+opts.SearchExcerptKey, inlinePeekStyledLines(theme.MutedStyle.Render("Match: "), opts.SearchExcerpt, opts.Indent, width))

	// The two summary lines follow, densest first: what the task is right now,
	// then what it still owes. Both are `Label: value` pairs on one wrapped line
	// rather than a section each, because a row of six one-word facts would spend
	// the whole window saying what fits in two. The task's spent instants are not
	// here: they are history, and history is the last thing in the document.
	add("status:"+inlinePeekStatusKey(task, opts.Planning), inlinePeekStatusLines(task, opts.Planning, now, opts.Indent, width))
	add("schedule:"+inlinePeekScheduleKey(task), inlinePeekScheduleLines(task, now, opts.Indent, width))

	// Resume precedes the description because it answers the question Peek is
	// opened to ask — where was this left — and the description answers a
	// different, older one.
	add("resume:"+task.ResumeNote, inlinePeekResumeLines(task.ResumeNote, opts.Indent, width))
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

// inlinePeekParentLines names the ancestor chain the row reduced to a bare `‹`.
// Root-first and uncompacted — Peek has the width the row did not — and wrapped
// rather than elided, so a chain six levels deep is read to its end by scrolling
// like every other long section.
func inlinePeekParentLines(breadcrumb, indent string, width int) []string {
	chain := strings.TrimSuffix(strings.TrimSpace(breadcrumb), " ›")
	if chain == "" {
		return nil
	}
	return inlinePeekStyledLines(theme.MutedStyle.Render("Parent: "), chain, indent, width)
}

// inlinePeekTitleLines is the task's whole title, wrapped to the pane and elided
// nowhere. The row spends its width on badges and clips the title to whatever is
// left, so Peek is where the title is read, and it leads the body rather than
// standing outside it: a title long enough to fill the pane is reached by
// scrolling exactly as a long description is. No label and the task's own title
// styling, so it reads as the title instead of as another Label: value line.
func inlinePeekTitleLines(task *models.Task, indent string, width int) []string {
	if task == nil || width <= 0 {
		return nil
	}
	title := strings.TrimSpace(task.Title)
	if title == "" {
		return nil
	}
	style := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
	paint := func(row string) string { return style.Render(row) }
	return inlinePeekHangingLines("", textfit.WrapText(title, width), indent, width, paint)
}

// inlinePeekFields collects `Label: value` pairs into one dense line. Peek has
// three such lines — what the task is, what it still owes, and what it has
// already spent — and they share this so a label reads the same way on all
// three: muted label, keyed value, a middot between fields. An empty value is
// no field, which is how each line omits what the task does not carry. Nothing
// here measures: the line is handed to the ordinary wrapping layout like every
// other section.
type inlinePeekFields struct {
	parts []string
}

func (f *inlinePeekFields) add(label, value string) {
	if value == "" {
		return
	}
	f.parts = append(f.parts, theme.HelpStyle.Render(label+": ")+theme.HelpKeyStyle.Render(value))
}

// flag is a field that is its own value. Pinned and Review are booleans a task
// either wears or does not, so a label beside one would say the word twice.
func (f *inlinePeekFields) flag(word string) {
	f.parts = append(f.parts, theme.HelpKeyStyle.Render(word))
}

func (f inlinePeekFields) line() string {
	if len(f.parts) == 0 {
		return ""
	}
	return strings.Join(f.parts, theme.HelpStyle.Render(" · "))
}

// inlinePeekStatusLine spells what the row's badges abbreviate and what it drops
// entirely: the planning collection the task belongs to, the lifecycle state in
// words rather than as a glyph, the assigned priority as a number, and the two
// independent boolean axes. The status is never omitted — every task is in one —
// so a task Peek always has at least this line to show.
//
// Plan leads and status follows it, which is the order the workflow chord and the
// capture route both read in, and the two sit adjacent so the pair is read as one
// answer: a task planned for Today is in Backlog, and the line says so rather
// than leaving a reader to know it. They are two axes and not two values of one,
// so an absent plan drops its field and changes nothing about the status beside
// it.
func inlinePeekStatusLine(task *models.Task, planning string, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	fields.add(chrome.PlanFieldLabel, planning)
	fields.add(chrome.StatusFieldLabel, inlinePeekStatusValue(task, now))
	// The same gate theme.PriorityIcon takes, so a task the row draws no priority
	// badge for names no priority here either.
	if models.ValidatePriority(task.Priority, false) == nil {
		fields.add("Priority", strconv.Itoa(task.Priority)+" "+strings.ToLower(models.PriorityName(task.Priority)))
	}
	if task.Pinned {
		fields.flag("Pinned")
	}
	if task.Review {
		fields.flag("Review")
	}
	return fields.line()
}

func inlinePeekStatusLines(task *models.Task, planning string, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekStatusLine(task, planning, now), indent, width)
}

// inlinePeekStatusValue names the lifecycle state and, for the two states whose
// age is a question about the work rather than about the record, how long it has
// held it. StateChangedAt is what it reads, so commenting on a blocked task does
// not reset how long it has been blocked.
func inlinePeekStatusValue(task *models.Task, now time.Time) string {
	state := chrome.PlanStatusValue(task.State().String())
	if !task.IsInState(models.StateDoing) && !task.IsInState(models.StateWaiting) {
		return state
	}
	age := query.FormatRecencyAge(task.StateChangedAt, now)
	if age == "" {
		return state
	}
	return state + " " + age
}

// inlinePeekStatusKey identifies the summary by its stored values, the state
// clock included rather than the age rendered from it: the age drifts with every
// tick and would otherwise return Peek's window to the top on the minute.
func inlinePeekStatusKey(task *models.Task, planning string) string {
	return strings.Join([]string{
		"plan:" + planning,
		"status:" + task.State().String(),
		"status-changed:" + task.StateChangedAt.Format(time.RFC3339Nano),
		"priority:" + strconv.Itoa(task.Priority),
		"pinned:" + strconv.FormatBool(task.Pinned),
		"review:" + strconv.FormatBool(task.Review),
	}, "|")
}

// inlinePeekChecklistLines keeps the checklist a count. It is the only place the
// tally is stated now that the row carries none, but it stays a tally: the items
// themselves are rows of their own under the task, where they are edited, ticked,
// and promoted, and repeating their text here would spend the window on content
// the outline already holds one keypress away.
func inlinePeekChecklistLines(task *models.Task, indent string, width int) []string {
	if task == nil || len(task.Checklist) == 0 {
		return nil
	}
	done := 0
	for _, item := range task.Checklist {
		if item.Done {
			done++
		}
	}
	entry := fmt.Sprintf("%d/%d done", done, len(task.Checklist))
	return inlinePeekEntryLines("Checklist: ", []string{entry}, indent, width)
}

func inlinePeekChecklistKey(task *models.Task) string {
	states := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		states = append(states, strconv.FormatBool(item.Done))
	}
	return strings.Join(states, ",")
}

// inlinePeekScheduleLine names what the task still owes: its deadline, the day a
// pending deferral brings it back, and its check-in with the cadence that
// repeats it. Every field here is a date in front of the task, which is why they
// are one line beneath the status summary rather than mixed in with the instants
// the task has already spent — a reader deciding what to do next reads this line
// and stops.
//
// The row keeps the deadline alone, so the other three are read here.
// Labels stay muted and values take the footer's key color, so the dates read as
// data rather than as prose.
func inlinePeekScheduleLine(task *models.Task, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	if task.DueDate != nil {
		fields.add("Due", chrome.FormatWeekdayDate(*task.DueDate, now, task.DueHasTime())+" ("+chrome.DayGloss(*task.DueDate, now)+")")
	}
	// The pending half of the deferral alone. Its lapsed half is a wake that
	// already happened, so it is history and is named there instead.
	if task.DeferredUntil != nil && task.IsDeferredAt(now) {
		fields.add("Defer", chrome.FormatDeferDate(*task.DeferredUntil, now))
	}
	if task.NextCheckIn != nil {
		// An arrived check-in is what the row calls `review`, so the gloss says
		// so: one fact keeps one name across the row and its document.
		gloss := chrome.DayGloss(*task.NextCheckIn, now)
		if task.CheckInIsDueAt(now) && models.TaskNeedsAttention(task, now) {
			gloss += ", review owed"
		}
		fields.add("Check-in", chrome.FormatWeekdayDate(*task.NextCheckIn, now, false)+" ("+gloss+")")
		// "every" keeps a cadence from reading as the row's lateness `+Nd`.
		repeat := "off"
		if task.CheckInRepeatDays > models.CheckInRepeatOff {
			repeat = fmt.Sprintf("every %dd", task.CheckInRepeatDays)
		}
		fields.add("Repeat", repeat)
	}
	return fields.line()
}

func inlinePeekScheduleLines(task *models.Task, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekScheduleLine(task, now), indent, width)
}

// inlinePeekScheduleKey identifies the schedule by its stored instants rather
// than by anything rendered from the clock, so a tick that moves no stored value
// leaves Peek's window where the reader left it.
func inlinePeekScheduleKey(task *models.Task) string {
	parts := []string{"check-in-repeat:" + strconv.Itoa(task.CheckInRepeatDays)}
	if task.DueDate != nil {
		parts = append(parts, "due:"+task.DueDate.Format(time.RFC3339Nano))
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "deferred:"+task.DeferredUntil.Format(time.RFC3339Nano))
	}
	if task.NextCheckIn != nil {
		parts = append(parts, "check-in:"+task.NextCheckIn.Format(time.RFC3339Nano))
	}
	return strings.Join(parts, "|")
}

// inlinePeekHistoryLine names the instants the task has already spent: how long
// ago it was created and last touched (the touch only when it differs from the
// creation to the second), when it was last reviewed, whether it
// recently came back from a deferral, and how many times it has been deferred.
// None of them is owed, so they close the document rather than standing between
// the schedule and the task's own text.
func inlinePeekHistoryLine(task *models.Task, now time.Time) string {
	if task == nil {
		return ""
	}
	var fields inlinePeekFields
	fields.add("Created", inlinePeekPastAge(task.CreatedAt, now))
	// A task never touched since its creation was updated when it was created,
	// so a second instant saying the same thing is noise.
	if !sameStoredSecond(task.CreatedAt, task.UpdatedAt) {
		fields.add("Updated", inlinePeekPastAge(task.UpdatedAt, now))
	}
	if task.LastReviewedAt != nil {
		fields.add("Last review", inlinePeekPastAge(*task.LastReviewedAt, now))
	}
	// The lapsed half of the deferral: a task that came back is work again, and
	// the row carries no deferral at all, so this is where the wake is said.
	if taskWokeRecently(task, now) {
		fields.add("Woke", inlinePeekPastAge(*task.DeferredUntil, now))
	}
	if taskHasRepeatedDeferrals(task) {
		fields.add("Deferred", fmt.Sprintf("×%d", task.DeferCount))
	}
	return fields.line()
}

// sameStoredSecond reports whether two stored instants name the same second,
// which is the precision a creation and its first write share.
func sameStoredSecond(a, b time.Time) bool {
	return a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

// inlinePeekPastAge spells an instant already spent as how long ago it was, so
// `47d ago` cannot be read as a duration the way the status line's `Doing 9d`
// is. `now` already says when, and an empty age stays empty so the field drops.
func inlinePeekPastAge(at, now time.Time) string {
	age := query.FormatRecencyAge(at, now)
	if age == "" || age == "now" {
		return age
	}
	return age + " ago"
}

func inlinePeekHistoryLines(task *models.Task, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekHistoryLine(task, now), indent, width)
}

// inlinePeekHistoryKey identifies the history by its stored instants rather than
// by the rendered durations, which drift with every tick and would otherwise
// restart Peek's paging on the minute. The deferral instant is keyed here as
// well as in the schedule because the wake is read off it: a section's key must
// identify that section's own content whether or not the other one is drawn.
func inlinePeekHistoryKey(task *models.Task) string {
	parts := []string{
		"created:" + task.CreatedAt.Format(time.RFC3339Nano),
		"updated:" + task.UpdatedAt.Format(time.RFC3339Nano),
		"defer-count:" + strconv.Itoa(task.DeferCount),
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "deferred:"+task.DeferredUntil.Format(time.RFC3339Nano))
	}
	if task.LastReviewedAt != nil {
		parts = append(parts, "last-review:"+task.LastReviewedAt.Format(time.RFC3339Nano))
	}
	return strings.Join(parts, "|")
}

// inlinePeekAttachmentsLines names every attachment, one per row, under a label
// carrying their count. The `&` panel opens them; Peek is where the names are
// read, so a name too long for the pane wraps rather than joining an elided
// tail, and the count says how many rows the section is worth scrolling for
// before the first of them is read.
//
// A URL or a code reference also links its rows (OSC 8), each wrapped row on
// its own, so a terminal that follows links opens the whole target from any
// row of it — the full URL behind a shortened GitHub issue name included. A
// stored file is the panel's to open, since its path needs the project.
func inlinePeekAttachmentsLines(task *models.Task, indent string, width int) []string {
	if task == nil || len(task.Attachments) == 0 || width <= 0 {
		return nil
	}
	count := 0
	for _, attachment := range task.Attachments {
		if strings.TrimSpace(listsurface.AttachmentDisplayName(attachment)) != "" {
			count++
		}
	}
	label := fmt.Sprintf("Attachments: %d · ", count)
	bodyWidth := max(width-lipgloss.Width(label), 1)
	chunks := make([]string, 0, count)
	for _, attachment := range task.Attachments {
		name := strings.TrimSpace(listsurface.AttachmentDisplayName(attachment))
		if name == "" {
			continue
		}
		target := attachmentLinkTarget(attachment)
		for _, chunk := range textfit.WrapText(name, bodyWidth) {
			chunks = append(chunks, textfit.Hyperlink(target, chunk))
		}
	}
	return inlinePeekHangingLines(label, chunks, indent, width, inlinePeekMutedRow)
}

// attachmentLinkTarget is the URI an attachment's Peek rows link to: a URL as
// stored, a code reference as a file URL to its absolute path, and nothing for
// a stored file or a relative reference, which cannot be resolved here.
func attachmentLinkTarget(attachment models.Attachment) string {
	switch {
	case attachment.IsURL:
		return attachment.Filename
	case attachment.IsCodeRef && filepath.IsAbs(attachment.OriginalPath):
		return (&url.URL{Scheme: "file", Path: attachment.OriginalPath}).String()
	default:
		return ""
	}
}

func inlinePeekAttachmentsKey(task *models.Task) string {
	names := make([]string, 0, len(task.Attachments))
	for _, attachment := range task.Attachments {
		names = append(names, listsurface.AttachmentDisplayName(attachment))
	}
	return strings.Join(names, ",")
}

// inlinePeekReferenceLines renders one read-only reference section, one entry
// per row. An entry that names an unavailable target keeps its stored locator
// and its state rather than being dropped: a reference whose target cannot be
// read is exactly the one worth seeing. The list is non-navigable — there is no
// cursor in it and no key acts on an entry — because a reference is information
// about other work, not a way to reach it.
func inlinePeekReferenceLines(label string, details []DependencyDetail, indent string, width int) []string {
	if len(details) == 0 {
		return nil
	}
	entries := make([]string, 0, len(details))
	for _, detail := range details {
		entries = append(entries, detail.String())
	}
	return inlinePeekEntryLines(label, entries, indent, width)
}

func inlinePeekReferenceKey(details []DependencyDetail) string {
	entries := make([]string, 0, len(details))
	for _, detail := range details {
		entries = append(entries, detail.String())
	}
	return strings.Join(entries, ",")
}

// inlinePeekCommentsLines keeps comments a count. The bodies belong to the
// Comments panel, which reads, edits, and deletes them; repeating one
// here would spend the page budget on a surface that already exists.
func inlinePeekCommentsLines(task *models.Task, indent string, width int) []string {
	if task == nil || len(task.Comments) == 0 {
		return nil
	}
	return inlinePeekEntryLines("Comments: ", []string{strconv.Itoa(len(task.Comments))}, indent, width)
}

func inlinePeekContextLines(task *models.Task, indent string, width int) []string {
	if task == nil {
		return nil
	}
	context := strings.TrimSpace(task.Context)
	if context == "" {
		return nil
	}
	badge := theme.TaskContextBadgeStyle(task).Render("[" + context + "]")
	return inlinePeekStyledLines(theme.MutedStyle.Render("Context: "), badge, indent, width)
}

func inlinePeekTagsLines(task *models.Task, indent string, width int) []string {
	if task == nil {
		return nil
	}
	chips := make([]string, 0, len(task.Tags))
	for _, tag := range listsurface.OrderedTaskTags(task.Tags) {
		if tag = strings.TrimSpace(tag); tag != "" {
			chips = append(chips, theme.TagChipStyle(tag).Render("#"+tag))
		}
	}
	if len(chips) == 0 {
		return nil
	}
	return inlinePeekStyledLines(theme.MutedStyle.Render("Tags: "), strings.Join(chips, " "), indent, width)
}

func inlinePeekDescriptionLines(description, indent string, width int) []string {
	return inlinePeekWrappedBodyLines("Description: ", description, indent, width)
}

// inlinePeekResumeLines renders the resume note above the description. Both are
// whole here: models.ValidateResumeNote already bounds the note, and a
// description long enough to page is what paging is for.
func inlinePeekResumeLines(note, indent string, width int) []string {
	return inlinePeekWrappedBodyLines("Resume: ", note, indent, width)
}

// inlinePeekWrappedBodyLines is the one renderer behind every labeled, wrapped
// Peek body. The resume note and the description differ only in their label, so
// they share this rather than each owning a copy of the hanging-indent rules.
// Blank rows inside the body are dropped rather than wrapped: a page is five
// rows wide and an empty one buys nothing.
func inlinePeekWrappedBodyLines(label, body, indent string, width int) []string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" || width <= 0 {
		return nil
	}

	bodyWidth := max(width-lipgloss.Width(label), 1)
	chunks := make([]string, 0, 4)
	for _, line := range textfit.WrapText(body, bodyWidth) {
		if strings.TrimSpace(line) != "" {
			chunks = append(chunks, line)
		}
	}
	return inlinePeekHangingLines(label, chunks, indent, width, inlinePeekMutedRow)
}

// inlinePeekEntryLines lays a list section out one entry per row, wrapping an
// entry too long for the pane under the same hanging indent. Nothing is elided
// and nothing is counted in place of being named: the page is what bounds the
// section now.
func inlinePeekEntryLines(label string, entries []string, indent string, width int) []string {
	if len(entries) == 0 || width <= 0 {
		return nil
	}
	bodyWidth := max(width-lipgloss.Width(label), 1)
	chunks := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry = strings.TrimSpace(entry); entry == "" {
			continue
		}
		chunks = append(chunks, textfit.WrapText(entry, bodyWidth)...)
	}
	return inlinePeekHangingLines(label, chunks, indent, width, inlinePeekMutedRow)
}

// inlinePeekMutedRow is the plain sections' painter, named so the hanging
// layout takes one row at a time rather than lipgloss' variadic renderer.
func inlinePeekMutedRow(row string) string {
	return theme.MutedStyle.Render(row)
}

// inlinePeekStyledLines is the same layout over content that already carries
// its own color. The wrap is ANSI-aware and the rows are left unpainted, so a
// tag chip or a context badge keeps the color it was minted with instead of
// being rendered inside a second style.
func inlinePeekStyledLines(label, content, indent string, width int) []string {
	if content == "" || width <= 0 {
		return nil
	}
	bodyWidth := max(width-lipgloss.Width(label), 1)
	return inlinePeekHangingLines(label, textfit.WrapStyled(content, bodyWidth), indent, width, nil)
}

// inlinePeekHangingLines assembles one section: the label leads its first row
// and later rows hang beneath it, every row indented to the task's own column
// and clamped to the pane. paint is the section's styling, applied per row so a
// styled composite is never re-wrapped inside another style; a nil paint leaves
// the content as it was minted.
func inlinePeekHangingLines(label string, chunks []string, indent string, width int, paint func(string) string) []string {
	if len(chunks) == 0 || width <= 0 {
		return nil
	}
	hanging := strings.Repeat(" ", lipgloss.Width(label))
	lines := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		prefix := hanging
		if i == 0 {
			prefix = label
		}
		row := textfit.TruncateToVisualWidth(prefix+chunk, width)
		if paint != nil {
			row = paint(row)
		}
		lines = append(lines, indent+row)
	}
	return lines
}

// searchMatchExcerpt shows at most two bounded windows, so distant search
// terms can explain a result without turning Peek into a second full document.
func searchMatchExcerpt(text string, positions []int) string {
	const contextBefore, excerptRunes = 40, 160
	runes := []rune(text)
	var windows []string
	end := 0
	for _, position := range positions {
		if position < end || position < 0 || position >= len(runes) {
			continue
		}
		start := max(position-contextBefore, end)
		end = min(start+excerptRunes, len(runes))
		part := highlightedExcerptWindow(runes, positions, start, end)
		if start > 0 {
			part = "…" + part
		}
		if end < len(runes) {
			part += "…"
		}
		windows = append(windows, part)
		if len(windows) == 2 {
			break
		}
	}
	return strings.Join(windows, " · ")
}

// searchMatchExcerpt's first window fitted to width without losing what it
// explains: the leading context shrinks until the first matched run — the
// consecutive positions starting at the first match — fits whole, and the tail
// is then cut. A width that cannot hold that run even with no context returns
// "", since an excerpt whose match was cut away is context and not a reason.
func searchMatchExcerptWithin(text string, positions []int, width int) string {
	if full := searchMatchExcerpt(text, positions); lipgloss.Width(full) <= width {
		return full
	}
	runes := []rune(text)
	first := -1
	for _, position := range positions {
		if position >= 0 && position < len(runes) {
			first = position
			break
		}
	}
	if first < 0 {
		return ""
	}
	runEnd := first + 1
	for _, position := range positions {
		if position == runEnd {
			runEnd++
		}
	}
	const contextBefore, excerptRunes = 40, 160
	for context := contextBefore; context >= 0; context-- {
		start := max(first-context, 0)
		// The kept run plus its lead-in, and one column for the ellipsis the
		// tail cut adds whenever anything follows it.
		kept := highlightedExcerptWindow(runes, positions, start, runEnd)
		if start > 0 {
			kept = "…" + kept
		}
		end := min(start+excerptRunes, len(runes))
		need := lipgloss.Width(kept)
		if end > runEnd {
			need++
		}
		if need > width {
			continue
		}
		part := highlightedExcerptWindow(runes, positions, start, end)
		if start > 0 {
			part = "…" + part
		}
		if end < len(runes) {
			part += "…"
		}
		return textfit.TruncateToVisualWidth(part, width)
	}
	return ""
}

// Normalize whitespace before styling, preserving the matcher's rune positions
// through newlines, repeated spaces and multibyte characters.
func highlightedExcerptWindow(runes []rune, positions []int, start, end int) string {
	matches := make(map[int]bool, len(positions))
	for _, position := range positions {
		matches[position] = true
	}
	var text []rune
	var highlights []int
	space := false
	for i := start; i < end; i++ {
		if unicode.IsSpace(runes[i]) {
			space = len(text) > 0
			continue
		}
		if space {
			text = append(text, ' ')
			space = false
		}
		if matches[i] {
			highlights = append(highlights, len(text))
		}
		text = append(text, runes[i])
	}
	return textfit.HighlightText(string(text), highlights, theme.MutedStyle)
}
