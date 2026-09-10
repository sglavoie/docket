package outliner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	// inlinePeekBodyRows is Peek's body budget. It is a window onto the detail
	// document rather than a cap on it: the document is assembled whole and this
	// many of its rows are on screen at a time, so a detail that does not fit is
	// a page away instead of gone.
	inlinePeekBodyRows = 5
	// inlinePeekPagedRows is what the body has left for content once the counter
	// row is on screen. The counter is only drawn when there is something off
	// screen to count, so an unpaged document keeps the whole budget.
	inlinePeekPagedRows = inlinePeekBodyRows - 1
	// inlinePeekHalfPageRows and inlinePeekFullPageRows are the two distances a
	// scroll travels. Both are counted in content rows rather than in pages, so
	// the window stops wherever the arithmetic lands instead of on a boundary the
	// document imposed. A full page leaves one content row on screen across the
	// move, which is what makes the jump readable: the row that ended the last
	// window leads the next one, so a wrapped sentence is never split across a
	// keystroke with nothing in common between the two halves. A half page is the
	// shorter step the same keys mean over every list in the product.
	inlinePeekHalfPageRows = 2
	inlinePeekFullPageRows = inlinePeekPagedRows - 1
	// inlinePeekTitleRows caps the title continuation alone. The continuation is
	// header rather than body — it completes what the row clipped — so it sits
	// outside the paged budget and outside its paging.
	inlinePeekTitleRows = 3
)

// inlinePeekDocument is the complete detail document behind Peek, separated
// from the page of it that is visible. Rendering and viewport measurement both
// consume it through Lines, so the height that is reserved and the height that
// is drawn are the same number by construction rather than by two agreeing
// calculations.
type inlinePeekDocument struct {
	// Header is the title continuation and the ancestor chain. Both complete
	// what the row itself had to elide, so they are never paged away: a page
	// turn moves the body under a stable heading.
	Header []string
	// Body is every detail row the task has, wrapped to the pane and in content
	// order. It is unbounded; inlinePeekBodyRows decides only how much of it is
	// on screen.
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
}

// Paged reports whether the body outgrows its budget. Nothing below the
// threshold reserves a counter row, so a short document keeps all five.
func (doc inlinePeekDocument) Paged() bool {
	return len(doc.Body) > inlinePeekBodyRows
}

// MaxOffset is the last content row the window may start on. The window is
// always full rather than short at the end of the document, so the final offset
// is pinned to the body's end and an unpaged document has the single offset
// zero.
func (doc inlinePeekDocument) MaxOffset() int {
	if !doc.Paged() {
		return 0
	}
	return len(doc.Body) - inlinePeekPagedRows
}

// ClampOffset bounds a content-row offset to the document. Every reader goes
// through it, so an offset remembered against a longer body narrows to the end
// here rather than slicing past it.
func (doc inlinePeekDocument) ClampOffset(offset int) int {
	return min(max(offset, 0), doc.MaxOffset())
}

// Lines renders the window at one content-row offset: the whole header, the
// content rows under it, and, when anything is off screen, the counter row that
// says exactly how much. It is the only way to either the drawn window or its
// height, so a render and a measurement cannot disagree about what is on screen.
func (doc inlinePeekDocument) Lines(offset int) []string {
	if len(doc.Header) == 0 && len(doc.Body) == 0 {
		return nil
	}
	lines := make([]string, 0, len(doc.Header)+inlinePeekBodyRows)
	lines = append(lines, doc.Header...)
	if !doc.Paged() {
		return append(lines, doc.Body...)
	}
	start := doc.ClampOffset(offset)
	end := start + inlinePeekPagedRows
	lines = append(lines, doc.Body[start:end]...)
	return append(lines, doc.counterLine(start, len(doc.Body)-end))
}

// counterLine spends the reserved row on exact counts rather than an ordinal:
// what a reader needs to know is how much detail is still unread in each
// direction, and a page number only answers that after arithmetic — the more so
// once a window can stop between two pages.
func (doc inlinePeekDocument) counterLine(above, below int) string {
	counts := fmt.Sprintf("%d above · %d below", above, below)
	return doc.indent + theme.MutedStyle.Render(textfit.TruncateIntentional(counts, doc.width, "…"))
}

