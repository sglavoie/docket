package outliner

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/keymap"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The full-screen reader borrows the presentation without moving the outline's
// cursor or viewport. Closing restores even the compact dock's reading position.
type fullscreenPeekReturn struct {
	open bool
	peek peekOffsetState
}

func (m *OutlinerModel) ToggleFullscreenPeek() {
	if m == nil || m.MultiSelectMode {
		return
	}
	if m.PeekFullscreen {
		m.closeFullscreenPeek()
		return
	}
	if !m.PeekMode && !m.cursorRowSupportsPeek() {
		return
	}
	m.fullscreenReturn = fullscreenPeekReturn{open: m.PeekMode, peek: m.peek}
	m.PeekMode, m.PeekFullscreen = true, true
	m.peek = peekOffsetState{}
}

func (m *OutlinerModel) closeFullscreenPeek() {
	m.PeekFullscreen = false
	m.PeekMode, m.peek = m.fullscreenReturn.open, m.fullscreenReturn.peek
	m.fullscreenReturn = fullscreenPeekReturn{}
}

func (m *OutlinerModel) fullscreenPeekTitle() string {
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		row := m.Rows[m.Cursor]
		if row.IsProjectHeader {
			return "Project Details"
		}
		if row.IsSectionHeader {
			return "Section Details"
		}
		if row.IsChecklistItem {
			return "Checklist Item Details"
		}
	}
	return "Task Details"
}

// This is a gated reader keymap, like the expanded query readout. The registry
// still owns B's entry operation; inside the reader, keys scroll its document
// and never dispatch mutations or navigate an invisible list.
func (m *OutlinerModel) fullscreenPeekKeyMap() *keymap.KeyMap {
	return &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("esc", "Return to list", "Navigation", keymap.WithFooter(1, "return to list", true), keymap.WithFooterKey("Esc/B"), keymap.WithHelpKey("Esc/B/b")),
		keymap.Bind("B", "Return to list", "Navigation", keymap.AsHidden()),
		keymap.Bind("b", "Return to list", "Navigation", keymap.AsHidden()),
		keymap.Bind("j", "Scroll down", "Navigation", keymap.WithFooter(2, "scroll", false), keymap.WithFooterKey("j/k"), keymap.WithHelpKey("j/k/↑/↓")),
		keymap.Bind("k", "Scroll up", "Navigation", keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Navigation", keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Navigation", keymap.AsHidden()),
		keymap.Bind("ctrl+d", "Scroll half a page down", "Navigation", keymap.WithHelpKey("Ctrl+D/Ctrl+U")),
		keymap.Bind("ctrl+u", "Scroll half a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("ctrl+f", "Scroll a page down", "Navigation", keymap.WithFooter(3, "page", false), keymap.WithFooterKey("PgUp/PgDn"), keymap.WithHelpKey("PgDn/PgUp/Ctrl+F/Ctrl+B")),
		keymap.Bind("ctrl+b", "Scroll a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("pgdown", "Scroll a page down", "Navigation", keymap.AsHidden()),
		keymap.Bind("pgup", "Scroll a page up", "Navigation", keymap.AsHidden()),
		keymap.Bind("home", "Jump to top", "Navigation", keymap.WithHelpKey("Home")),
		keymap.Bind("end", "Jump to bottom", "Navigation", keymap.WithHelpKey("End/G")),
		keymap.Bind("G", "Jump to bottom", "Navigation", keymap.AsHidden()),
		keymap.Bind("?", "Help", "System", keymap.WithFooter(4, "help", true)),
	}}
}

func (m *OutlinerModel) handleFullscreenPeekKey(key string) tea.Cmd {
	km := m.fullscreenPeekKeyMap()
	if !slices.Contains(km.HandledKeys(), key) {
		return nil
	}
	switch key {
	case "esc", "B", "b":
		m.closeFullscreenPeek()
	case "j", "down":
		m.scrollPeek(1)
	case "k", "up":
		m.scrollPeek(-1)
	case "ctrl+d":
		m.HalfPagePeek(1)
	case "ctrl+u":
		m.HalfPagePeek(-1)
	case "ctrl+f", "pgdown":
		m.PagePeek(1)
	case "ctrl+b", "pgup":
		m.PagePeek(-1)
	case "home":
		m.scrollPeek(-math.MaxInt32)
	case "end", "G":
		m.scrollPeek(math.MaxInt32)
	case "?":
		return help.HelpCmd(m.fullscreenPeekTitle(), help.HelpItemsFromKeyMap(km))
	}
	return nil
}

