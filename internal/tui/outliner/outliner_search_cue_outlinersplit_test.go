package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

func cueRowLine(t *testing.T, source MatchSource, width int) string {
	t.Helper()
	task := cueTask()
	outliner, project := cursorHighlightOutliner(t, task)
	row := OutlinerRow{Task: task, ProjectID: project.ID, MatchSource: source}
	return ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, 0, nil))
}

// TestSearchCueNamesEveryNonTitleSource is the cue vocabulary itself: one word
// per source the projection can report, and silence for the two sources that
// explain themselves — a title hit the row already highlights, and the no-source
// value every ordinary row carries.
func TestSearchCueNamesEveryNonTitleSource(t *testing.T) {
	cases := map[MatchSource]string{
		MatchSourceNone:        "",
		MatchSourceTitle:       "",
		MatchSourceDescription: "· description",
		MatchSourceResume:      "· resume",
		MatchSourceChecklist:   "· checklist",
		MatchSourceComment:     "· comment",
		MatchSourceTag:         "· tag",
		MatchSourceContext:     "· context",
		MatchSourceAttachment:  "· attachment",
		MatchSourceProject:     "· project",
		MatchSourceDetails:     "· details",
	}
	for source, want := range cases {
		plain := cueRowLine(t, source, 120)
		if want == "" {
			if strings.Contains(plain, "·") {
				t.Fatalf("source %d rendered a cue it should not explain: %q", source, plain)
			}
			continue
		}
		if !strings.Contains(plain, want) {
			t.Fatalf("source %d missing cue %q: %q", source, want, plain)
		}
	}

	// Every source between the two silent ends must be named, so a tier added to
	// the projection cannot reach a row as an unexplained result.
	for source := MatchSourceDescription; source <= MatchSourceDetails; source++ {
		if matchSourceLabels[source] == "" {
			t.Fatalf("source %d has no cue label", source)
		}
	}
}

// TestSearchCueRendersFromTheLiveProjection proves the row reads the attribution
// the search itself produced, rather than a value only a test can set.
func TestSearchCueRendersFromTheLiveProjection(t *testing.T) {
	task := primaryRowsTask("attributed", models.StateBacklog)
	task.Title = "Untitled row"
	task.Description = "zeppelin hangar notes"
	project := primaryRowsProject("holdings", task)
	outliner := scopeTestOutliner(project)

	render := func() string {
		t.Helper()
		if len(outliner.Rows) == 0 || outliner.Rows[0].Task != task {
			t.Fatalf("search rows = %v, want the seeded task", searchVisibleTaskIDs(outliner))
		}
		return ansi.Strip(outliner.RenderOutlinerTaskRow(outliner.Rows[0], false, 120, 0, nil))
	}

	searchRankedList(outliner, "zeppelin")
	if plain := render(); !strings.Contains(plain, "· description") {
		t.Fatalf("description hit did not explain itself: %q", plain)
	}

	searchRankedList(outliner, "untitled")
	if plain := render(); strings.Contains(plain, "·") {
		t.Fatalf("title hit wore a cue: %q", plain)
	}

	outliner.ClearSearch()
	for index, row := range outliner.Rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, index, nil)); strings.Contains(plain, "·") {
			t.Fatalf("restored row wore a cue: %q", plain)
		}
	}
}

// TestSearchCueKeepsCursorPaintAndNeutralTitle holds the split paint against the
// new segment: the cue is content, so the bar and the trailing fill still mark
// the cursor, and the title beside them stays ordinary text in both themes.
func TestSearchCueKeepsCursorPaintAndNeutralTitle(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			task := cueTask()
			task.Priority = 2
			outliner, project := cursorHighlightOutlinerTheme(t, themeName, task)
			row := OutlinerRow{Task: task, ProjectID: project.ID, MatchSource: MatchSourceComment}
			cue := theme.MutedStyle.Render("  · comment")

			for _, selected := range []bool{false, true} {
				line := outliner.RenderOutlinerTaskRow(row, selected, 80, 0, nil)
				if !strings.Contains(line, cue) {
					t.Fatalf("selected=%v row lost its muted cue:\n%q", selected, line)
				}
				if !strings.Contains(line, theme.NormalStyle.Render(task.Title)) {
					t.Fatalf("selected=%v row did not render its title as ordinary text:\n%q", selected, line)
				}
				if strings.Contains(line, theme.GetTitlePriorityStyle(2).Render(task.Title)) {
					t.Fatalf("selected=%v row colored its title by priority:\n%q", selected, line)
				}
				if got := lipgloss.Width(line); got > 80 {
					t.Fatalf("selected=%v row rendered %d columns, want at most 80", selected, got)
				}
				if !selected {
					continue
				}
				if got := lipgloss.Width(line); got != 80 {
					t.Fatalf("cursor row rendered %d columns, want exactly 80", got)
				}
				if !strings.Contains(line, backgroundParams(t, theme.ColorCursorBarBg)) {
					t.Fatalf("cue row dropped its cursor bar:\n%q", line)
				}
				if !strings.Contains(line, backgroundParams(t, theme.ColorSelectedDimBg)) {
					t.Fatalf("cue row dropped its trailing fill:\n%q", line)
				}
			}
		})
	}
}

