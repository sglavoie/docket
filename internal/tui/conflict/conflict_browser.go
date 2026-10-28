// Package conflict provides the merge-conflict browser and resolution editor.
package conflict

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/util"
)

const (
	conflictBrowserItemPrefix = "conflict-browser:"
	conflictBrowserTitle      = "Merge Conflicts"
	conflictBrowserIntro      = "Enter to inspect, compare changes, and resolve"
	// conflictBrowserShortIDWidth matches the CLI's own shortening, so an id read
	// off this list can be typed straight into `docket conflict resolve`.
	conflictBrowserShortIDWidth = 8
	// conflictBrowserTimeFormat is the CLI's recorded-at format. The list drops
	// the seconds the detail pane keeps, because a row is a locator and the pane
	// is the record.
	conflictBrowserTimeFormat    = "2006-01-02 15:04:05"
	conflictBrowserRowTimeFormat = "2006-01-02 15:04"
	// ConflictBrowserEmptyStatus is the refusal the shell sets when `gc` finds an
	// empty store. It is exported because the host decides not to mount the
	// overlay at all in that case, so the line has to be said from outside the
	// surface that owns it.
	ConflictBrowserEmptyStatus = "No unresolved merge conflicts"

	conflictBrowserEmptyTitle     = "No unresolved merge conflicts"
	conflictBrowserEmptyDetail    = "A conflict is recorded when a project changed under a mutation."
	conflictBrowserMissingRecord  = "That row is not a recorded conflict"
	conflictBrowserRevisionAbsent = "<none>"
	// The list is capped at the lens picker's height because both are locators.
	// The detail pane is capped higher because it is the record itself, and a
	// bundle with several structured conflicts is long enough that a taller
	// terminal should spend its rows on it.
	conflictBrowserMaxListRows   = 14
	conflictBrowserMaxDetailRows = 24
	conflictBrowserMinBodyRows   = 4
)

// ConflictBrowserClosedMsg dismisses the browser.
type ConflictBrowserClosedMsg struct{}

// ConflictBrowserModel lists the recorded conflict bundles and inspects one of
// them. The loader is injected rather than a store held directly, so the model
// stays a projection over whatever the host read and the detail pane can be
// driven from a fixture in a test.
type ConflictBrowserModel struct {
	surface *listsurface.ListSurfaceModel
	records []storage.ConflictRecord
	// syncConflicts are the open sync conflicts, listed after the bundles. Each
	// is derived from the sync history, which retains every competing version,
	// so a row carries its whole record and inspecting it reloads nothing.
	syncConflicts []SyncConflictEntry
	load          func(id string) (*storage.ConflictBundle, error)
	// detail is the inspected bundle's rendered lines, unwrapped. A non-empty
	// slice is what puts the overlay in its detail state; Esc empties it.
	detail       []string
	detailOffset int
	// detailLines and detailHeight are the last render's wrapped line count and
	// window height, which is what lets a scroll key clamp against what is
	// actually on screen rather than against the unwrapped source.
	detailLines  int
	detailHeight int
	notice       string
	bundle       *storage.ConflictBundle
	syncEntry    *SyncConflictEntry
	version      int
	diff         bool
	diffBase     bool
	confirmation *ResolveMsg
	busy         bool
	discard      bool
	editor       *mergeEditor
	// listTop is the popup row the last render drew the list's first row on, or
	// -1 when it drew the detail pane instead. See HandlePointer.
	listTop int
}

// NewConflictBrowserModel builds the browser over one listing. The loader is
// required: the list carries enough metadata to draw a row, but inspecting
// re-reads the bundle so a conflict the CLI resolved in another terminal since
// the list was taken refuses rather than showing a record that is gone.
//
// An enrolled workspace has open sync conflicts beside its bundles; they are
// listed after them.
func NewConflictBrowserModel(
	records []storage.ConflictRecord,
	syncConflicts []SyncConflictEntry,
	load func(id string) (*storage.ConflictBundle, error),
) (*ConflictBrowserModel, error) {
	if load == nil {
		return nil, fmt.Errorf("the conflict browser requires a conflict loader")
	}
	model := &ConflictBrowserModel{
		records:       append([]storage.ConflictRecord(nil), records...),
		syncConflicts: append([]SyncConflictEntry(nil), syncConflicts...),
		load:          load,
	}
	surface, err := listsurface.NewListSurfaceModel(model.items(), nil, listsurface.ListSurfaceConfig{
		SelectionMode: listsurface.ListSelectionSingle,
		EmptyState: listsurface.ListEmptyState{
			Title:  conflictBrowserEmptyTitle,
			Detail: conflictBrowserEmptyDetail,
		},
	})
	if err != nil {
		return nil, err
	}
	model.surface = surface
	return model, nil
}

