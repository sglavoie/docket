package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// attentionCase is one row of the acceptance matrix: a task carrying one
// combination of attention signals, plus the plain text that combination must
// still spell once every color is thrown away.
type attentionCase struct {
	name string
	// task returns a fresh task per render, since a row renderer is entitled to
	// read and a selection is per-fixture.
	task func(now time.Time) *models.Task
	// plain is the extra text the row must contain with the color stripped. The
	// state and priority glyphs are added for every case, since they are what
	// makes the same screen legible to a monochrome terminal and to a reader who
	// cannot tell the two reds apart.
	plain []string
	// absent is text the row must not spell, which is how a quiet signal is held
	// quiet rather than merely unasserted.
	absent []string
	// tinted is true only for the one signal that repaints a whole row.
	tinted bool
	// titleStyle names the style the title must wear. Every attention signal
	// resolves to NormalStyle; lifecycle is the only thing that moves it.
	titleStyle func() lipgloss.Style
}

func attentionMatrix() []attentionCase {
	normal := func() lipgloss.Style { return theme.NormalStyle }
	return []attentionCase{
		{
			name: "overdue",
			task: func(now time.Time) *models.Task {
				due := now.AddDate(0, 0, -3)
				return &models.Task{
					ID: "overdue", ShortID: 1, Title: "Past due",
					WorkflowState: models.StateBacklog, DueDate: &due,
				}
			},
			plain:      []string{"Past due"},
			tinted:     true,
			titleStyle: normal,
		},
		{
			name: "review-flag",
			task: func(now time.Time) *models.Task {
				return &models.Task{
					ID: "flagged", ShortID: 2, Title: "Flagged for review",
					WorkflowState: models.StateBacklog, Review: true,
				}
			},
			plain:      []string{"Flagged for review", "review"},
			titleStyle: normal,
		},
		{
			name: "check-in-arrived",
			task: func(now time.Time) *models.Task {
				owed := now.AddDate(0, 0, -2)
				return &models.Task{
					ID: "owed", ShortID: 3, Title: "Chase the vendor",
					WorkflowState: models.StateBacklog, NextCheckIn: &owed,
				}
			},
			plain:      []string{"Chase the vendor", "review +2d"},
			titleStyle: normal,
		},
		{
			name: "check-in-ahead",
			task: func(now time.Time) *models.Task {
				ahead := now.AddDate(0, 0, 4)
				return &models.Task{
					ID: "ahead", ShortID: 4, Title: "Not owed yet",
					WorkflowState: models.StateBacklog, NextCheckIn: &ahead,
				}
			},
			plain:      []string{"Not owed yet"},
			absent:     []string{"review"},
			titleStyle: normal,
		},
		{
			name: "due-today",
			task: func(now time.Time) *models.Task {
				// A date-only deadline, so the day is what is compared: a due
				// date carrying this morning's clock time would already be
				// overdue and would test the tier above this one.
				today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				return &models.Task{
					ID: "today", ShortID: 5, Title: "Due today",
					WorkflowState: models.StateBacklog, DueDate: &today,
				}
			},
			plain:      []string{"Due today"},
			titleStyle: normal,
		},
		{
			name: "priority-1",
			task: func(now time.Time) *models.Task {
				return &models.Task{
					ID: "p1", ShortID: 6, Title: "Top priority",
					WorkflowState: models.StateBacklog, Priority: 1,
				}
			},
			plain:      []string{"Top priority"},
			titleStyle: normal,
		},
		{
			name: "priority-3",
			task: func(now time.Time) *models.Task {
				return &models.Task{
					ID: "p3", ShortID: 7, Title: "Low priority",
					WorkflowState: models.StateBacklog, Priority: 3,
				}
			},
			plain:      []string{"Low priority"},
			titleStyle: normal,
		},
		{
			name: "waiting",
			task: func(now time.Time) *models.Task {
				return &models.Task{
					ID: "waiting", ShortID: 8, Title: "Blocked on someone",
					WorkflowState: models.StateWaiting, StateChangedAt: now.AddDate(0, 0, -5),
				}
			},
			plain: []string{"Blocked on someone"},
			// The state badge is identity and says it; the age it used to wear
			// beside it is Peek's now, so an ordinary Waiting row stays quiet.
			absent:     []string{"waiting"},
			titleStyle: normal,
		},
		{
			name: "waiting-overdue-p1",
			task: func(now time.Time) *models.Task {
				due := now.AddDate(0, 0, -6)
				return &models.Task{
					ID: "waiting-late", ShortID: 9, Title: "Blocked and late",
					WorkflowState: models.StateWaiting, StateChangedAt: now.AddDate(0, 0, -5),
					DueDate: &due, Priority: 1, Review: true,
				}
			},
			plain:      []string{"Blocked and late", "review"},
			absent:     []string{"waiting"},
			tinted:     true,
			titleStyle: normal,
		},
		{
			name: "canceled",
			task: func(now time.Time) *models.Task {
				due := now.AddDate(0, 0, -3)
				return &models.Task{
					ID: "canceled", ShortID: 10, Title: "Abandoned work",
					WorkflowState: models.StateCanceled, DueDate: &due, Priority: 1,
					StateChangedAt: now.AddDate(0, 0, -1),
				}
			},
			plain:      []string{"Abandoned work"},
			titleStyle: func() lipgloss.Style { return theme.CanceledStyle },
		},
		{
			name: "done",
			task: func(now time.Time) *models.Task {
				completed := now.AddDate(0, 0, -1)
				return &models.Task{
					ID: "done", ShortID: 11, Title: "Finished work",
					WorkflowState: models.StateDone, CompletedAt: &completed,
					StateChangedAt: completed, Priority: 1,
				}
			},
			plain:      []string{"Finished work"},
			titleStyle: normal,
		},
		{
			name: "semantic-tag",
			task: func(now time.Time) *models.Task {
				return &models.Task{
					ID: "bug", ShortID: 12, Title: "Crash on save",
					WorkflowState: models.StateBacklog, Tags: []string{"bug"}, Priority: 1,
				}
			},
			plain: []string{"Crash on save"},
			// A semantic tag reaches the reader as a Peek chip. It was never
			// allowed to reach them through the title, and it no longer reaches
			// them through a row chip either.
			absent:     []string{"#bug"},
			titleStyle: normal,
		},
	}
}

