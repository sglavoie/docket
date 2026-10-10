package conflict

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/storage"
)

func TestUnifiedDiff(t *testing.T) {
	cases := []struct{ name, before, after, want string }{
		{"edit", "a\nb\nc\n", "a\nB\nc\n", "--- base\n+++ local\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c"},
		{"add", "", "hello\n", "--- base\n+++ local\n@@ -0,0 +1 @@\n+hello"},
		{"delete", "hello\n", "", "--- base\n+++ local\n@@ -1 +0,0 @@\n-hello"},
		{"newline", "hello", "hello\n", "--- base\n+++ local\n@@ -1 +1 @@\n-hello\n\\ No newline at end of file\n+hello"},
		{"same", "same\n", "same\n", "No changes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := strings.Join(unifiedDiff(c.before, c.after, "base", "local"), "\n"); got != c.want {
				t.Fatalf("diff:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

func editableBrowser(t *testing.T) *ConflictBrowserModel {
	t.Helper()
	base, one, two := "common\n", "one\n", "two\n"
	browser, err := NewConflictBrowserModel(nil, []SyncConflictEntry{{ID: "conflict-id", Path: "note.md", Label: "Note", BaseText: &base, CurrentText: &one, Replaceable: true,
		Versions: []SyncConflictVersion{{Index: 3, Blob: "one", Text: &one}, {Index: 7, Blob: "two", Text: &two, AlsoResolves: []string{"Other note", "Planning"}}}}}, func(string) (*storage.ConflictBundle, error) { return nil, errors.New("unexpected bundle") })
	if err != nil {
		t.Fatal(err)
	}
	browser.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	return browser
}

func TestResolutionConfirmsExactVersionAndCoupledFiles(t *testing.T) {
	b := editableBrowser(t)
	b.HandleKey(syntheticKey(']'))
	b.HandleKey(syntheticKey('d'))
	if diff := strings.Join(b.detail, "\n"); !strings.Contains(diff, "-one\n+two") {
		t.Fatal(diff)
	}
	_, _, msg := b.HandleKey(syntheticKey('v'))
	if msg != nil {
		t.Fatal("selection committed without confirmation")
	}
	if text := strings.Join(b.detail, "\n"); !strings.Contains(text, "Other note") || !strings.Contains(text, "Planning") {
		t.Fatal(text)
	}
	b.HandleKey(syntheticSpecialKey(tea.KeyEscape))
	if b.confirmation != nil {
		t.Fatal("cancel retained confirmation")
	}
	b.HandleKey(syntheticKey('v'))
	_, _, msg = b.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	req, ok := msg.(ResolveMsg)
	if !ok || req.ID != "conflict-id" || !req.Sync || req.Version != 7 || req.Choice != "version" {
		t.Fatalf("request = %#v", msg)
	}
	_, _, again := b.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	if again != nil {
		t.Fatal("duplicate resolution while busy")
	}
	b.ResolutionFailed(errors.New("changed since inspected"))
	if b.busy || !strings.Contains(b.Notice(), "changed") {
		t.Fatal("failure lost")
	}
}

func TestConflictDiffComparesCurrentContentWithoutCommonBase(t *testing.T) {
	b := editableBrowser(t)
	empty := ""
	b.syncEntry.BaseText = &empty
	b.HandleKey(syntheticKey(']'))
	b.HandleKey(syntheticKey('d'))
	view := strings.Join(b.detail, "\n")
	if !strings.Contains(view, "Current → version 2") || !strings.Contains(view, "--- current/note.md") || !strings.Contains(view, "-one\n+two") {
		t.Fatalf("diff should show replacements even with no common base:\n%s", view)
	}
	b.HandleKey(syntheticKey('b'))
	view = strings.Join(b.detail, "\n")
	if !strings.Contains(view, "Base → version 2") || !strings.Contains(view, "The base is empty") || !strings.Contains(view, "+two") {
		t.Fatalf("explicit base comparison should explain whole-file additions:\n%s", view)
	}
	b.HandleKey(syntheticKey('b'))
	if view := strings.Join(b.detail, "\n"); !strings.Contains(view, "-one\n+two") {
		t.Fatalf("could not return to current comparison: %s", view)
	}
}

func TestConflictDiffStartsWithCompetingVersion(t *testing.T) {
	b := editableBrowser(t)
	b.closeDetail()
	// Rebuild the list with its first version marked as the current file.
	entry := b.syncConflicts[0]
	entry.Versions[0].Current = true
	b, err := NewConflictBrowserModel(nil, []SyncConflictEntry{entry}, b.load)
	if err != nil {
		t.Fatal(err)
	}
	b.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	b.HandleKey(syntheticKey('d'))
	if view := strings.Join(b.detail, "\n"); !strings.Contains(view, "-one\n+two") {
		t.Fatalf("initial diff compares the current file with itself: %s", view)
	}
}

func TestMergeEditorPreservesDraftThroughDiffConfirmationAndFailure(t *testing.T) {
	b := editableBrowser(t)
	b.HandleKey(syntheticKey('e'))
	pasted := "\tUnicode 界\r\n" + strings.Repeat("line\n", 150)
	b.HandlePaste(tea.PasteMsg{Content: pasted})
	want := pasted + "one\n"
	if got := string(b.editor.text); got != want {
		t.Fatal("paste truncated or normalized")
	}
	b.HandleKey(syntheticSpecialKey(tea.KeyEscape))
	b.HandleKey(syntheticKey('d'))
	b.HandleKey(syntheticKey('e'))
	if !b.editorActive() || string(b.editor.text) != want {
		t.Fatal("draft lost while comparing")
	}
	b.HandleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if b.confirmation == nil || b.confirmation.Text != want {
		t.Fatal("merge preview lost draft")
	}
	b.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	b.ResolutionFailed(errors.New("invalid merge"))
	if !b.editorActive() || string(b.editor.text) != want {
		t.Fatal("validation failure lost draft")
	}
	b.HandleKey(syntheticSpecialKey(tea.KeyEscape))
	b.HandleKey(syntheticSpecialKey(tea.KeyEscape))
	if !b.discard {
		t.Fatal("edited draft left without discard prompt")
	}
	b.HandleKey(syntheticKey('n'))
	b.HandleKey(syntheticKey('e'))
	if string(b.editor.text) != want {
		t.Fatal("discard cancellation lost draft")
	}
}

func TestConflictResolutionNarrowLayout(t *testing.T) {
	b := editableBrowser(t)
	for _, key := range []string{"", "d", "v", "esc", "e"} {
		if key != "" {
			if key == "esc" {
				b.HandleKey(syntheticSpecialKey(tea.KeyEscape))
			} else {
				b.HandleKey(syntheticKey([]rune(key)[0]))
			}
		}
		view := b.RenderWithin(60, 20)
		if lipgloss.Width(view) > 60 || lipgloss.Height(view) > 20 {
			t.Fatalf("%s: %dx%d", key, lipgloss.Width(view), lipgloss.Height(view))
		}
	}
}

func TestResolutionErrorAppearsOnce(t *testing.T) {
	for _, pane := range []string{"details", "diff", "editor"} {
		t.Run(pane, func(t *testing.T) {
			b := editableBrowser(t)
			switch pane {
			case "diff":
				b.HandleKey(syntheticKey('d'))
			case "editor":
				b.HandleKey(syntheticKey('e'))
			}
			b.ResolutionFailed(errors.New("Resolution failed: the file changed"))
			view := b.RenderWithin(100, 25)
			if got := strings.Count(view, "Resolution failed:"); got != 1 {
				t.Fatalf("error appears %d times in %s: %s", got, pane, view)
			}
			if lipgloss.Width(view) > 100 || lipgloss.Height(view) > 25 {
				t.Fatal("error overflows the modal")
			}
		})
	}
}

func TestRetryOnlyBundleRefusesLocalAndManualResolution(t *testing.T) {
	b := newTestConflictBrowser(t, conflictBrowserFixture("retry", true))
	b.HandleKey(syntheticSpecialKey(tea.KeyEnter))
	for _, key := range []rune{'l', 'e'} {
		b.HandleKey(syntheticKey(key))
		if b.confirmation != nil || b.editor != nil {
			t.Fatal("retry-only bundle offers partial apply")
		}
	}
	b.HandleKey(syntheticKey('r'))
	if b.confirmation == nil || b.confirmation.Choice != "remote" {
		t.Fatal("cannot clear retry-only bundle")
	}
}

func TestDeletionAndUnavailableContentAreExplicit(t *testing.T) {
	b := editableBrowser(t)
	b.syncEntry.Replaceable = false
	b.syncEntry.Versions[0].Blob = ""
	b.syncEntry.Versions[0].Text = nil
	b.syncEntry.Versions[0].Content = []string{"(deleted)"}
	b.HandleKey(syntheticKey('d'))
	if !strings.Contains(strings.Join(b.detail, "\n"), "Text diff unavailable") {
		t.Fatal("missing bytes presented as an ordinary diff")
	}
	b.HandleKey(syntheticKey('e'))
	if b.editor != nil {
		t.Fatal("binary content offered a text editor")
	}
	b.HandleKey(syntheticKey('v'))
	if !strings.Contains(strings.Join(b.detail, "\n"), "deletes the file") {
		t.Fatal("deletion absent from confirmation")
	}
}

func TestMergeEditorNavigationAndDeletionPreserveOtherLines(t *testing.T) {
	e := newMergeEditor("é\talpha\nbeta\nlast")
	e.key(syntheticSpecialKey(tea.KeyEnd))
	e.key(syntheticSpecialKey(tea.KeyDown))
	e.key(syntheticSpecialKey(tea.KeyBackspace))
	e.key(syntheticSpecialKey(tea.KeyHome))
	e.key(syntheticSpecialKey(tea.KeyDelete))
	e.key(syntheticSpecialKey(tea.KeyTab))
	e.key(syntheticSpecialKey(tea.KeyEnter))
	e.key(syntheticKey('界'))
	if got := string(e.text); got != "é\talpha\n\t\n界et\nlast" {
		t.Fatalf("edit: %q", got)
	}
	e.key(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	e.key(syntheticSpecialKey(tea.KeyLeft))
	e.key(syntheticSpecialKey(tea.KeyRight))
	if e.cursor != len(e.text) {
		t.Fatal("end navigation lost cursor")
	}
	e.key(tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModCtrl})
	e.key(syntheticSpecialKey(tea.KeyUp))
	e.key(syntheticSpecialKey(tea.KeyBackspace))
	if e.cursor != 0 {
		t.Fatal("cursor escaped start")
	}
	e.insert(strings.Repeat("界", 200))
	rendered := e.render(40, 8)
	if lipgloss.Width(rendered) > 40 || lipgloss.Height(rendered) > 8 {
		t.Fatalf("long Unicode line overflowed: %dx%d", lipgloss.Width(rendered), lipgloss.Height(rendered))
	}
}