// items renders one row per recorded bundle, in the store's own newest-first
// order. There is no query and no ranking: the population is small and its order
// is the only thing that makes "the one that just happened" findable.
func (model *ConflictBrowserModel) items() []listsurface.ListItem {
	items := make([]listsurface.ListItem, 0, len(model.records))
	for _, record := range model.records {
		items = append(items, listsurface.ListItem{
			ID:            conflictBrowserItemID(record.ID),
			PrimaryText:   conflictBrowserRowTitle(record),
			SecondaryText: conflictBrowserRowSummary(record),
			Data:          record,
		})
	}
	for _, entry := range model.syncConflicts {
		items = append(items, listsurface.ListItem{
			ID:            conflictBrowserItemID("sync:" + entry.ID),
			PrimaryText:   syncConflictRowTitle(entry),
			SecondaryText: syncConflictRowSummary(entry),
			Data:          entry,
		})
	}
	return items
}

func conflictBrowserItemID(id string) listsurface.ListItemID {
	return listsurface.ListItemID(conflictBrowserItemPrefix + id)
}

// conflictBrowserShortID is the CLI's own id shortening, which `docket conflict
// resolve` accepts back as a prefix.
func conflictBrowserShortID(id string) string {
	if len(id) <= conflictBrowserShortIDWidth {
		return id
	}
	return id[:conflictBrowserShortIDWidth]
}

// conflictShortRevision trims a hex revision the way `docket conflict show` does,
// naming an absent one rather than printing nothing.
func conflictShortRevision(revision string) string {
	if revision == "" {
		return conflictBrowserRevisionAbsent
	}
	if len(revision) > 12 {
		return revision[:12]
	}
	return revision
}

// conflictBrowserRowTitle identifies the bundle and its project.
func conflictBrowserRowTitle(record storage.ConflictRecord) string {
	name := strings.TrimSpace(record.ProjectName)
	if name == "" {
		name = record.ProjectSlug
	}
	if name == "" {
		return conflictBrowserShortID(record.ID)
	}
	return conflictBrowserShortID(record.ID) + "  " + name
}

// conflictBrowserRowSummary carries the remaining `docket conflict list` columns:
// how many structured conflicts the bundle holds, what mutation recorded it, and
// when.
func conflictBrowserRowSummary(record storage.ConflictRecord) string {
	parts := make([]string, 0, 3)
	if count := len(record.Conflicts); count == 1 {
		parts = append(parts, "1 conflict")
	} else {
		parts = append(parts, fmt.Sprintf("%d conflicts", count))
	}
	if action := strings.TrimSpace(record.Action); action != "" {
		parts = append(parts, action)
	}
	parts = append(parts, record.CreatedAt.Local().Format(conflictBrowserRowTimeFormat))
	return strings.Join(parts, " · ")
}

// conflictDetailLines describes the recorded inputs and available resolutions.
func conflictDetailLines(record storage.ConflictRecord) []string {
	lines := []string{
		"Conflict " + record.ID,
		"  Project:   " + record.ProjectName + " (" + record.ProjectSlug + ")",
	}
	if record.Action != "" {
		lines = append(lines, "  Action:    "+record.Action)
	}
	if record.Context != "" {
		lines = append(lines, "  Context:   "+record.Context)
	}
	lines = append(lines, "  Recorded:  "+record.CreatedAt.Local().Format(conflictBrowserTimeFormat))
	lines = append(lines, fmt.Sprintf("  Revisions: base %s · local %s · remote %s",
		conflictShortRevision(record.BaseRevision),
		conflictShortRevision(record.LocalRevision),
		conflictShortRevision(record.RemoteRevision),
	))
	if record.OperationRetryRequired() {
		lines = append(lines,
			"  Retry:     required — adopt remote to clear this record, then retry the original operation")
	}
	lines = append(lines, "", util.Count(len(record.Conflicts), "conflict")+":")
	for i, conflict := range record.Conflicts {
		header := fmt.Sprintf("  [%d] %s", i+1, conflict.Kind)
		if conflict.TaskTitle != "" {
			header += fmt.Sprintf(" — %q", conflict.TaskTitle)
		}
		lines = append(lines, "", header)
		if conflict.Field != "" {
			lines = append(lines, "      field:   "+conflict.Field)
		}
		if conflict.Base != "" || conflict.Local != "" || conflict.Current != "" {
			lines = append(lines,
				"      base:    "+conflict.Base,
				"      local:   "+conflict.Local,
				"      remote:  "+conflict.Current,
			)
		}
		if conflict.Detail != "" {
			lines = append(lines, "      detail:  "+conflict.Detail)
		}
	}
	return append(lines, "", conflictResolveHint(record))
}

// conflictResolveHint offers only resolutions the transaction policy permits.
func conflictResolveHint(record storage.ConflictRecord) string {
	if record.OperationRetryRequired() {
		return "r keep current state and clear; retry the original operation"
	}
	return "l keep local · r keep current remote · e edit merged result"
}

// inspecting reports whether the detail pane owns the overlay. The rendered
// lines are the state, so there is no second flag to keep in step with them.
func (model *ConflictBrowserModel) inspecting() bool {
	return model != nil && len(model.detail) > 0
}