// TestAttentionSignalsRenderAcrossThemesAndFocus is the acceptance pass over the
// finished bytes rather than over a style lookup. Every attention combination is
// rendered in both themes, under the cursor and away from it, with and without a
// visual selection, and then projected through an ASCII color profile: what
// survives that projection is what a monochrome terminal shows, so a signal that
// exists only as a color fails here by name.
func TestAttentionSignalsRenderAcrossThemesAndFocus(t *testing.T) {
	now := time.Now()
	focusStates := []struct {
		name     string
		cursor   bool
		selected bool
	}{
		{name: "resting"},
		{name: "focused", cursor: true},
		{name: "selected", selected: true},
		{name: "focused-and-selected", cursor: true, selected: true},
	}

	for _, themeName := range []string{"dark", "light"} {
		for _, testCase := range attentionMatrix() {
			for _, focus := range focusStates {
				t.Run(themeName+"/"+testCase.name+"/"+focus.name, func(t *testing.T) {
					task := testCase.task(now)
					outlinerView, project := cursorHighlightOutlinerTheme(t, themeName, task)
					row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}
					if focus.selected {
						outlinerView.ReplaceSelection([]taskref.TaskIdentity{outlinerView.TaskIdentityForRow(row)})
					}

					line := outlinerView.RenderOutlinerTaskRow(row, focus.cursor, 100, 0, nil)
					plain := ansi.Strip(line)

					for _, want := range requiredPlainText(task, testCase) {
						if !strings.Contains(plain, want) {
							t.Fatalf("row does not spell %q without color:\n%s", want, plain)
						}
					}
					for _, unwanted := range testCase.absent {
						if strings.Contains(plain, unwanted) {
							t.Fatalf("row spelled %q it had not earned:\n%s", unwanted, plain)
						}
					}
					assertMonochromeKeepsContent(t, line, plain)
					assertTitleWearsStyle(t, line, testCase.titleStyle(), task.Title)
					assertRowTint(t, line, testCase.tinted, focus.cursor)
				})
			}
		}
	}
}

