package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// This file covers the segments a row reserves rather than earns: the relation
// and pin markers, the counts a project header carries, and the badges a
// cross-project row must not repeat. Each is reserved at every width, which is
// why these cases assert at narrow widths too. It also covers what the row no
// longer reserves — context, tags, and the supporting counts — and where each
// went. The indicators are in outliner_render_indicators_test.go and the width
// ladder itself in outliner_render_rows_pressure_test.go.

// TestTaskContextAndTagsLeaveTheRowForPeek is the two removals as one rule. The
// row said both in fixed-width badges that cost the title a dozen columns at
// every width; Peek says both at full value, which is the trade the simplified
// budget makes.
func TestTaskContextAndTagsLeaveTheRowForPeek(t *testing.T) {
	task := &models.Task{
		ID:      "task",
		Title:   "Readable title",
		Context: "abcdefghijklmnop",
		Tags:    []string{"secondary"},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	segments := outliner.buildTaskRowSegments(row, 60, nil, cursorPaint{})
	if segments.titleWidth(60) != 60-outlinerNonTitleWidth(
		segments.cursorPrefix, segments.prefixStr, segments.foldIndicator,
		segments.statusIcon, segments.prioritySegment, segments.projectLabel, segments.pinMarker,
		segments.indicators, segments.relationMarker, segments.matchCue,
	) {
		t.Fatal("a segment drifted between titleWidth and outlinerNonTitleWidth")
	}

	// 24 columns is where a reserved badge used to leave the title four of them.
	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 24, 0, nil))
	for _, gone := range []string{"[abcde", "abcdefghijk", "#secondary"} {
		if strings.Contains(plain, gone) {
			t.Fatalf("narrow row still reserves %q: %q", gone, plain)
		}
	}
	if !strings.Contains(plain, "Readable title") {
		t.Fatalf("narrow row did not spend the reclaimed width on the title: %q", plain)
	}

	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 60), "\n"))
	for _, want := range []string{"abcdefghijklmnop", "secondary"} {
		if !strings.Contains(peek, want) {
			t.Fatalf("Peek lost %q:\n%s", want, peek)
		}
	}
}

func TestProjectHeaderPressureDegradesBeforeTruncation(t *testing.T) {
	const line = "▾ [project] (4)"
	row := OutlinerRow{OverdueCount: 2, TodayCount: 3}

	cases := []struct {
		name     string
		rowWidth int
		want     string
	}{
		{"both counts fit", 60, "▾ [project] (4)  ! overdue 2 · $today 3"},
		{"today drops first", 30, "▾ [project] (4)  ! overdue 2"},
		{"both drop", 20, "▾ [project] (4)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ansi.Strip(appendProjectHeaderPressure(line, row, tc.rowWidth))
			if got != tc.want {
				t.Fatalf("pressure suffix at width %d = %q, want %q", tc.rowWidth, got, tc.want)
			}
		})
	}

	quiet := ansi.Strip(appendProjectHeaderPressure(line, OutlinerRow{}, 60))
	if quiet != line {
		t.Fatalf("a group with no pressure wore a suffix: %q", quiet)
	}
}

// A folded group hides the rows whose own chips would have said what is urgent
// inside it, so the header says it at the same weight. The two counts still read
// as two facts: the deadline count wears the due family's accent and the Today
// count wears the plan's, and the header is where a shared weight could most
// easily collapse them into one.
func TestProjectHeaderPressureCountsCarryTheirEmphasis(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	const line = "▾ [project] (4)"
	row := OutlinerRow{OverdueCount: 2, TodayCount: 3}

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			suffix := appendProjectHeaderPressure(line, row, 60)
			overdue := theme.ProjectPressureOverdueStyle.Render("! overdue 2")
			today := theme.ProjectPressureTodayStyle.Render("$today 3")
			if !strings.Contains(suffix, overdue) {
				t.Errorf("header dropped the overdue count's emphasis: %q", suffix)
			}
			if !strings.Contains(suffix, today) {
				t.Errorf("header dropped the Today count's emphasis: %q", suffix)
			}
			if overdue == today {
				t.Fatal("both pressure counts rendered alike, so a plan reads as a deadline")
			}
		})
	}
}