// CurrentRecord returns the conflict under the cursor.
func (model *ConflictBrowserModel) CurrentRecord() (storage.ConflictRecord, bool) {
	if model == nil || model.surface == nil {
		return storage.ConflictRecord{}, false
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return storage.ConflictRecord{}, false
	}
	record, ok := item.Data.(storage.ConflictRecord)
	return record, ok
}

// Notice returns the retained refusal, which stays on the overlay so a bundle
// that vanished between the listing and the Enter says so on the row that named
// it.
func (model *ConflictBrowserModel) Notice() string {
	if model == nil {
		return ""
	}
	return model.notice
}

// HandleKey implements the shell's compact key-interceptor contract. The two
// states are dispatched separately rather than through one switch with guards,
// because they share no key whose meaning survives the state change.
func (model *ConflictBrowserModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, ConflictBrowserClosedMsg{}
	}
	if model.inspecting() {
		if model.busy {
			return nil, false, nil
		}
		if model.confirmation != nil {
			return model.handleConfirmation(msg.String())
		}
		if model.discard {
			switch msg.String() {
			case "y":
				model.closeDetail()
				model.discard = false
			case "n", "esc":
				model.discard = false
				model.rebuildDetail()
			}
			return nil, false, nil
		}
		if model.editorActive() {
			return model.handleEditorKey(msg)
		}
		return model.handleDetailKey(msg.String())
	}
	return model.handleListKey(msg.String())
}

func (model *ConflictBrowserModel) handleListKey(key string) (tea.Cmd, bool, tea.Msg) {
	switch key {
	case "esc":
		return nil, true, ConflictBrowserClosedMsg{}
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "enter":
		return model.inspectCursor()
	}
	return nil, false, nil
}

// handleDetailKey scrolls the pane. Esc returns to the list rather than closing
// the overlay, so unwinding an inspected conflict costs the same two presses
// entering it did.
func (model *ConflictBrowserModel) handleDetailKey(key string) (tea.Cmd, bool, tea.Msg) {
	page := max(model.detailHeight-1, 1)
	switch key {
	case "d":
		model.diff = !model.diff
		model.rebuildDetail()
	case "b":
		model.diffBase = !model.diffBase
		model.diff = true
		model.rebuildDetail()
	case "[", "]":
		if model.syncEntry != nil && len(model.syncEntry.Versions) > 0 {
			delta := 1
			if key == "[" {
				delta = -1
			}
			model.version = (model.version + delta + len(model.syncEntry.Versions)) % len(model.syncEntry.Versions)
			model.rebuildDetail()
		}
	case "l", "r", "v", "c":
		model.chooseResolution(key)
	case "e":
		model.openEditor()
	case "esc":
		if model.hasDraft() {
			model.discard = true
			model.detail = []string{"Discard the edited merge?", "y discard · n/Esc keep draft"}
			model.detailOffset = 0
		} else {
			model.closeDetail()
		}
	case "up", "k", "ctrl+p":
		model.scrollDetail(-1)
	case "down", "j", "ctrl+n":
		model.scrollDetail(1)
	case "pgup":
		model.scrollDetail(-page)
	case "pgdown":
		model.scrollDetail(page)
	case "home", "g":
		model.detailOffset = 0
	case "end", "G":
		model.scrollDetail(model.detailLines)
	}
	return nil, false, nil
}

func (model *ConflictBrowserModel) closeDetail() {
	model.bundle, model.syncEntry = nil, nil
	model.editor, model.confirmation = nil, nil
	model.diff = false
	model.diffBase = false
	model.detail = nil
	model.detailOffset = 0
	model.detailLines = 0
	model.detailHeight = 0
	model.notice = ""
}

// scrollDetail moves the pane's window and clamps it against the last render's
// measurements, which is the only place the wrapped line count is known.
func (model *ConflictBrowserModel) scrollDetail(delta int) {
	maxOffset := max(model.detailLines-model.detailHeight, 0)
	model.detailOffset = min(max(model.detailOffset+delta, 0), maxOffset)
}

// inspectCursor re-reads the bundle under the cursor. The reload is the point:
// the listing is a snapshot, and a conflict the CLI resolved since it was taken
// must refuse here rather than paint a record that no longer exists.
func (model *ConflictBrowserModel) inspectCursor() (tea.Cmd, bool, tea.Msg) {
	if item, ok := model.surface.CurrentItem(); ok {
		if entry, isSync := item.Data.(SyncConflictEntry); isSync {
			model.notice = ""
			model.syncEntry = &entry
			model.version = 0
			for i, version := range entry.Versions {
				if !version.Current {
					model.version = i
					break
				}
			}
			model.rebuildDetail()
			model.detailOffset = 0
			return nil, false, nil
		}
	}
	record, ok := model.CurrentRecord()
	if !ok {
		model.notice = conflictBrowserMissingRecord
		return nil, false, status.StatusMsg(model.notice)
	}
	bundle, err := model.load(record.ID)
	if err != nil {
		model.notice = err.Error()
		return nil, false, status.StatusMsg(model.notice)
	}
	model.notice = ""
	model.bundle = bundle
	model.rebuildDetail()
	model.detailOffset = 0
	return nil, false, nil
}