// requiredPlainText adds the two glyphs every row carries to the case's own
// words. Lifecycle state and assigned priority are exactly the two things this
// session had to keep readable while the title went neutral, and each is a shape
// rather than a color, so both survive a terminal that paints nothing.
func requiredPlainText(task *models.Task, testCase attentionCase) []string {
	required := append([]string{}, testCase.plain...)
	required = append(required, strings.TrimSpace(ansi.Strip(theme.GetStateBadge(task.WorkflowState))))
	if task.Priority != 0 {
		required = append(required, strings.TrimSpace(ansi.Strip(chrome.RenderPriorityBadge(task.Priority))))
	}
	return required
}

// assertMonochromeKeepsContent is the `--no-color` half of the pass: an ASCII
// color profile drops every SGR color the row asked for, and the text that comes
// out the other side must be the same text. A signal carried by color alone
// disappears here while one carried by a word does not.
func assertMonochromeKeepsContent(t *testing.T, line, plain string) {
	t.Helper()
	var mono bytes.Buffer
	writer := colorprofile.Writer{Forward: &mono, Profile: colorprofile.ASCII}
	if _, err := writer.WriteString(line); err != nil {
		t.Fatal(err)
	}
	if got := ansi.Strip(mono.String()); got != plain {
		t.Fatalf("monochrome projection changed the row:\nwant %q\ngot  %q", plain, got)
	}
}

// assertTitleWearsStyle is the session's rule stated over the bytes: a title is
// neutral text, and the only thing that may move it is lifecycle. Each attention
// style is named as a negative so a future renderer that reaches for one fails
// here rather than in review.
//
// The cursor's band is taken off the line first, because it arms itself after
// every reset the title carries and a title drawn one rune at a time — the
// canceled one is — would otherwise read as repainted when every rune of it still
// wears exactly this style.
func assertTitleWearsStyle(t *testing.T, line string, want lipgloss.Style, title string) {
	t.Helper()
	line = withoutTheCursorBand(t, line)
	if !strings.Contains(line, want.Render(title)) {
		t.Fatalf("title did not render as %q:\n%q", want.Render(title), line)
	}
	for name, unwanted := range map[string]lipgloss.Style{
		"priority 1 title":  theme.GetTitlePriorityStyle(1),
		"priority 2 title":  theme.GetTitlePriorityStyle(2),
		"priority 3 title":  theme.GetTitlePriorityStyle(3),
		"selected title":    theme.SelectedTaskTitleStyle,
		"overdue title":     theme.OverdueStyle,
		"due today title":   theme.DueTodayStyle,
		"semantic bug tint": theme.SemanticTaskTitleStyle(models.SemanticTaskTagBug, 0, false, false),
	} {
		rendered := unwanted.Render(title)
		if rendered == want.Render(title) {
			continue
		}
		if strings.Contains(line, rendered) {
			t.Fatalf("title was repainted with the %s:\n%q", name, line)
		}
	}
}