func fullscreenChecklistLines(task *models.Task, indent string, width int) []string {
	lines := inlinePeekChecklistLines(task, indent, width)
	for _, item := range task.Checklist {
		mark := "[ ] "
		if item.Done {
			mark = "[x] "
		}
		lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render(mark), item.Text, indent+"  ", max(width-2, 1))...)
	}
	return lines
}

func fullscreenCommentSection(task *models.Task, indent string, width int) inlinePeekSection {
	section := inlinePeekSection{key: "comments:"}
	if len(task.Comments) == 0 {
		return section
	}
	section.lines = inlinePeekCommentsLines(task, indent, width)
	for i := len(task.Comments) - 1; i >= 0; i-- {
		comment := task.Comments[i]
		if comment == nil {
			continue
		}
		at := comment.CreatedAt.Local().Format(time.DateTime)
		section.key += at + "\x1e" + comment.Text + "\x1e"
		section.lines = append(section.lines, indent+theme.MutedStyle.Render(at))
		// Preserve paragraph breaks in the full reader; only the dock needs a
		// compact summary. Every line wraps rather than losing a long tail.
		for _, line := range textfit.WrapText(comment.Text, max(width-2, 1)) {
			section.lines = append(section.lines, indent+"  "+line)
		}
	}
	return section
}

func fullscreenMetadataSection(task *models.Task, indent string, width int) inlinePeekSection {
	section := inlinePeekSection{key: "metadata:"}
	add := func(label, value string) {
		if value == "" {
			return
		}
		section.key += label + "=" + value + "\x1e"
		section.lines = append(section.lines, inlinePeekStyledLines(theme.MutedStyle.Render(label+": "), value, indent, width)...)
	}
	instant := func(label string, value time.Time) {
		if !value.IsZero() {
			add(label, value.Local().Format(time.RFC3339))
		}
	}
	add("ID", fmt.Sprintf("#%d · %s", task.ShortID, task.ID))
	if task.Hidden {
		add("Hidden", "Yes")
	}
	instant("Created", task.CreatedAt)
	instant("Updated", task.UpdatedAt)
	instant("State changed", task.StateChangedAt)
	for _, entry := range []struct {
		label string
		at    *time.Time
	}{
		{"Completed", task.CompletedAt},
		{"Pinned at", task.PinnedAt},
		{"Last reviewed", task.LastReviewedAt},
		{"Deferred until", task.DeferredUntil},
	} {
		if entry.at != nil {
			instant(entry.label, *entry.at)
		}
	}
	if task.DeferCount > 0 {
		add("Deferrals", fmt.Sprint(task.DeferCount))
	}
	if task.ChecklistOrigin != nil {
		origin := task.ChecklistOrigin
		add("Checklist origin", fmt.Sprintf("%s/%s · item %d", origin.ProjectID, origin.TaskID, origin.Index+1))
	}
	var children []string
	for _, child := range task.Children {
		if child != nil {
			children = append(children, fmt.Sprintf("#%d %s · %s", child.ShortID, child.Title, child.WorkflowState))
		}
	}
	if len(children) > 0 {
		add("Subtasks", strings.Join(children, "\n"))
	}
	return section
}

func fullscreenAttachmentLines(task *models.Task, indent string, width int) []string {
	lines := inlinePeekAttachmentsLines(task, indent, width)
	for _, attachment := range task.Attachments {
		target := attachment.OriginalPath
		if target == "" {
			target = attachment.Filename
		}
		if attachment.IsCodeRef && attachment.LineNumber > 0 {
			target += fmt.Sprintf(":%d", attachment.LineNumber)
		}
		lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render("Location: "), target, indent, width)...)
		if !attachment.AddedAt.IsZero() {
			lines = append(lines, inlinePeekStyledLines(theme.MutedStyle.Render("Added: "), attachment.AddedAt.Local().Format(time.RFC3339), indent, width)...)
		}
	}
	return lines
}
