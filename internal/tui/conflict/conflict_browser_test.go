package conflict

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
)

// conflictBrowserFixture is one recorded bundle with both a field edit carrying
// three diverging values and a hierarchy conflict carrying only a detail line,
// so the detail pane's two shapes are both exercised.
func conflictBrowserFixture(id string, retryRequired bool) storage.ConflictRecord {
	return storage.ConflictRecord{
		ID:                     id,
		ProjectName:            "Rich Project",
		ProjectSlug:            "rich-project",
		Action:                 "update task",
		Context:                "renaming a task while the file moved",
		BaseRevision:           "b0b0b0b0b0b0b0b0b0",
		LocalRevision:          "1oca11oca11oca11o",
		RemoteRevision:         "",
		RequiresOperationRetry: &retryRequired,
		CreatedAt:              time.Date(2026, 3, 4, 9, 30, 15, 0, time.UTC),
		Conflicts: []storage.MergeConflict{
			{
				Kind:      storage.MergeConflictFieldEdit,
				TaskTitle: "Ship the release notes",
				Field:     "title",
				Base:      "Draft the release notes",
				Local:     "Ship the release notes",
				Current:   "Publish the release notes",
			},
			{
				Kind:   storage.MergeConflictHierarchy,
				Detail: "both sides reparented this task",
			},
		},
	}
}

func newTestConflictBrowser(t *testing.T, records ...storage.ConflictRecord) *ConflictBrowserModel {
	t.Helper()
	browser, err := NewConflictBrowserModel(records, nil, func(id string) (*storage.ConflictBundle, error) {
		for _, record := range records {
			if record.ID == id {
				return &storage.ConflictBundle{Record: record}, nil
			}
		}
		return nil, fmt.Errorf("conflict %s not found", id)
	})
	if err != nil {
		t.Fatalf("build conflict browser: %v", err)
	}
	return browser
}

func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// pressConflictBrowserKey drives the overlay through the same contract the shell
// uses, `HandleKey`, so a test presses a key rather than calling the handler the
// key happens to reach.
func pressConflictBrowserKey(
	t *testing.T,
	browser *ConflictBrowserModel,
	key tea.KeyPressMsg,
) (bool, tea.Msg) {
	t.Helper()
	_, dismissed, msg := browser.HandleKey(key)
	return dismissed, msg
}

// TestConflictDetailLinesRenderTheCLIRecord holds the detail pane to `tt conflict
// show`'s field set and, above all, to its closing command: a retry-only bundle
// may only be cleared by adopting remote, and saying otherwise would invite a
// merge decision the store refuses.
func TestConflictDetailLinesRenderTheCLIRecord(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	body := strings.Join(conflictDetailLines(record), "\n")

	for _, want := range []string{
		"Conflict 11111111-2222-3333-4444-555555555555",
		"Project:   Rich Project (rich-project)",
		"Action:    update task",
		"Context:   renaming a task while the file moved",
		"Recorded:  ",
		"Revisions: base b0b0b0b0b0b0 · local 1oca11oca11o · remote <none>",
		"Retry:     required",
		"2 conflicts:",
		`[1] field_edit — "Ship the release notes"`,
		"field:   title",
		"base:    Draft the release notes",
		"local:   Ship the release notes",
		"remote:  Publish the release notes",
		"[2] hierarchy",
		"detail:  both sides reparented this task",
		"Clear with: tt conflict resolve 11111111 --use remote",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("detail pane missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "Resolve with:") {
		t.Errorf("retry-only bundle offered a side to adopt\n%s", body)
	}
}

// TestConflictDetailLinesOfferEverySideWhenApplying is the other half of that
// rule: a bundle the resolver may apply names all three sides.
func TestConflictDetailLinesOfferEverySideWhenApplying(t *testing.T) {
	record := conflictBrowserFixture("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", false)
	body := strings.Join(conflictDetailLines(record), "\n")

	if want := "Resolve with: tt conflict resolve aaaaaaaa --use local|remote|file"; !strings.Contains(body, want) {
		t.Errorf("detail pane missing %q\n%s", want, body)
	}
	if strings.Contains(body, "Retry:") {
		t.Errorf("applying bundle claimed a retry requirement\n%s", body)
	}
}

// TestConflictBrowserListsInspectsAndUnwinds walks the whole overlay: the list
// names every bundle and says it cannot resolve them, Enter reads one, and Esc
// unwinds one level at a time.
func TestConflictBrowserListsInspectsAndUnwinds(t *testing.T) {
	first := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	second := conflictBrowserFixture("99999999-8888-7777-6666-555555555555", false)
	second.ProjectName = "Second Project"
	browser := newTestConflictBrowser(t, first, second)

	list := browser.Render(120)
	for _, want := range []string{
		conflictBrowserTitle,
		conflictBrowserReadOnlyLine,
		"11111111",
		"Rich Project",
		"99999999",
		"Second Project",
		"2 conflicts",
	} {
		if !strings.Contains(list, want) {
			t.Errorf("conflict list missing %q\n%s", want, list)
		}
	}

	if dismissed, _ := pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyEnter)); dismissed {
		t.Fatal("Enter dismissed the browser; want the detail pane")
	}
	if !browser.inspecting() {
		t.Fatal("Enter did not open the detail pane")
	}
	detail := browser.Render(120)
	if !strings.Contains(detail, "Conflict 11111111-2222-3333-4444-555555555555") {
		t.Errorf("detail pane did not render the inspected record\n%s", detail)
	}
	if strings.Contains(detail, conflictBrowserReadOnlyLine) {
		t.Errorf("detail pane kept the list's header line\n%s", detail)
	}

	if dismissed, _ := pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyEscape)); dismissed {
		t.Fatal("Esc closed the browser from the detail pane; want a return to the list")
	}
	if browser.inspecting() {
		t.Fatal("Esc did not leave the detail pane")
	}

	dismissed, msg := pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyEscape))
	if !dismissed {
		t.Fatal("Esc did not close the browser from the list")
	}
	if _, ok := msg.(ConflictBrowserClosedMsg); !ok {
		t.Fatalf("close message = %T, want ConflictBrowserClosedMsg", msg)
	}
}

