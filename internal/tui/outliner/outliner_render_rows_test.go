package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
)

// This file covers the segments a row reserves rather than earns: the context
// badge, the relation and pin markers, the counts a project header carries, and
// the badges a cross-project row must not repeat. Each is reserved at every
// width, which is why these cases assert at narrow widths too. The indicators
// are in outliner_render_indicators_test.go and the width ladder itself in
// outliner_render_rows_pressure_test.go.

func TestTaskContextSegmentIsReservedAndBudgeted(t *testing.T) {
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
	if got := ansi.Strip(segments.contextSegment); got != "[abcdefghijk…] " {
		t.Fatalf("context segment = %q, want a 12-column value budget", got)
	}
	if got := lipgloss.Width(segments.contextSegment); got != 15 {
		t.Fatalf("context segment width = %d, want 15", got)
	}
	if segments.titleWidth(60) != 60-outlinerNonTitleWidth(
		segments.cursorPrefix, segments.checkbox, segments.prefixStr, segments.foldIndicator,
		segments.statusIcon, segments.prioritySegment, segments.pinMarker, segments.contextSegment,
		segments.tagSegment, segments.indicators, segments.relationMarker, segments.matchCue,
	) {
		t.Fatal("context segment drifted between titleWidth and outlinerNonTitleWidth")
	}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 24, 24, 0, nil))
	if !strings.Contains(plain, "[abcde…]") {
		t.Fatalf("narrow row dropped its reserved context badge: %q", plain)
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
		{"both counts fit", 60, "▾ [project] (4)  ! overdue 2 · ◷ today 3"},
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

func TestCrossProjectTaskRowsOmitProjectBadge(t *testing.T) {
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-task", models.StateBacklog))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-task", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	headerLabels := make(map[string]bool)
	for index, row := range outliner.Rows {
		if row.IsProjectHeader {
			headerLabels[row.ProjectID] = strings.Contains(
				ansi.Strip(outliner.RenderOutlinerProjectHeaderRow(row, false, 120, index)),
				"["+row.ProjectID+"]",
			)
			continue
		}
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}

		line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, index, nil))
		if strings.Contains(line, "["+row.ProjectID+"]") {
			t.Errorf("task row %q repeated project badge in %q", row.Task.ID, line)
		}
	}

	for _, projectID := range []string{alpha.ID, beta.ID} {
		if !headerLabels[projectID] {
			t.Errorf("project header %q did not retain its label", projectID)
		}
	}
}

func TestOutlinerTaskRowShowsCompactCommentAndAttachmentCounts(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Title:         "Review compact metadata",
		WorkflowState: models.StateBacklog,
		Comments:      []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:   []models.Attachment{{Filename: "design.pdf"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"▪ 2", "⊕ 1"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("task row missing %q: %q", want, plain)
		}
	}
}

func TestOutlinerTaskRowShowsSemanticTitleWithoutMarkerPrefix(t *testing.T) {
	task := &models.Task{
		ID:            "bug-task",
		Title:         "Fix capture grammar",
		WorkflowState: models.StateBacklog,
		Tags:          []string{"bug", "composer"},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"Fix capture grammar", "#bug", "#composer"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("task row missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, " B Fix capture grammar") {
		t.Fatalf("task row retained retired bug marker prefix: %q", plain)
	}
}

func TestOutlinerTaskRowShowsAncestorMarkerOnlyForPromotedRoot(t *testing.T) {
	task := &models.Task{ID: "child", Title: "Child", WorkflowState: models.StateBacklog}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())

	promoted := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, Breadcrumb: "Parent ›",
	}, false, 120, 120, 0, nil))
	if !strings.Contains(promoted, "Child  ‹") {
		t.Fatalf("promoted task row omitted ancestor marker: %q", promoted)
	}
	if strings.Contains(promoted, "Parent") {
		t.Fatalf("task row spent title width on the ancestor path: %q", promoted)
	}

	nested := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, Prefix: connectorSpace + connectorLastItem,
	}, false, 120, 120, 0, nil))
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
	}, false, 120, 120, 0, nil))
	if !strings.Contains(detached, "Parent  ›2") {
		t.Fatalf("detached parent row omitted the child count: %q", detached)
	}

	combined := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: tasks[1], Breadcrumb: "Parent ›",
	}, false, 120, 120, 0, nil))
	if !strings.Contains(combined, "open-a  ‹") {
		t.Fatalf("promoted child row omitted the ancestor marker: %q", combined)
	}
	if strings.Contains(combined, "›") {
		t.Fatalf("childless row carried a down marker: %q", combined)
	}

	rendered := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: parent, HasVisibleChildren: true,
	}, false, 120, 120, 0, nil))
	if strings.Contains(rendered, "›") {
		t.Fatalf("row showing its children restated parenthood: %q", rendered)
	}

	terminal := &models.Task{ID: "terminal-parent", Title: "Terminal parent"}
	terminalParentID := terminal.ID
	closedChild := &models.Task{ID: "closed", Title: "closed", WorkflowState: models.StateCanceled, ParentID: &terminalParentID}
	closedProject := renderFixtureProject(t, "closed", "closed", "Closed", []*models.Task{terminal, closedChild})
	closedOutliner := NewOutlinerModel(closedProject, nil, newTestConfig())
	closedOutliner.openChildCounts = query.OpenChildCounts([]*models.Project{closedProject})
	quiet := ansi.Strip(closedOutliner.RenderOutlinerTaskRow(OutlinerRow{Task: terminal}, false, 120, 120, 0, nil))
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
	}, false, 120, 120, 0, nil))
	if !strings.Contains(plain, "Mid  ‹›1") {
		t.Fatalf("row missing the combined relation marker: %q", plain)
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
		}, false, width, width, 0, nil))
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
		plain := ansi.Strip(outliner.RenderOutlinerTaskRow(elsewhere, false, width, width, 0, nil))
		if !strings.Contains(plain, "* Pinne") {
			t.Fatalf("width %d dropped the pin marker: %q", width, plain)
		}
		if got := lipgloss.Width(plain); got > width {
			t.Fatalf("width %d row assembled %d columns: %q", width, got, plain)
		}
	}

	lane := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: task, ProjectID: project.ID, IsPinnedSection: true,
	}, false, 120, 120, 0, nil))
	if strings.Contains(lane, "* Pinned work") {
		t.Fatalf("Pinned lane row restated its own section header: %q", lane)
	}

	unpinned := &models.Task{ID: "plain", Title: "Plain work", WorkflowState: models.StateInbox}
	quiet := ansi.Strip(outliner.RenderOutlinerTaskRow(OutlinerRow{
		Task: unpinned, ProjectID: project.ID,
	}, false, 120, 120, 0, nil))
	if strings.Contains(quiet, "*") {
		t.Fatalf("unpinned row wore the marker: %q", quiet)
	}
}