type inlinePeekOptions struct {
	Task          *models.Task
	Indent        string
	Width         int
	TitleOverflow string
	Breadcrumb    string
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
// The selected row already owns task identity and operational metadata, so Peek
// contains only richer content: the resume note and description first, then
// dates, attachments, the comment count, the two reference lists, the context,
// and tags. A title the row had to clip is completed above them and a promoted
// root's ancestor chain follows it; both are header rather than body.
func inlinePeekDocumentFor(opts inlinePeekOptions) inlinePeekDocument {
	if opts.Task == nil || opts.Width <= 0 {
		return inlinePeekDocument{}
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	header := inlinePeekTitleLines(opts.Task, opts.TitleOverflow, opts.Indent, available)
	if parent := inlinePeekParentLine(opts.Breadcrumb, available); parent != "" {
		header = append(header, opts.Indent+parent)
	}

	sections := inlinePeekBodySections(opts, available)
	body := make([]string, 0, len(sections))
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		body = append(body, section.lines...)
		keys = append(keys, section.key)
	}
	if len(header) == 0 && len(body) == 0 {
		body = append(body, opts.Indent+theme.MutedStyle.Render("No additional details"))
	}

	return inlinePeekDocument{
		Header:      header,
		Body:        body,
		Fingerprint: strings.Join(keys, "\x1f"),
		indent:      opts.Indent,
		width:       available,
	}
}

// inlinePeekBodySections is the body's content order, one entry per section
// that has anything to say. Order is the whole of what this function decides:
// every section below renders itself in full and none of them competes with
// another for room, because room is the page's problem rather than theirs.
func inlinePeekBodySections(opts inlinePeekOptions, width int) []inlinePeekSection {
	task := opts.Task
	sections := make([]inlinePeekSection, 0, 9)
	add := func(key string, lines []string) {
		if len(lines) == 0 {
			return
		}
		sections = append(sections, inlinePeekSection{key: key, lines: lines})
	}

	// Resume leads the body because it answers the question Peek is opened to
	// ask — where was this left — and the description answers a different, older
	// one.
	add("resume:"+task.ResumeNote, inlinePeekResumeLines(task.ResumeNote, opts.Indent, width))
	add("description:"+task.Description, inlinePeekDescriptionLines(task.Description, opts.Indent, width))
	add("dates:"+inlinePeekDatesKey(task), inlinePeekDatesLines(task, time.Now(), opts.Indent, width))
	add("attachments:"+inlinePeekAttachmentsKey(task), inlinePeekAttachmentsLines(task, opts.Indent, width))
	add("comments:"+strconv.Itoa(len(task.Comments)), inlinePeekCommentsLines(task, opts.Indent, width))

	// The two reference lists sit above tags because Peek is the only surface
	// that names a reference at all: the row carries `~S/N`, and there is no
	// panel behind it the way `;` and `&` stand behind comments and attachments.
	// Each label carries its own count — the outgoing one the row's very numbers
	// — so a reader sees the whole size of the set before reading it. No `?`
	// follows the fraction here: every entry below names its own state in full.
	dependencyLabel := "Dependencies " + opts.DependencyCensus.counts() + ": "
	add(
		"dependencies:"+inlinePeekReferenceKey(opts.Dependencies),
		inlinePeekReferenceLines(dependencyLabel, opts.Dependencies, opts.Indent, width),
	)
	dependentLabel := fmt.Sprintf("Dependents %d: ", len(opts.Dependents))
	add(
		"dependents:"+inlinePeekReferenceKey(opts.Dependents),
		inlinePeekReferenceLines(dependentLabel, opts.Dependents, opts.Indent, width),
	)

	// Context is its own section rather than a prefix on the tags line. The row
	// reserves a segment for it at every width and truncates it there, so Peek
	// is where the whole value is readable, and a value long enough to need
	// wrapping is exactly the one the row could not show.
	add("context:"+task.Context, inlinePeekContextLines(task, opts.Indent, width))
	add("tags:"+strings.Join(listsurface.OrderedTaskTags(task.Tags), ","), inlinePeekTagsLines(task, opts.Indent, width))
	return sections
}

// inlinePeekParentLine names the ancestor chain the row reduced to a bare `‹`.
// It is header like the title continuation and for the same reason: the row
// deliberately elided it, and Peek is the surface that shows what the row had no
// width for. Root-first and uncompacted — Peek has the width the row did not.
func inlinePeekParentLine(breadcrumb string, width int) string {
	if width <= 0 {
		return ""
	}
	chain := strings.TrimSuffix(strings.TrimSpace(breadcrumb), " ›")
	if chain == "" {
		return ""
	}
	return textfit.TruncateIntentional(theme.MutedStyle.Render("Parent: ")+chain, width, "…")
}

// inlinePeekTitleLines completes a title the row clipped rather than restating
// it: the row keeps its ellipsis, which now reads as "continues below", and Peek
// carries only the remainder. No label and the task's own title styling, so it
// reads as the same title instead of as another Label: value line.
func inlinePeekTitleLines(task *models.Task, overflow, indent string, width int) []string {
	if task == nil || overflow == "" || width <= 0 {
		return nil
	}

	wrapped := textfit.WrapText(overflow, width)
	if len(wrapped) == 0 {
		return nil
	}
	if len(wrapped) > inlinePeekTitleRows {
		wrapped = wrapped[:inlinePeekTitleRows]
		wrapped[len(wrapped)-1] = textfit.TruncateIntentional(wrapped[len(wrapped)-1]+"…", width, "…")
	}

	style := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, indent+style.Render(textfit.TruncateIntentional(line, width, "…")))
	}
	return lines
}