// assertRowTint holds the loudest mark to the top of the ladder. Overdue is the
// only signal that repaints a whole row, so every other combination — Waiting
// included — must leave the row background alone.
//
// The cursor's band is above even that: a background says one thing at a time, so
// the row the cursor is on wears the band and the tint is not applied over or
// under it. The deadline is still on that row in its badge, its `!` emphasis and
// its overdue text color, all of which the band leaves alone, and the assertions
// above this one are what hold them there.
func assertRowTint(t *testing.T, line string, want, cursor bool) {
	t.Helper()
	tint := backgroundParams(t, theme.OverdueRowStyle.GetBackground())
	if cursor {
		if strings.Contains(line, tint) {
			t.Fatalf("the cursor row wears both the band and the overdue tint:\n%q", line)
		}
		if !strings.Contains(line, backgroundParams(t, theme.ColorCursorRowBg)) {
			t.Fatalf("the cursor row wears no band:\n%q", line)
		}
		return
	}
	if got := strings.Contains(line, tint); got != want {
		if want {
			t.Fatalf("row dropped the overdue tint it earned:\n%q", line)
		}
		t.Fatalf("row painted the overdue tint it did not earn:\n%q", line)
	}
}

// monochromeRowText renders one task as a row and returns what a terminal that
// paints nothing would show.
func monochromeRowText(t *testing.T, task *models.Task) string {
	t.Helper()
	outlinerView, project := cursorHighlightOutlinerTheme(t, "dark", task)
	row := outliner.OutlinerRow{Task: task, ProjectID: project.ID}
	return strings.TrimRight(ansi.Strip(outlinerView.RenderOutlinerTaskRow(row, false, 100, 0, nil)), " ")
}

// TestAttentionSignalsAreDistinguishableWithoutColor is the check the matrix
// above cannot make: it holds the title fixed across every attention
// combination, so the only thing that can tell two rows apart is what the row
// says. If a future signal is carried by a hue alone, two of these rows collapse
// onto one string and this fails naming both.
func TestAttentionSignalsAreDistinguishableWithoutColor(t *testing.T) {
	now := time.Now()
	const title = "One fixed title"
	overdue := now.AddDate(0, 0, -3)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	owed := now.AddDate(0, 0, -2)
	completed := now.AddDate(0, 0, -1)

	distinct := map[string]*models.Task{
		"plain":            {ID: "a", ShortID: 1, Title: title, WorkflowState: models.StateBacklog},
		"overdue":          {ID: "b", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, DueDate: &overdue},
		"due today":        {ID: "c", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, DueDate: &today},
		"review flag":      {ID: "d", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, Review: true},
		"check-in arrived": {ID: "e", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, NextCheckIn: &owed},
		"priority 1":       {ID: "f", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, Priority: 1},
		"priority 3":       {ID: "g", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, Priority: 3},
		"waiting":          {ID: "h", ShortID: 1, Title: title, WorkflowState: models.StateWaiting, StateChangedAt: now.AddDate(0, 0, -5)},
		"canceled":         {ID: "i", ShortID: 1, Title: title, WorkflowState: models.StateCanceled, StateChangedAt: completed},
		"done":             {ID: "j", ShortID: 1, Title: title, WorkflowState: models.StateDone, StateChangedAt: completed, CompletedAt: &completed},
	}

	seen := make(map[string]string, len(distinct))
	for name, task := range distinct {
		text := monochromeRowText(t, task)
		if other, clash := seen[text]; clash {
			t.Fatalf("%q and %q are the same row without color:\n%s", name, other, text)
		}
		seen[text] = name
	}
}

// TestFutureCheckInAddsNothingToItsRow is the deliberate exception to the rule
// above, and it is stated rather than left out: a schedule whose day has not
// come is not asking for anything yet, so its row is the same row it would be
// with no schedule at all and the date lives in Peek until it arrives.
func TestFutureCheckInAddsNothingToItsRow(t *testing.T) {
	now := time.Now()
	ahead := now.AddDate(0, 0, 4)
	const title = "One fixed title"

	plain := monochromeRowText(t, &models.Task{
		ID: "a", ShortID: 1, Title: title, WorkflowState: models.StateBacklog,
	})
	scheduled := monochromeRowText(t, &models.Task{
		ID: "a", ShortID: 1, Title: title, WorkflowState: models.StateBacklog, NextCheckIn: &ahead,
	})

	if plain != scheduled {
		t.Fatalf("a check-in still ahead changed its row:\nwant %q\ngot  %q", plain, scheduled)
	}
}
