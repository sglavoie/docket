package outliner

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

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
	return inlinePeekStyledLines("", textfit.RenderTaskTitle(title, nil, style), indent, width)
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
