package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/conflict"
)

func conflictKey(t *testing.T, m *AppModel, key string) tea.Msg {
	t.Helper()
	msg := tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	if key == "enter" {
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	_, _, out := m.conflictBrowser.HandleKey(msg)
	return out
}

func TestConflictBrowserResolvesSyncWithoutCLI(t *testing.T) {
	for _, choice := range []string{"version", "current", "replace", "stale"} {
		t.Run(choice, func(t *testing.T) {
			pair := newEnrolledPair(t)
			retitleOnDisk(t, pair.desk, "Desk title")
			retitleOnDisk(t, pair.laptop, "Laptop title")
			exchangeOnce(t, pair.desk)
			exchangeOnce(t, pair.laptop)
			exchangeOnce(t, pair.desk)
			views, err := pair.desk.SyncConflicts(nil)
			if err != nil || len(views) != 1 {
				t.Fatalf("conflicts: %v, %d", err, len(views))
			}
			view := views[0]
			m, err := NewAppModel(pair.desk)
			if err != nil {
				t.Fatal(err)
			}
			m.openConflictBrowser()
			conflictKey(t, m, "enter")
			var req conflict.ResolveMsg
			expected := ""
			switch choice {
			case "current":
				expected = *view.Current.Text
				conflictKey(t, m, "c")
			case "version", "stale":
				for _, version := range view.Versions {
					if !version.Current {
						expected = *version.Text
						break
					}
				}
				conflictKey(t, m, "v")
			case "replace":
				// Move to the first task heading and edit through the actual
				// in-app editor, including the shell's bracketed paste route.
				conflictKey(t, m, "e")
				for _, line := range strings.Split(*view.Versions[0].Text, "\n") {
					if strings.HasPrefix(line, "## Task:") {
						break
					}
					m.conflictBrowser.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
				}
				m.conflictBrowser.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnd})
				m.handleActivePasteOverlay(tea.PasteMsg{Content: " merged"})
				m.conflictBrowser.HandleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
			}
			msg := conflictKey(t, m, "enter")
			var ok bool
			req, ok = msg.(conflict.ResolveMsg)
			if !ok {
				t.Fatalf("expected resolution request, got %T", msg)
			}
			if choice == "stale" {
				if _, resolveErr := pair.desk.ResolveSyncConflict(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveCurrent}); resolveErr != nil {
					t.Fatal(resolveErr)
				}
			}
			if handled, _ := m.handleOperationOverlayMessage(req); !handled {
				t.Fatal("resolution message was not dispatched")
			}
			if choice == "stale" {
				if m.conflictBrowser == nil || !strings.Contains(m.conflictBrowser.Notice(), "no longer open") {
					t.Fatal("stale resolution was not retained as an error")
				}
				return
			}
			if m.conflictBrowser != nil {
				t.Fatalf("resolution failed: %s", m.conflictBrowser.Notice())
			}
			remaining, err := pair.desk.SyncConflicts(nil)
			if err != nil || len(remaining) != 0 {
				t.Fatalf("remaining: %d, %v", len(remaining), err)
			}
			doc, err := storage.LoadProjectDocument(pair.desk.State().SystemUnfiledProject())
			if err != nil {
				t.Fatal(err)
			}
			if choice == "replace" {
				if !strings.HasSuffix(doc.Tasks[0].Title, " merged") {
					t.Fatal("merged title not committed")
				}
			} else if !strings.Contains(expected, "## Task: "+doc.Tasks[0].Title) {
				t.Fatalf("wrong version kept: %s", doc.Tasks[0].Title)
			}
			if m.feedback.outstandingConflictCount() != 0 {
				t.Fatal("conflict indicator not refreshed")
			}
		})
	}
}

func TestConflictBrowserResolvesProjectBundleAndRefreshesTasks(t *testing.T) {
	pair := newEnrolledPair(t)
	rt := pair.original
	project := rt.State().SystemUnfiledProject()
	base, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatal(err)
	}
	local, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatal(err)
	}
	local.Tasks[0].Title = "Chosen local title"
	m, err := NewAppModel(rt)
	if err != nil {
		t.Fatal(err)
	}
	id, err := storage.RecordMergeConflict(m.conflictStore(), project, base, local, base, nil, "rename", "")
	if err != nil {
		t.Fatal(err)
	}
	m.openConflictBrowser()
	conflictKey(t, m, "enter")
	conflictKey(t, m, "l")
	req := conflictKey(t, m, "enter").(conflict.ResolveMsg)
	if req.ID != id {
		t.Fatal("wrong bundle selected")
	}
	if handled, _ := m.handleOperationOverlayMessage(req); !handled {
		t.Fatal("resolution message was not dispatched")
	}
	if m.conflictBrowser != nil {
		t.Fatalf("resolution failed: %s", m.conflictBrowser.Notice())
	}
	doc, err := storage.LoadProjectDocument(project)
	if err != nil || doc.Tasks[0].Title != "Chosen local title" {
		t.Fatalf("local title not committed: %v", err)
	}
	cached := m.domain.projects[project.ID]
	if cached == nil || cached.Tasks[0].Title != "Chosen local title" {
		t.Fatal("project cache not refreshed")
	}
}