// TestConflictBrowserScrollsTheDetailPane covers the one piece of state the pane
// owns. The closing command sits below the first window, so reaching it is the
// honest assertion that the pane scrolls at all.
func TestConflictBrowserScrollsTheDetailPane(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	browser := newTestConflictBrowser(t, record)

	pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyEnter))
	// Render inside a short terminal so the pane is genuinely windowed.
	browser.RenderWithin(120, 20)
	if browser.detailHeight <= 0 {
		t.Fatal("detail pane recorded no height")
	}
	if browser.detailLines <= browser.detailHeight {
		t.Fatalf("detail pane fits in one window (%d lines, %d rows); nothing to scroll",
			browser.detailLines, browser.detailHeight)
	}

	pressConflictBrowserKey(t, browser, syntheticKey('G'))
	tail := browser.RenderWithin(120, 20)
	if !strings.Contains(tail, "Clear with: tt conflict resolve") {
		t.Errorf("end of the detail pane did not reach the closing command\n%s", tail)
	}

	pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyHome))
	if browser.detailOffset != 0 {
		t.Errorf("Home left the pane at offset %d, want 0", browser.detailOffset)
	}
	pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyUp))
	if browser.detailOffset != 0 {
		t.Errorf("scrolling above the top left offset %d, want 0", browser.detailOffset)
	}
}

// TestConflictBrowserRefusesAResolvedBundle is why inspecting reloads rather
// than reusing the listing: the CLI may have resolved the conflict in another
// terminal since the list was taken, and the row must say so instead of painting
// a record that is gone.
func TestConflictBrowserRefusesAResolvedBundle(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	browser, err := NewConflictBrowserModel(
		[]storage.ConflictRecord{record},
		nil,
		func(string) (*storage.ConflictBundle, error) {
			return nil, fmt.Errorf("conflict %s not found", record.ID)
		},
	)
	if err != nil {
		t.Fatalf("build conflict browser: %v", err)
	}

	dismissed, msg := pressConflictBrowserKey(t, browser, syntheticSpecialKey(tea.KeyEnter))
	if dismissed {
		t.Fatal("a refused inspect closed the browser")
	}
	if browser.inspecting() {
		t.Fatal("a refused inspect opened the detail pane")
	}
	status, ok := msg.(status.StatusMsg)
	if !ok || !strings.Contains(string(status), "not found") {
		t.Fatalf("refusal message = %#v, want a StatusMsg naming the missing bundle", msg)
	}
	if !strings.Contains(browser.Render(120), "not found") {
		t.Error("the refusal was not retained on the overlay")
	}
}

// TestConflictBrowserRequiresALoader keeps the one construction rule: a browser
// that cannot reload a bundle would silently show stale records.
func TestConflictBrowserRequiresALoader(t *testing.T) {
	if _, err := NewConflictBrowserModel(nil, nil, nil); err == nil {
		t.Fatal("a browser without a loader was accepted")
	}
}

// TestConflictBrowserListsSyncConflicts lists an enrolled workspace's sync
// conflicts after the bundles and inspects one without a reload: every
// competing version, who wrote it, and the version the workspace holds.
func TestConflictBrowserListsSyncConflicts(t *testing.T) {
	bundle := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", false)
	entry := SyncConflictEntry{
		ID:   "sha256:abcdef0123456789",
		Path: "projects/unfiled/tasks.md",
		Kind: "project_document",
		Versions: []SyncConflictVersion{
			{Blob: "sha256:1111111111111111", Writers: []string{"sha256:aaaaaaaaaaaaaaaa"}},
			{Blob: "sha256:2222222222222222", Writers: []string{"sha256:bbbbbbbbbbbbbbbb"}},
		},
		Result:  "sha256:1111111111111111",
		Details: []string{"task title: competing edits"},
	}
	loads := 0
	browser, err := NewConflictBrowserModel([]storage.ConflictRecord{bundle}, []SyncConflictEntry{entry},
		func(string) (*storage.ConflictBundle, error) {
			loads++
			return &storage.ConflictBundle{Record: bundle}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	list := browser.RenderWithin(120, 40)
	for _, want := range []string{"projects/unfiled/tasks.md", "sync conflict · 2 versions", conflictBrowserReadOnlySyncLine[:20]} {
		if !strings.Contains(list, want) {
			t.Fatalf("list does not show %q:\n%s", want, list)
		}
	}
	browser.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if _, isBundle := browser.CurrentRecord(); isBundle {
		t.Fatal("the sync row reads as a bundle")
	}
	browser.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	detail := strings.Join(browser.detail, "\n")
	for _, want := range []string{"Sync conflict abcdef0123456789", "[1] content 11111111", "[2] content 22222222",
		"written by bbbbbbbb", "Holding:   content 11111111", "task title: competing edits", "nothing was lost"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail does not show %q:\n%s", want, detail)
		}
	}
	if loads != 0 {
		t.Fatalf("inspecting a sync conflict reloaded %d bundles", loads)
	}
}