// Under the cursor every color a row earned survives, so the chip a deadline
// wears has to survive with it: flattening it there would hide the emphasis on
// exactly the row the user is standing on. The band is laid under that chip
// rather than over it, which is why the assertion asks about the row's own bytes
// with the band's arms removed and then asks for the band separately.
func TestSelectedRowKeepsItsDueChip(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	due := util.StartOfDay(time.Now())
	task := &models.Task{
		ID:            "due-today",
		Title:         "Answer the auditor",
		WorkflowState: models.StateBacklog,
		DueDate:       &due,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			chip := query.FormatDueDate(task)
			if chip == "" {
				t.Fatal("a task due today rendered no due chip")
			}
			for _, variant := range []struct {
				name     string
				selected bool
			}{{"resting", false}, {"cursor", true}} {
				line := outliner.RenderOutlinerTaskRow(row, variant.selected, 120, 0, nil)
				if !strings.Contains(withoutTheCursorBand(t, line), chip) {
					t.Errorf("%s row lost its due chip: %q", variant.name, line)
				}
				if variant.selected && !strings.Contains(line, backgroundParams(t, theme.ColorCursorRowBg)) {
					t.Errorf("cursor row wore no band under that chip: %q", line)
				}
				if got := lipgloss.Width(ansi.Strip(line)); got > 120 {
					t.Errorf("%s row assembled %d columns", variant.name, got)
				}
			}
		})
	}
}

// TestCrossProjectProjectIdentityHasExactlyOneCarrier walks both halves of the
// same guarantee: a group states its project once, on the header that opens it.
// Expanded, the header sits above the task rows and no row repeats the name;
// folded, the same header is the summary that replaces those rows.
func TestCrossProjectProjectIdentityHasExactlyOneCarrier(t *testing.T) {
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-task", models.StateBacklog))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-task", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	headed := make(map[string]int)
	for index, row := range outliner.Rows {
		if row.IsProjectHeader {
			headed[row.ProjectID]++
			continue
		}
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, index, nil))
		if strings.Contains(line, "["+row.ProjectID+"]") {
			t.Errorf("task row %q repeated its project beneath the header: %q", row.Task.ID, line)
		}
	}
	if headed[alpha.ID] != 1 || headed[beta.ID] != 1 {
		t.Fatalf("expanded headers = %v, want one per project", headed)
	}

	for _, group := range outliner.VisibleProjectGroups() {
		outliner.SetProjectGroupFolded(group, true)
	}
	outliner.RebuildRows()

	summaries := make(map[string]bool)
	for index, row := range outliner.Rows {
		if row.IsTaskRow() {
			t.Fatalf("folded group retained a task row: %+v", row)
		}
		if !row.IsProjectHeader {
			continue
		}
		summaries[row.ProjectID] = strings.Contains(
			ansi.Strip(outliner.RenderOutlinerProjectHeaderRow(row, false, 120, index)),
			"["+row.ProjectID+"]",
		)
	}
	for _, projectID := range []string{alpha.ID, beta.ID} {
		if !summaries[projectID] {
			t.Errorf("folded summary %q did not name its project", projectID)
		}
	}
}

// TestOutlinerTaskRowLeavesSupportingCountsToPeek keeps the checklist, comment,
// and attachment counts out of the row at its widest. They were never a reason
// to act on a task, only a reason to open one, and opening one is what Peek is.
func TestOutlinerTaskRowLeavesSupportingCountsToPeek(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Title:         "Review compact metadata",
		WorkflowState: models.StateBacklog,
		Comments:      []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:   []models.Attachment{{Filename: "design.pdf"}},
		Checklist:     []models.ChecklistItem{{Text: "Read", Done: true}, {Text: "Reply"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 0, nil))
	for _, gone := range []string{"▪ 2", "⊕ 1", "1/2"} {
		if strings.Contains(plain, gone) {
			t.Fatalf("task row still carries %q at its widest: %q", gone, plain)
		}
	}

	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 120), "\n"))
	for _, want := range []string{"1/2 done", "design.pdf", "Comments"} {
		if !strings.Contains(peek, want) {
			t.Fatalf("Peek lost %q:\n%s", want, peek)
		}
	}
}