// inlinePeekDatesLine names the task clock the row has no width for: how long
// ago it was created and last touched, when it is due, and whether it recently
// came back from a deferral. Labels stay muted and values take the footer's key
// color, so the durations read as data rather than as prose.
func inlinePeekDatesLine(task *models.Task, now time.Time, width int) string {
	if task == nil || width <= 0 {
		return ""
	}

	parts := make([]string, 0, 5)
	field := func(label, value string) {
		parts = append(parts, theme.HelpStyle.Render(label+" ")+theme.HelpKeyStyle.Render(value))
	}
	if age := query.FormatRecencyAge(task.CreatedAt, now); age != "" {
		field("created", age)
	}
	if age := query.FormatRecencyAge(task.UpdatedAt, now); age != "" {
		field("updated", age)
	}
	if task.DueDate != nil {
		field("due", chrome.DueGrammarValue(task))
	}
	if taskWokeRecently(task, now) {
		field("woke", query.FormatRecencyAge(*task.DeferredUntil, now))
	}
	if taskHasRepeatedDeferrals(task) {
		field("deferred", fmt.Sprintf("×%d", task.DeferCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, theme.HelpStyle.Render(" · "))
}

func inlinePeekDatesLines(task *models.Task, now time.Time, indent string, width int) []string {
	return inlinePeekStyledLines("", inlinePeekDatesLine(task, now, width), indent, width)
}

// inlinePeekDatesKey identifies the clock by its stored instants rather than by
// the rendered durations, which drift with every tick and would otherwise
// restart Peek's paging on the minute.
func inlinePeekDatesKey(task *models.Task) string {
	parts := []string{
		task.CreatedAt.Format(time.RFC3339),
		task.UpdatedAt.Format(time.RFC3339),
		strconv.Itoa(task.DeferCount),
	}
	if task.DueDate != nil {
		parts = append(parts, task.DueDate.Format(time.RFC3339))
	}
	if task.DeferredUntil != nil {
		parts = append(parts, task.DeferredUntil.Format(time.RFC3339))
	}
	return strings.Join(parts, "|")
}

// inlinePeekAttachmentsLines names every attachment, one per row. The row
// counts them and the `&` panel opens them; Peek is where the names are read,
// so a name too long for the pane wraps rather than joining an elided tail.
func inlinePeekAttachmentsLines(task *models.Task, indent string, width int) []string {
	if task == nil || len(task.Attachments) == 0 {
		return nil
	}
	entries := make([]string, 0, len(task.Attachments))
	for _, attachment := range task.Attachments {
		if name := strings.TrimSpace(listsurface.AttachmentDisplayName(attachment)); name != "" {
			entries = append(entries, name)
		}
	}
	return inlinePeekEntryLines("Attachments: ", entries, indent, width)
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
// Comments panel, which reads, edits, reorders, and deletes them; repeating one
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
