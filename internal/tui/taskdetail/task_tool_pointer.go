package taskdetail

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/route"
)

// The Comments and Attachments panels take the list surface's pointer answer
// over their own entry lists. Both are drawn from the screen's top-left cell,
// so the rows their last list frame recorded are screen rows. A click anywhere
// on an entry's rows seats the cursor on it, as j/k would, a double-click is
// Enter on it, and a right-click is Space, the entry's menu. The wheel steps
// the cursor a row. Every other sub-state owns its keys outright — the
// composers, the expanded reader, the add input, a confirmation — and so
// swallows the pointer.

// HandlePointer answers the pointer while the comment list is what is drawn.
func (m *CommentsModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil || m.owner == nil || m.ExpandedView || m.Confirm.IsActive() || m.EditMode || m.AddMode {
		return nil
	}
	m.pendingTopChord = false
	return route.PressKey(answerEntryPointer(m.pointerEntries, &m.Cursor, len(m.entries), event))
}

// HandlePointer answers the pointer while the attachment list is what is drawn.
func (m *TaskAttachmentsModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if m == nil || m.Task == nil || m.AddMode || m.Confirm.IsActive() {
		return nil
	}
	m.pendingTopChord = false
	return route.PressKey(answerEntryPointer(m.pointerEntries, &m.Cursor, len(m.Task.Attachments), event))
}

// answerEntryPointer seats cursor on the entry frame drew under the event, and
// returns the key the event then presses.
func answerEntryPointer(frame listsurface.ListPointerFrame, cursor *int, total int, event route.PointerEvent) string {
	seated := false
	if !event.IsWheel() {
		if index, ok := frame.IndexAt(event.X, event.Y); ok && index < total {
			*cursor = index
			seated = true
		}
	}
	return listsurface.ListPointerKey(event, seated, true)
}

// PointerTarget is the comment the cursor names: its identity where the owner
// keeps one, else its position and creation instant.
func (m *CommentsModel) PointerTarget() string {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.entries) {
		return ""
	}
	entry := m.entries[m.Cursor]
	if entry.ID != "" {
		return entry.ID
	}
	return fmt.Sprintf("%d %s", m.Cursor, entry.CreatedAt.Format(time.RFC3339Nano))
}

// PointerTarget is the attachment the cursor names.
func (m *TaskAttachmentsModel) PointerTarget() string {
	if m == nil || m.Task == nil || m.Cursor < 0 || m.Cursor >= len(m.Task.Attachments) {
		return ""
	}
	attachment := m.Task.Attachments[m.Cursor]
	return fmt.Sprintf("%d %s %s", m.Cursor, attachment.Filename, attachment.AddedAt.Format(time.RFC3339Nano))
}

var (
	_ route.PointerView       = (*CommentsModel)(nil)
	_ route.PointerView       = (*TaskAttachmentsModel)(nil)
	_ route.PointerTargetView = (*CommentsModel)(nil)
	_ route.PointerTargetView = (*TaskAttachmentsModel)(nil)
)
