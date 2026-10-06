// Package conflict is the read-only merge-conflict browser the `gc` chord
// raises. It holds the overlay alone: reading the store and mounting it are the
// shell's, in conflict_browser_host.go, and the status strip's outstanding count
// is the outliner's, in conflict_indicator.go. That split is why this package is
// a sibling of `outliner` rather than a dependency of it — the shell owns the
// edge, and the strip's line never imports the browser.
package conflict

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// The conflict browser is the read-only companion to the status strip's
// outstanding-conflict count: the strip says how many bundles `.tt/conflicts/`
// holds, and this overlay says what is in them. Like the lens picker it is a
// transient overlay rather than a routed view — no PresentationID, no
// InputStrategy, no manifest entry — but it goes one step further and registers
// no operations of its own, because it offers no act to register. Enter reads,
// Esc unwinds, and nothing on this surface writes.
//
// Resolution deliberately stays in the CLI. Adopting one side of a three-way
// merge is a destructive decision the transaction model already expresses in
// `tt conflict resolve`, so the overlay names that command rather than
// duplicating it: every detail pane ends with the exact line to run.

const (
	conflictBrowserItemPrefix = "conflict-browser:"
	conflictBrowserTitle      = "Merge Conflicts"
	// conflictBrowserReadOnlyLine heads the list so the one thing this surface
	// cannot do is stated before the rows rather than discovered by pressing keys
	// that do nothing.
	conflictBrowserReadOnlyLine = "Read-only — resolve with tt conflict resolve"
	// conflictBrowserReadOnlySyncLine replaces it when sync conflicts are listed,
	// which that command does not resolve: their every version is kept, and
	// nothing on this surface writes.
	conflictBrowserReadOnlySyncLine = "Read-only — resolve bundles with tt conflict resolve; sync conflicts keep every version"
	// conflictBrowserShortIDWidth matches the CLI's own shortening, so an id read
	// off this list can be typed straight into `tt conflict resolve`.
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

// ConflictBrowserClosedMsg dismisses the browser. It is the only message the
// overlay raises, because reading a bundle changes nothing the shell has to
// apply.
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

// conflictBrowserShortID is the CLI's own id shortening, which `tt conflict
// resolve` accepts back as a prefix.
func conflictBrowserShortID(id string) string {
	if len(id) <= conflictBrowserShortIDWidth {
		return id
	}
	return id[:conflictBrowserShortIDWidth]
}

// conflictShortRevision trims a hex revision the way `tt conflict show` does,
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

// conflictBrowserRowTitle leads with the short id, because that is the token the
// CLI command underneath this list takes.
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

// conflictBrowserRowSummary carries the remaining `tt conflict list` columns:
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

// conflictDetailLines is the detail pane, and it is `tt conflict show`'s output
// as a slice: the same fields, in the same order, with the same omissions and
// the same closing command. Keeping it a pure function of the record is what
// lets the two surfaces be compared without running either one.
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

// conflictResolveHint is the closing line of both this pane and `tt conflict
// show`: a retry-only bundle can only be cleared by adopting remote, and an
// applying one offers all three sides.
func conflictResolveHint(record storage.ConflictRecord) string {
	short := conflictBrowserShortID(record.ID)
	if record.OperationRetryRequired() {
		return "Clear with: tt conflict resolve " + short + " --use remote"
	}
	return "Resolve with: tt conflict resolve " + short + " --use local|remote|file"
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
	case "esc":
		model.closeDetail()
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
			model.detail = syncConflictDetailLines(entry)
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
	model.detail = conflictDetailLines(bundle.Record)
	model.detailOffset = 0
	return nil, false, nil
}

// conflictBrowserInnerWidth is the overlay box width the browser claims, matching
// the lens picker's so the two overlays sit in the same frame at every terminal
// size.
func conflictBrowserInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (model *ConflictBrowserModel) Render(width int) string {
	return model.RenderWithin(width, 0)
}

// RenderWithin budgets the body after reserving rows for the modal chrome, so a
// minimum-supported terminal drops list rows or detail lines rather than losing
// the title, the header block, or the footer.
func (model *ConflictBrowserModel) RenderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := conflictBrowserInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	header := make([]string, 0, 2)
	if model.notice != "" {
		header = append(header, theme.ErrorStatusStyle.Render(
			textfit.TruncateToVisualWidth(model.notice, contentWidth),
		))
	}
	if !model.inspecting() {
		line := conflictBrowserReadOnlyLine
		if len(model.syncConflicts) > 0 {
			line = conflictBrowserReadOnlySyncLine
		}
		for _, wrapped := range textfit.WrapText(line, contentWidth) {
			header = append(header, theme.MutedStyle.Render(wrapped))
		}
	}

	// Halo, frame, title, the header block's separating row, and the footer
	// consume nine rows before any header line of its own.
	bodyHeight := 0
	if height > 0 {
		bodyHeight = max(height-(9+len(header)), 1)
	}

	content := model.renderList(contentWidth, bodyHeight)
	if model.inspecting() {
		content = model.renderDetail(contentWidth, bodyHeight)
	}
	if len(header) > 0 {
		content = strings.Join(header, "\n") + "\n\n" + content
	}

	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(conflictBrowserTitle)
	return chrome.RenderOverlayBox(
		content,
		innerWidth,
		title,
		chrome.RenderGuidanceText(model.guidanceSpec(), max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

func (model *ConflictBrowserModel) renderList(width, bodyHeight int) string {
	listHeight := min(max(len(model.surface.Rows()), conflictBrowserMinBodyRows), conflictBrowserMaxListRows)
	if bodyHeight > 0 {
		listHeight = min(listHeight, bodyHeight)
	}
	return listsurface.RenderCompactListSurface(model.surface, width, listHeight)
}

// renderDetail wraps the pane at the box's width and paints one window of it.
// The wrapped count and the window height are recorded here because this is the
// only place either is known, and the scroll keys clamp against them.
func (model *ConflictBrowserModel) renderDetail(width, bodyHeight int) string {
	wrapped := make([]string, 0, len(model.detail))
	for _, line := range model.detail {
		if strings.TrimSpace(line) == "" {
			wrapped = append(wrapped, "")
			continue
		}
		wrapped = append(wrapped, textfit.WrapText(line, width)...)
	}
	paneHeight := min(max(len(wrapped), conflictBrowserMinBodyRows), conflictBrowserMaxDetailRows)
	if bodyHeight > 0 {
		paneHeight = min(paneHeight, bodyHeight)
	}
	model.detailLines = len(wrapped)
	model.detailHeight = paneHeight
	model.detailOffset = min(model.detailOffset, max(len(wrapped)-paneHeight, 0))

	window := wrapped[model.detailOffset:min(model.detailOffset+paneHeight, len(wrapped))]
	return strings.Join(window, "\n")
}

// guidanceSpec returns the footer spec alone: this overlay always draws a
// footer, so there is no "no guidance" case for a caller to branch on. Both
// states reuse the same three bindings under different labels, because the keys
// genuinely do not change — only what they move.
func (model *ConflictBrowserModel) guidanceSpec() chrome.GuidanceSpec {
	if model == nil || model.surface == nil {
		return chrome.GuidanceSpec{}
	}
	inspecting := model.inspecting()
	moveLabel := "navigate"
	escapeLabel := "close"
	if inspecting {
		moveLabel = "scroll"
		escapeLabel = "back"
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move through the conflicts", "Navigation",
			keymap.WithFooter(10, moveLabel, false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("enter", "Inspect the conflict", "Actions",
			keymap.WithGate(func() bool { return !inspecting }),
			keymap.WithFooter(1, "inspect", true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind the conflict browser", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	spec, _ := listsurface.ListSurfaceGuidanceSpec(model.surface.GuidanceState(false), km, nil)
	return spec
}
