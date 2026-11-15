package outliner

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/util"
)

func allProgressOutliner(t *testing.T, count int) *OutlinerModel {
	t.Helper()
	now := time.Now()
	done := primaryRowsTask("done", models.StateDone)
	done.CompletedAt = &now
	tasks := []*models.Task{done}
	for i := range count {
		tasks = append(tasks, primaryRowsTask(fmt.Sprintf("open-%d", i), models.StateInbox))
	}
	m := nowCardOutliner(t, 80, 30, tasks...)
	m.SelectOutlinerMode(chrome.ModeAll)
	m.CompletionActivity = app.CompletionActivity{DailyCompletions: map[time.Time]int{
		util.StartOfDay(now.AddDate(0, 0, -1)): 2,
		util.StartOfDay(now):                   99, // the canvas must use the population for today
	}}
	return m
}

func TestAllDrawsProgressLineWhenRoomy(t *testing.T) {
	for _, dock := range []bool{false, true} {
		t.Run(fmt.Sprintf("dock=%t", dock), func(t *testing.T) {
			m := allProgressOutliner(t, 2)
			if dock {
				m.TogglePeek()
			}
			activity := m.CompletionActivity
			m.CompletionActivity = app.CompletionActivity{}
			m.RenderOutlinerTreeContent(80)
			before := m.LastViewport
			cursor, offset, dockTop := m.Cursor, m.ViewScrollOffset, m.pointerFrame.dockTop
			screenRow, _ := m.CursorScreenRow()
			rows := append([]OutlinerRow(nil), m.Rows...)
			m.CompletionActivity = activity
			view := ansi.Strip(m.RenderOutlinerTreeContent(80))
			if !strings.Contains(view, "Done today · 1   this week ▁▁▁▁▁█▄ 3") {
				t.Fatalf("missing canonical progress reading:\n%s", view)
			}
			after := m.LastViewport
			if after.canvasRows != 2 {
				t.Fatalf("canvas rows = %d, want separator and line", after.canvasRows)
			}
			after.canvasRows = 0
			if after != before || m.Cursor != cursor || m.ViewScrollOffset != offset || !reflect.DeepEqual(rows, m.Rows) {
				t.Fatalf("progress moved the list: before %+v, after %+v", before, after)
			}
			if got, _ := m.CursorScreenRow(); got != screenRow || m.pointerFrame.dockTop != dockTop {
				t.Fatal("progress moved cursor or dock on screen")
			}
			progressRow := before.topRow + before.drawnRows() + 1
			lines := strings.Split(view, "\n")
			if strings.TrimSpace(lines[progressRow-1]) != "" || !strings.Contains(lines[progressRow], "Done today") {
				t.Fatalf("progress must follow one blank row under the list:\n%s", view)
			}
			for _, y := range []int{progressRow - 1, progressRow} {
				if hit, ok := m.pointerHitAt(4, y); ok {
					t.Fatalf("paint targeted row %+v", hit)
				}
			}
		})
	}
}

func TestAllProgressLineHiddenWhenFull(t *testing.T) {
	m := allProgressOutliner(t, 30)
	m.SetSize(chrome.MinTerminalWidth, chrome.MinTerminalHeight-2)
	view := ansi.Strip(m.RenderOutlinerTreeContent(chrome.MinTerminalWidth))
	if strings.Contains(view, "Done today") || m.LastViewport.canvasRows != 0 {
		t.Fatalf("full frame drew progress:\n%s", view)
	}
	// At the exact boundary the separator cannot survive on its own, even when
	// transient fold paint takes the second spare row of a settled window.
	m = allProgressOutliner(t, 2)
	m.RenderOutlinerTreeContent(80)
	viewport := m.LastViewport
	viewport.canvasRows = 0
	viewport.VisibleRows = viewport.drawnRows() + 2
	for _, gap := range []int{0, 1, 2} {
		m.LastViewport = viewport
		m.LastViewport.gapRows = gap
		var b strings.Builder
		m.writeDayCanvas(&b, viewport)
		want := 0
		if gap == 0 {
			want = 2
		}
		if m.LastViewport.canvasRows != want {
			t.Fatalf("gap %d: drew %d rows, want %d", gap, m.LastViewport.canvasRows, want)
		}
	}
}

func TestAllProgressLineHiddenWhenNothingDoneThisWeek(t *testing.T) {
	m := allProgressOutliner(t, 2)
	for _, p := range m.ScopeProjects() {
		for _, task := range p.Tasks {
			if task.IsTerminalState() {
				old := time.Now().AddDate(0, 0, -8)
				task.CompletedAt = &old
			}
		}
	}
	m.CompletionActivity = app.CompletionActivity{DailyCompletions: map[time.Time]int{
		util.StartOfDay(time.Now().AddDate(0, 0, -8)): 3,
	}}
	if view := ansi.Strip(m.RenderOutlinerTreeContent(80)); strings.Contains(view, "Done today") {
		t.Fatalf("old history drew progress:\n%s", view)
	}
	t.Run("empty projection", func(t *testing.T) {
		empty := allProgressOutliner(t, 0)
		if view := ansi.Strip(empty.RenderOutlinerTreeContent(80)); strings.Contains(view, "Done today") {
			t.Fatalf("empty projection drew progress:\n%s", view)
		}
	})
	t.Run("nothing today but some yesterday", func(t *testing.T) {
		m.CompletionActivity.DailyCompletions[util.StartOfDay(time.Now().AddDate(0, 0, -1))] = 2
		if view := ansi.Strip(m.RenderOutlinerTreeContent(80)); !strings.Contains(view, "Done today · 0   this week") {
			t.Fatalf("yesterday's progress missing:\n%s", view)
		}
	})
}

func TestAllProgressLineHiddenUnderSearch(t *testing.T) {
	for _, ranked := range []bool{false, true} {
		t.Run(fmt.Sprintf("ranked=%t", ranked), func(t *testing.T) {
			m := allProgressOutliner(t, 2)
			if ranked {
				m.BeginSearchProjection()
			} else {
				m.SetSearchQuery("open")
			}
			if view := ansi.Strip(m.RenderOutlinerTreeContent(80)); strings.Contains(view, "Done today") {
				t.Fatalf("search drew progress:\n%s", view)
			}
		})
	}
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAgenda, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent} {
		t.Run(string(mode), func(t *testing.T) {
			m := allProgressOutliner(t, 2)
			m.SelectOutlinerMode(mode)
			if view := ansi.Strip(m.RenderOutlinerTreeContent(80)); strings.Contains(view, "Done today") {
				t.Fatalf("%s drew progress:\n%s", mode, view)
			}
		})
	}
}
