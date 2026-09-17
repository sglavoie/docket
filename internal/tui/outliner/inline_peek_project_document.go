package outliner

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file is Peek's second document kind. It shares inlinePeekDocument whole —
// the same window arithmetic, the same counter row, the same four scrolling keys
// — and owns only what a project has to say that a task does not. Nothing here
// measures or clips: the page is what bounds a project with a hundred comments,
// exactly as it bounds a task with a long description.

// inlinePeekProjectOptions is the project half of Peek's input. It carries the
// resolved project rather than a row, because the document is a pure renderer:
// the row's explicit identity is resolved by the caller, so a header naming a
// project the population no longer holds produces no document instead of a
// fabricated empty one.
type inlinePeekProjectOptions struct {
	Project *models.Project
	Indent  string
	Width   int
	// Window is the frame's dock geometry, carried for the same reason the task
	// half carries it: a project header's document is drawn in the identical
	// window a task's is, so the two cannot disagree about height.
	Window peekDockGeometry
}

// inlinePeekProjectDocumentFor assembles the detail behind a project header. The
// header row spends its width on the slug badge, the group size, and the
// pressure chips, so everything below is what the row has no room to say: the
// project's own name, then its description, its tags, and every stored comment
// newest first. All four are body, so a long name pages like a long comment.
func inlinePeekProjectDocumentFor(opts inlinePeekProjectOptions) inlinePeekDocument {
	if opts.Project == nil || opts.Width <= 0 {
		return inlinePeekDocument{}
	}

	available := max(opts.Width-lipgloss.Width(opts.Indent), 1)
	sections := inlinePeekProjectBodySections(opts.Project, opts.Indent, available)
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

// inlinePeekProjectBodySections is the project body's content order. The comment
// section is appended unconditionally rather than through add, because a project
// with nothing to say still owes the reader the sentence saying so — and that
// keeps every project's document non-empty, so a header always has a Peek.
func inlinePeekProjectBodySections(project *models.Project, indent string, width int) []inlinePeekSection {
	sections := make([]inlinePeekSection, 0, 4)
	add := func(key string, lines []string) {
		if len(lines) == 0 {
			return
		}
		sections = append(sections, inlinePeekSection{key: key, lines: lines})
	}

	add("name:"+project.Name, inlinePeekProjectNameLines(project, indent, width))
	add("description:"+project.Description, inlinePeekDescriptionLines(project.Description, indent, width))
	add("tags:"+strings.Join(project.Tags, ","), inlinePeekProjectTagLines(project.Tags, indent, width))
	return append(sections, inlinePeekProjectCommentSection(project.Comments, indent, width))
}

// inlinePeekProjectNameLines is the project's name, leading its body the way a
// task's title leads that one: wrapped to the pane, elided nowhere, and reached
// by scrolling when it is long. It is a name rather than a continuation because
// the header row never showed one — the badge carries the slug, which is the
// portable directory component and not what the project is called.
func inlinePeekProjectNameLines(project *models.Project, indent string, width int) []string {
	name := strings.TrimSpace(project.Name)
	if name == "" || width <= 0 {
		return nil
	}
	style := theme.ProjectColorStyle(project).Bold(true)
	paint := func(row string) string { return style.Render(row) }
	return inlinePeekHangingLines("", textfit.WrapText(name, width), indent, width, paint)
}

func inlinePeekProjectTagLines(tags []string, indent string, width int) []string {
	chips := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			chips = append(chips, theme.TagChipStyle(tag).Render("#"+tag))
		}
	}
	if len(chips) == 0 {
		return nil
	}
	return inlinePeekStyledLines(theme.MutedStyle.Render("Tags: "), strings.Join(chips, " "), indent, width)
}

// inlinePeekProjectCommentLabel is the section's hanging label. Both arms spell
// it — one styled and one bare — so the entries and the empty sentence start in
// the same column.
const inlinePeekProjectCommentLabel = "Comments: "

// inlinePeekProjectCommentSection renders the comment bodies themselves, which
// is the whole reason a project Peek exists: a task's comments have the `;`
// panel standing behind their count, and so do a project's, but a project header
// carries no count at all — the row has no column for one. Newest first, matching
// the panel: storage keeps them oldest first because that order is canonical and
// byte-stable, and reversing for a reader touches nothing stored.
func inlinePeekProjectCommentSection(comments []*models.ProjectComment, indent string, width int) inlinePeekSection {
	label := theme.MutedStyle.Render(inlinePeekProjectCommentLabel)
	bodyWidth := max(width-lipgloss.Width(label), 1)

	chunks := make([]string, 0, len(comments)*2)
	keys := make([]string, 0, len(comments))
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if comment == nil {
			continue
		}
		// The key names the stored value alone. The rendered stamp carries a
		// relative age beside it, which drifts every minute; keying on that would
		// return the window to the top on the clock tick.
		keys = append(keys, comment.ID+"@"+comment.CreatedAt.Format(time.RFC3339Nano)+":"+comment.Text)
		chunks = append(chunks, theme.MutedStyle.Render(chrome.FormatTimestampBracket(comment.CreatedAt)))
		chunks = append(chunks, inlinePeekProjectCommentTextRows(comment.Text, bodyWidth)...)
	}

	if len(chunks) == 0 {
		return inlinePeekSection{
			key:   "comments:none",
			lines: inlinePeekEntryLines(inlinePeekProjectCommentLabel, []string{"No comments yet."}, indent, width),
		}
	}
	return inlinePeekSection{
		key: "comments:" + strings.Join(keys, "\x1e"),
		// Nothing is painted over these rows: the stamps arrive muted and the
		// bodies deliberately do not, so a comment reads as text under its date
		// rather than as one more dimmed metadata line.
		lines: inlinePeekHangingLines(label, chunks, indent, width, nil),
	}
}

// inlinePeekProjectCommentTextRows wraps one comment's body under its stamp. The
// extra two columns are the panel's own indent, reduced to what a five-row pane
// can afford; blank rows inside a body are dropped for the reason every other
// Peek body drops them, and every remaining row stays in the document, so a long
// comment is a page away rather than an ellipsis.
func inlinePeekProjectCommentTextRows(text string, width int) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" || width <= 0 {
		return nil
	}
	const hanging = "  "
	rows := make([]string, 0, 2)
	for _, line := range textfit.WrapText(text, max(width-len(hanging), 1)) {
		if strings.TrimSpace(line) != "" {
			rows = append(rows, hanging+line)
		}
	}
	return rows
}
