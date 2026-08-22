package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestPanelEndKeysReachBothEndsOfTheList holds Notes, Comments, and Attachments
// to the spelling the Outliner and the Hub already answer to. Each panel owns
// its own cursor, so the keys reaching the ends of one list say nothing about
// the other two and all three are pressed here.
func TestPanelEndKeysReachBothEndsOfTheList(t *testing.T) {
	t.Run("notes", func(t *testing.T) {
		view := mountEndsNavigationNotes(t)
		if got := len(view.visibleRows); got != 3 {
			t.Fatalf("visible notes = %d, want 3", got)
		}
		press := notesKeyPresser(t, view)
		noise := func() string { return view.statusMessage }
		assertPanelReachesBothEnds(t, press, func() int { return view.cursor }, 3)
		assertLoneChordPrefixIsSilent(t, press, func() int { return view.cursor }, noise)
	})

	t.Run("comments", func(t *testing.T) {
		_, panel, press := mountCommentsPanel(t)
		if got := len(panel.task.Comments); got != 2 {
			t.Fatalf("mounted comments = %d, want 2", got)
		}
		assertPanelReachesBothEnds(t, press, func() int { return panel.cursor }, 2)
		assertLoneChordPrefixIsSilent(t, press, func() int { return panel.cursor }, panelNoise(&panel.BaseView, &panel.InteractiveViewState))
	})

	t.Run("attachments", func(t *testing.T) {
		_, panel, press := mountAttachmentsPanel(t)
		if got := len(panel.task.Attachments); got != 2 {
			t.Fatalf("mounted attachments = %d, want 2", got)
		}
		assertPanelReachesBothEnds(t, press, func() int { return panel.cursor }, 2)
		assertLoneChordPrefixIsSilent(t, press, func() int { return panel.cursor }, panelNoise(&panel.BaseView, &panel.InteractiveViewState))
	})
}

// TestEntitySurfaceHelpDocumentsBothEndSpellings holds all four entity surfaces
// to the grammar the test above proves they execute. Each surface's own `?` help
// must name gg/Home and G/End once apiece: once, because four physical keys
// reaching two acts are two rows rather than four, and named, because a key the
// surface answers and never spells is undiscoverable from inside it.
//
// The Outliner is absent because its help is the registry's own projection of
// the same two DisplayKeys, which is what these four surfaces are being held to.
func TestEntitySurfaceHelpDocumentsBothEndSpellings(t *testing.T) {
	for _, surface := range []struct {
		name string
		help func(*testing.T) []HelpItem
	}{
		{
			name: "project hub",
			help: func(t *testing.T) []HelpItem {
				return openProjectHub(t, projectHubApp(t)).GetHelpItems()
			},
		},
		{
			name: "notes",
			help: func(t *testing.T) []HelpItem {
				return mountEndsNavigationNotes(t).GetHelpItems()
			},
		},
		{
			name: "comments",
			help: func(t *testing.T) []HelpItem {
				_, panel, _ := mountCommentsPanel(t)
				return panel.GetHelpItems()
			},
		},
		{
			name: "attachments",
			help: func(t *testing.T) []HelpItem {
				_, panel, _ := mountAttachmentsPanel(t)
				return panel.GetHelpItems()
			},
		},
	} {
		t.Run(surface.name, func(t *testing.T) {
			items := surface.help(t)
			if len(items) == 0 {
				t.Fatal("the surface documents no keys at all")
			}
			spelled := make(map[string]int, len(items))
			for _, item := range items {
				spelled[item.Key]++
			}
			for _, spelling := range []string{"gg/Home", "G/End"} {
				if spelled[spelling] != 1 {
					t.Errorf("help names %q %d times, want exactly 1: %+v", spelling, spelled[spelling], items)
				}
			}
			// The composed spellings replace the bare keys rather than joining them,
			// so the same act cannot be discovered twice under two names. `end` was
			// already hidden on the panels that bind it.
			for _, superseded := range []string{"home", "G", "end", "gg"} {
				if spelled[superseded] != 0 {
					t.Errorf("help still names the bare key %q beside the composed spelling", superseded)
				}
			}
		})
	}
}

// assertPanelReachesBothEnds presses each of the four end spellings in turn. The
// presses alternate ends so a cursor that never moved cannot pass by sitting
// where the previous step left it.
func assertPanelReachesBothEnds(t *testing.T, press func(string), cursor func() int, rows int) {
	t.Helper()
	for _, step := range []struct {
		keys []string
		want int
	}{
		{keys: []string{"G"}, want: rows - 1},
		{keys: []string{"g", "g"}, want: 0},
		{keys: []string{"end"}, want: rows - 1},
		{keys: []string{"home"}, want: 0},
	} {
		for _, key := range step.keys {
			press(key)
		}
		if got := cursor(); got != step.want {
			t.Errorf("%v left the cursor on row %d, want %d", step.keys, got, step.want)
		}
	}
}

// assertLoneChordPrefixIsSilent reproduces the Hub's deliberate abandon: a g the
// user does not complete says nothing, moves nothing, and hands the key after it
// to the grammar unread, so a mistyped prefix costs one press rather than the
// act the next key names.
func assertLoneChordPrefixIsSilent(t *testing.T, press func(string), cursor func() int, noise func() string) {
	t.Helper()
	press("home")
	press("g")
	if got := cursor(); got != 0 {
		t.Errorf("a lone g moved the cursor to row %d", got)
	}
	if said := noise(); said != "" {
		t.Errorf("a lone g said %q; abandoning the prefix is silent", said)
	}
	press("j")
	if got := cursor(); got != 1 {
		t.Errorf("g then j left the cursor on row %d, want 1: the abandoned prefix swallowed the key after it", got)
	}
}

// panelNoise reports whatever a panel would have shown the user, so the silent
// abandon is checked against the panel's own voice rather than only its cursor.
func panelNoise(base *BaseView, state *InteractiveViewState) func() string {
	return func() string {
		if base.err != nil {
			return base.err.Error()
		}
		return state.statusMessage
	}
}

// notesKeyPresser drives the manager directly. Notes mounts inside the Manage
// hub rather than over the Outliner, so its keys arrive at the model the same
// way every other Notes test delivers them.
func notesKeyPresser(t *testing.T, view *NotesViewModel) func(string) {
	t.Helper()
	return func(key string) {
		t.Helper()
		press, err := parseUITestKey(key)
		if err != nil {
			t.Fatalf("parseUITestKey(%q): %v", key, err)
		}
		_, _ = view.Update(press)
	}
}

// mountEndsNavigationNotes builds a three-row All Notes list. Three rows is the
// smallest list where the middle is neither end, so a cursor that only clamps
// cannot pass for one that jumps.
func mountEndsNavigationNotes(t *testing.T) *NotesViewModel {
	t.Helper()
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {
			{Title: "First", Filename: "first.md", ModifiedAt: time.Unix(3, 0)},
			{Title: "Second", Filename: "second.md", ModifiedAt: time.Unix(2, 0)},
			{Title: "Third", Filename: "third.md", ModifiedAt: time.Unix(1, 0)},
		},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)
	view.SetSize(80, 24)
	return view
}