// TestOutlinerTaskRowTitleStaysNeutralWithoutTagChips is the neutral-title rule
// at the row's widest: a semantic tag reaches the reader through Peek's chips
// now, and it was never allowed to reach them through the title.
func TestOutlinerTaskRowTitleStaysNeutralWithoutTagChips(t *testing.T) {
	task := &models.Task{
		ID:            "bug-task",
		Title:         "Fix capture grammar",
		WorkflowState: models.StateBacklog,
		Tags:          []string{"bug", "composer"},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 0, nil))
	if !strings.Contains(plain, "Fix capture grammar") {
		t.Fatalf("task row missing its title: %q", plain)
	}
	for _, gone := range []string{"#bug", "#composer", " B Fix capture grammar"} {
		if strings.Contains(plain, gone) {
			t.Fatalf("task row carries %q: %q", gone, plain)
		}
	}

	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 120), "\n"))
	for _, want := range []string{"bug", "composer"} {
		if !strings.Contains(peek, want) {
			t.Fatalf("Peek lost the %q chip:\n%s", want, peek)
		}
	}
}

func TestOutlinerTaskRowShowsAncestorMarkerOnlyForPromotedRoot(t *testing.T) {
	task := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	promoted := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, Breadcrumb: "Parent ›",
	}, false, 120, 0, nil))
	if !strings.Contains(promoted, "Child  ‹") {
		t.Fatalf("promoted task row omitted ancestor marker: %q", promoted)
	}
	if strings.Contains(promoted, "Parent") {
		t.Fatalf("task row spent title width on the ancestor path: %q", promoted)
	}

	nested := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, Prefix: connectorSpace + connectorLastItem,
	}, false, 120, 0, nil))
	if strings.Contains(nested, "‹") {
		t.Fatalf("nested task row retained ancestor marker: %q", nested)
	}
}

// TestOutlinerTaskRowCountsChildrenPlacedElsewhere covers the mirror mark: a
// parent whose open children are all seated in other sections otherwise renders
// with no connector beneath it and reads exactly like a leaf.
func TestOutlinerTaskRowCountsChildrenPlacedElsewhere(t *testing.T) {
	parentID := "parent"
	newChild := func(id string, state models.WorkflowState) *models.Task {
		return &models.Task{ID: id, Title: id, WorkflowState: state, ParentID: &parentID}
	}
	parent := &models.Task{ID: parentID, Title: "Parent", WorkflowState: models.StateDoing}
	tasks := []*models.Task{
		parent,
		newChild("open-a", models.StateBacklog),
		newChild("open-b", models.StateWaiting),
		newChild("finished", models.StateDone),
	}
	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.openChildCounts = query.OpenChildCounts([]*models.Project{project})

	detached := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: parent,
	}, false, 120, 0, nil))
	if !strings.Contains(detached, "Parent  ›2") {
		t.Fatalf("detached parent row omitted the child count: %q", detached)
	}

	combined := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: tasks[1], Breadcrumb: "Parent ›",
	}, false, 120, 0, nil))
	if !strings.Contains(combined, "open-a  ‹") {
		t.Fatalf("promoted child row omitted the ancestor marker: %q", combined)
	}
	if strings.Contains(combined, "›") {
		t.Fatalf("childless row carried a down marker: %q", combined)
	}

	rendered := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: parent, HasVisibleChildren: true,
	}, false, 120, 0, nil))
	if strings.Contains(rendered, "›") {
		t.Fatalf("row showing its children restated parenthood: %q", rendered)
	}

	terminal := &models.Task{ID: "terminal-parent", Title: "Terminal parent"}
	terminalParentID := terminal.ID
	closedChild := &models.Task{ID: "closed", Title: "closed", WorkflowState: models.StateCanceled, ParentID: &terminalParentID}
	closedProject := renderFixtureProject(t, "closed", "closed", "Closed", []*models.Task{terminal, closedChild})
	closedOutliner := NewOutlinerModel(closedProject, nil, newTestConfig())
	closedOutliner.openChildCounts = query.OpenChildCounts([]*models.Project{closedProject})
	quiet := ansi.Strip(closedOutliner.RenderOutlinerTaskRow(OutlinerRow{Task: terminal}, false, 120, 0, nil))
	if strings.Contains(quiet, "›") {
		t.Fatalf("parent with only terminal children carried a marker: %q", quiet)
	}
}