// TestSearchCueIsChargedOnceAtEveryWidth measures the cue like any other
// segment, including under a title whose runes are two columns wide.
func TestSearchCueIsChargedOnceAtEveryWidth(t *testing.T) {
	for _, title := range []string{"Untitled row", "重建采石场的排水系统"} {
		task := cueTask()
		task.Title = title
		outliner, project := cursorHighlightOutliner(t, task)
		row := OutlinerRow{Task: task, ProjectID: project.ID, MatchSource: MatchSourceAttachment}

		for _, width := range []int{8, 12, 20, 30, 40, 60, 80, 120} {
			for _, selected := range []bool{false, true} {
				line := outliner.RenderOutlinerTaskRow(row, selected, width, 0, nil)
				got := lipgloss.Width(line)
				if selected && got != width {
					t.Fatalf("%q cursor row at width %d rendered %d columns", title, width, got)
				}
				if got > width {
					t.Fatalf("%q row at width %d assembled %d columns: %q", title, width, got, ansi.Strip(line))
				}
			}

			segments := outliner.buildTaskRowSegments(row, width, nil, cursorPaint{})
			bare := row
			bare.MatchSource = MatchSourceNone
			bareSegments := outliner.buildTaskRowSegments(bare, width, nil, cursorPaint{})
			if segments.matchCue == "" {
				continue
			}
			if want := bareSegments.titleWidth(width) - lipgloss.Width(segments.matchCue); segments.titleWidth(width) != want {
				t.Fatalf("%q cue at width %d charged the title budget %d, want %d",
					title, width, segments.titleWidth(width), want)
			}
		}
	}
}

// cueLadderTask is a row wearing as many indicators at once as the lifecycle
// permits, so the pressure ladder has something to shed after the cue goes.
func cueLadderTask(state models.WorkflowState) *models.Task {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, -4))
	deferred := now.Add(48 * time.Hour)
	return &models.Task{
		ID:             "ladder-" + string(state),
		ShortID:        1,
		Title:          "Investigate the intermittent portal sync timeout",
		Context:        "portal",
		Tags:           []string{"release"},
		WorkflowState:  state,
		StateChangedAt: now.AddDate(0, 0, -120),
		UpdatedAt:      now,
		DueDate:        &due,
		DeferredUntil:  &deferred,
		DeferCount:     3,
		Checklist:      []models.ChecklistItem{{Text: "Draft the notes"}},
		Comments:       []*models.Comment{{Text: "One"}},
		Attachments:    []models.Attachment{{Filename: "trace.log"}},
	}
}

// TestSearchCueDropsBeforeEveryIndicator locks the cue's place under width
// pressure: it is the first thing a crowded row sheds, so no member of
// indicatorDropLadder — and no tag chip — can be missing while the cue survives.
func TestSearchCueDropsBeforeEveryIndicator(t *testing.T) {
	// Doing and Waiting are exclusive, and the check-back countdown only exists on
	// the waiting row, so the invariant is stated against both.
	for _, state := range []models.WorkflowState{models.StateDoing, models.StateWaiting} {
		t.Run(string(state), func(t *testing.T) {
			task := cueLadderTask(state)
			outliner, project := cursorHighlightOutliner(t, task)
			row := OutlinerRow{Task: task, ProjectID: project.ID, MatchSource: MatchSourceChecklist}

			full := outliner.renderTaskIndicatorSegments(row, time.Now()).render()
			for _, want := range []string{"deferred ×3", "☑", "▪ 1", "⊕ 1"} {
				if !strings.Contains(ansi.Strip(full), want) {
					t.Fatalf("fixture did not wear %q: %q", want, ansi.Strip(full))
				}
			}

			var sawCue, sawDrop bool
			for width := 200; width >= 10; width-- {
				segments := outliner.buildTaskRowSegments(row, width, nil, cursorPaint{})
				if segments.matchCue == "" {
					sawDrop = true
					continue
				}
				sawCue = true
				if segments.indicators != full {
					t.Fatalf("width %d dropped an indicator while the cue survived: %q vs %q",
						width, ansi.Strip(segments.indicators), ansi.Strip(full))
				}
				if segments.tagSegment == "" {
					t.Fatalf("width %d dropped the tag chip while the cue survived", width)
				}
			}
			if !sawCue || !sawDrop {
				t.Fatalf("fixture never crossed the cue threshold (cue seen %v, drop seen %v)", sawCue, sawDrop)
			}
		})
	}
}