// TestOutlinerTaskRowCombinesBothRelationDirections pins the shared segment: a
// row that is both a promoted child and a detached parent reads `‹›N`.
func TestOutlinerTaskRowCombinesBothRelationDirections(t *testing.T) {
	rootID := "root"
	midID := "mid"
	tasks := []*models.Task{
		{ID: rootID, Title: "Root"},
		{ID: midID, Title: "Mid", WorkflowState: models.StateDoing, ParentID: &rootID},
		{ID: "leaf", Title: "Leaf", WorkflowState: models.StateBacklog, ParentID: &midID},
	}
	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.openChildCounts = query.OpenChildCounts([]*models.Project{project})

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: tasks[1], Breadcrumb: "Root ›",
	}, false, 120, 0, nil))
	if !strings.Contains(plain, "Mid  ‹›1") {
		t.Fatalf("row missing the combined relation marker: %q", plain)
	}

	// The marker is the one cue that family sits elsewhere, so it wears the
	// accent in bold rather than the muted tone of the connectors beside it.
	style := theme.RelationMarkerStyle
	if !style.GetBold() || style.GetForeground() != theme.ColorAccent {
		t.Fatal("RelationMarkerStyle is not the bold accent")
	}
	marker := outliner.taskRelationMarker(OutlinerRow{Task: tasks[1], Breadcrumb: "Root ›"})
	if want := style.Render("  ‹›1"); marker != want {
		t.Fatalf("relation marker = %q, want it drawn in RelationMarkerStyle %q", marker, want)
	}
}

// TestOutlinerTaskRowKeepsTheAncestorMarkerOnACrowdedRow is the regression: the
// marker is reserved width, so a title that overruns the row clips against it
// instead of pushing it off the line and reading as a root task.
func TestOutlinerTaskRowKeepsTheAncestorMarkerOnACrowdedRow(t *testing.T) {
	task := &models.Task{
		ID:            "child",
		Title:         strings.Repeat("Overlong child title ", 8),
		WorkflowState: models.StateBacklog,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	for _, width := range []int{60, 72, 100} {
		plain := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
			Task: task, Breadcrumb: "Grandparent › Parent ›",
		}, false, width, 0, nil))
		if !strings.Contains(plain, "…  ‹") {
			t.Fatalf("width %d dropped the marker from a clipped title: %q", width, plain)
		}
		if got := lipgloss.Width(plain); got > width {
			t.Fatalf("width %d row assembled %d columns: %q", width, got, plain)
		}
	}
}

// TestPinnedMarkerNamesAPinOutsideThePinnedLane pins the asymmetry: the lane's
// header already says it, so only rows drawn elsewhere wear the mark, and the
// mark is reserved width rather than a ladder entry.
func TestPinnedMarkerNamesAPinOutsideThePinnedLane(t *testing.T) {
	task := &models.Task{
		ID:            "pinned",
		Title:         "Pinned work",
		WorkflowState: models.StateInbox,
		Pinned:        true,
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	elsewhere := OutlinerRow{Task: task, ProjectID: project.ID}
	for _, width := range []int{120, 24} {
		plain := ansi.Strip(outliner.RenderOutlinerTaskRow(elsewhere, false, width, 0, nil))
		if !strings.Contains(plain, "* Pinne") {
			t.Fatalf("width %d dropped the pin marker: %q", width, plain)
		}
		if got := lipgloss.Width(plain); got > width {
			t.Fatalf("width %d row assembled %d columns: %q", width, got, plain)
		}
	}

	lane := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, ProjectID: project.ID, IsPinnedSection: true,
	}, false, 120, 0, nil))
	if strings.Contains(lane, "* Pinned work") {
		t.Fatalf("Pinned lane row restated its own section header: %q", lane)
	}

	unpinned := &models.Task{ID: "plain", Title: "Plain work", WorkflowState: models.StateInbox}
	quiet := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: unpinned, ProjectID: project.ID,
	}, false, 120, 0, nil))
	if strings.Contains(quiet, "*") {
		t.Fatalf("unpinned row wore the marker: %q", quiet)
	}
}
