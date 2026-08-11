package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func outlineTestProject(t *testing.T) *models.Project {
	t.Helper()
	project := &models.Project{ID: "project-1", Name: "Sonar", Slug: "sonar"}
	specs := []struct {
		id      string
		shortID int
		title   string
		parent  string
		state   models.WorkflowState
	}{
		{id: "4f2a9c11aaaa", shortID: 1, title: "Ship auth rewrite", state: models.StateDoing},
		{id: "9b31de07bbbb", shortID: 2, title: "Token refresh path", parent: "4f2a9c11aaaa", state: models.StateBacklog},
		{id: "c0184aa2cccc", shortID: 3, title: "Retry budget", parent: "9b31de07bbbb", state: models.StateBacklog},
		{id: "71f0b3d5dddd", shortID: 4, title: "Sweep dead flags", state: models.StateInbox},
	}
	for _, spec := range specs {
		task, err := models.NewGenericTask(spec.id, spec.shortID, spec.title)
		if err != nil {
			t.Fatalf("new task %s: %v", spec.id, err)
		}
		task.ApplyWorkflowState(spec.state)
		if spec.parent != "" {
			parent := spec.parent
			task.ParentID = &parent
		}
		project.Tasks = append(project.Tasks, task)
	}
	return project
}

func TestOutlineDumpRendersIndentedParentageAndMarkers(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Priority = 1
	due := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	project.Tasks[0].DueDate = &due
	project.Tasks[0].Tags = []string{"epic"}
	project.Tasks[1].Tags = []string{"backend"}

	now := time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC)
	buffer, _, err := dumpOutlineBuffer(project, "Backlog", now)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	want := []string{
		"# tt outline · project: Sonar · mode: Backlog · 2026-08-10 14:02",
		"Ship auth rewrite !1 @2026-08-20 #epic $doing  [[tt:4f2a9c11]]",
		"  Token refresh path #backend $backlog  [[tt:9b31de07]]",
		"    Retry budget $backlog  [[tt:c0184aa2]]",
		"Sweep dead flags $inbox  [[tt:71f0b3d5]]",
	}
	for _, line := range want {
		if !strings.Contains(buffer, line+"\n") {
			t.Fatalf("dump missing %q:\n%s", line, buffer)
		}
	}
}

// TestOutlineDumpIsDeterministic keeps the buffer a stable function of the
// project, so an unedited round trip can never report a change nobody made.
func TestOutlineDumpIsDeterministic(t *testing.T) {
	project := outlineTestProject(t)
	now := time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC)
	first, _, err := dumpOutlineBuffer(project, "All", now)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	for range 5 {
		again, _, err := dumpOutlineBuffer(project, "All", now)
		if err != nil {
			t.Fatalf("dump: %v", err)
		}
		if again != first {
			t.Fatalf("dump is not deterministic:\n%s\n---\n%s", first, again)
		}
	}
}

func TestOutlineDumpRefusesCollidingMarkers(t *testing.T) {
	project := outlineTestProject(t)
	clash, err := models.NewGenericTask("4f2a9c11eeee", 5, "Colliding task")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = append(project.Tasks, clash)

	if _, _, err := dumpOutlineBuffer(project, "All", time.Now()); err == nil {
		t.Fatal("dump accepted a project whose short markers collide")
	} else if !strings.Contains(err.Error(), "share the marker") {
		t.Fatalf("collision refusal reads %q", err.Error())
	}
}

// TestOutlineRoundTripOfAnUneditedBufferIsFaithful drives the dump straight back
// through the parser: the buffer is declarative, so every value it prints must
// parse back into the same plan line.
func TestOutlineRoundTripOfAnUneditedBufferIsFaithful(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Priority = 2
	project.Tasks[0].Tags = []string{"epic", "auth"}
	// The due date is a calendar day, so the round trip has to be faithful at the
	// instant the parser mints for that day; anything else would report an edit
	// nobody made.
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	project.Tasks[0].DueDate = &due

	buffer, _, err := dumpOutlineBuffer(project, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	lines, problems := parseOutlineBuffer(buffer, project)
	if len(problems) > 0 {
		t.Fatalf("unedited buffer was rejected: %v", problems)
	}
	if len(lines) != len(project.Tasks) {
		t.Fatalf("plan holds %d lines, want %d", len(lines), len(project.Tasks))
	}
	for i, line := range lines {
		task := project.Tasks[i]
		if line.TaskID != task.ID {
			t.Errorf("line %d names %s, want %s", i, line.TaskID, task.ID)
		}
		if line.Title != task.Title {
			t.Errorf("line %d title = %q, want %q", i, line.Title, task.Title)
		}
		if line.Priority != task.Priority {
			t.Errorf("line %d priority = %d, want %d", i, line.Priority, task.Priority)
		}
		if string(line.Destination) == "" {
			t.Errorf("line %d dropped its destination", i)
		}
	}
	if lines[0].Due == nil || !lines[0].Due.Equal(due) {
		t.Errorf("round trip lost the due date: %v", lines[0].Due)
	}
	if strings.Join(lines[0].Tags, ",") != "epic,auth" {
		t.Errorf("round trip tags = %v, want [epic auth]", lines[0].Tags)
	}
	if lines[1].Depth != 1 || lines[2].Depth != 2 {
		t.Errorf("round trip depths = %d/%d, want 1/2", lines[1].Depth, lines[2].Depth)
	}
}

func TestOutlineParseBuildsPlanLinesForNewAndEditedTasks(t *testing.T) {
	project := outlineTestProject(t)
	buffer := strings.Join([]string{
		"# comment",
		"",
		"Ship auth rewrite !1 $doing  [[tt:4f2a9c11]]",
		"  Rate-limit headers $backlog",
		"Sweep dead flags  [[tt:71f0b3d5]]",
	}, "\n")

	lines, problems := parseOutlineBuffer(buffer, project)
	if len(problems) > 0 {
		t.Fatalf("buffer was rejected: %v", problems)
	}
	if len(lines) != 3 {
		t.Fatalf("plan holds %d lines, want 3", len(lines))
	}
	if lines[0].SourceLine != 3 || lines[1].SourceLine != 4 || lines[2].SourceLine != 5 {
		t.Errorf("source lines = %d/%d/%d, want 3/4/5", lines[0].SourceLine, lines[1].SourceLine, lines[2].SourceLine)
	}
	if lines[0].TaskID != "4f2a9c11aaaa" {
		t.Errorf("marker resolved to %q", lines[0].TaskID)
	}
	if lines[1].TaskID != "" {
		t.Errorf("markerless line claimed task %q instead of creating one", lines[1].TaskID)
	}
	if lines[1].Depth != 1 {
		t.Errorf("new child depth = %d, want 1", lines[1].Depth)
	}
	if lines[0].Priority != 1 {
		t.Errorf("priority = %d, want 1", lines[0].Priority)
	}
	// An omitted destination stays empty so the application layer preserves the
	// state the task already holds instead of laundering a transition.
	if lines[2].Destination != "" {
		t.Errorf("omitted destination became %q", lines[2].Destination)
	}
	if lines[0].Destination != app.CaptureDestinationDoing {
		t.Errorf("destination = %q, want doing", lines[0].Destination)
	}
}

func TestOutlineParseRejectsEveryOutlinePhaseError(t *testing.T) {
	project := outlineTestProject(t)
	tests := []struct {
		name    string
		buffer  string
		line    int
		message string
	}{
		{
			name:    "odd indent",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n   Token refresh path  [[tt:9b31de07]]",
			line:    2,
			message: "indent of 3 spaces is not a multiple of 2",
		},
		{
			name:    "tab indent",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n\tToken refresh path",
			line:    2,
			message: "indent with spaces, not tabs",
		},
		{
			name:    "indent jump",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n    Retry budget",
			line:    2,
			message: "indent jumps from level 0 to level 2; indent one level at a time",
		},
		{
			name:    "unknown marker",
			buffer:  "Ship auth rewrite  [[tt:deadbeef]]",
			line:    1,
			message: "marker [[tt:deadbeef]] does not name a task in this project",
		},
		{
			name:    "duplicate marker",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\nShip auth rewrite again  [[tt:4f2a9c11]]",
			line:    2,
			message: "marker [[tt:4f2a9c11]] already claimed on line 1",
		},
		{
			name:    "malformed marker",
			buffer:  "Ship auth rewrite  [[tt:]]",
			line:    1,
			message: `marker "[[tt:]]" is malformed`,
		},
		{
			name:    "marker suffix without prefix",
			buffer:  "Ship auth rewrite]]",
			line:    1,
			message: "a line ending in ]] must carry a [[tt:…]] marker",
		},
		{
			name:    "orphaned by a rejected parent",
			buffer:  "Ship auth rewrite  [[tt:deadbeef]]\n  Token refresh path  [[tt:9b31de07]]",
			line:    2,
			message: "its parent on line 1 was rejected, so it has nowhere to attach",
		},
		{
			name:    "over the nesting cap",
			buffer:  outlineDeepBuffer(models.MaxNestingDepth + 1),
			line:    models.MaxNestingDepth + 2,
			message: fmt.Sprintf("nests deeper than the maximum of %d levels", models.MaxNestingDepth),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, problems := parseOutlineBuffer(tt.buffer, project)
			if lines != nil {
				t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
			}
			if !outlineProblemsContain(problems, tt.line, tt.message) {
				t.Fatalf("problems %v do not name line %d: %s", problems, tt.line, tt.message)
			}
		})
	}
}

func TestOutlineParseRejectsEveryLinePhaseError(t *testing.T) {
	project := outlineTestProject(t)
	tests := []struct {
		name    string
		buffer  string
		line    int
		message string
	}{
		{
			name:    "empty title",
			buffer:  "!1 $doing  [[tt:4f2a9c11]]",
			line:    1,
			message: "task title cannot be empty",
		},
		{
			name:    "parent sigil",
			buffer:  "Ship auth rewrite >2  [[tt:4f2a9c11]]",
			line:    1,
			message: "indentation sets parentage here; remove the >parent sigil",
		},
		{
			name:   "unknown destination",
			buffer: "Ship auth rewrite $ready  [[tt:4f2a9c11]]",
			line:   1,
		},
		{
			name:   "retired priority",
			buffer: "Ship auth rewrite !5  [[tt:4f2a9c11]]",
			line:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, problems := parseOutlineBuffer(tt.buffer, project)
			if lines != nil {
				t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
			}
			if len(problems) == 0 {
				t.Fatal("the buffer was accepted")
			}
			if tt.message != "" && !outlineProblemsContain(problems, tt.line, tt.message) {
				t.Fatalf("problems %v do not name line %d: %s", problems, tt.line, tt.message)
			}
			if problems[0].line != tt.line {
				t.Fatalf("first problem names line %d, want %d", problems[0].line, tt.line)
			}
		})
	}
}

// TestOutlineParseCollectsEveryProblemInOnePass is the contract that makes the
// error surface worth having: a buffer wrong in several places reports all of
// them, in line order, instead of one reopen per mistake.
func TestOutlineParseCollectsEveryProblemInOnePass(t *testing.T) {
	project := outlineTestProject(t)
	buffer := strings.Join([]string{
		"Ship auth rewrite  [[tt:4f2a9c11]]",
		"   Token refresh path  [[tt:9b31de07]]",
		"!1  [[tt:71f0b3d5]]",
		"Sweep dead flags again  [[tt:71f0b3d5]]",
	}, "\n")

	lines, problems := parseOutlineBuffer(buffer, project)
	if lines != nil {
		t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
	}
	if len(problems) != 3 {
		t.Fatalf("collected %d problems, want 3: %v", len(problems), problems)
	}
	for i := 1; i < len(problems); i++ {
		if problems[i-1].line > problems[i].line {
			t.Fatalf("problems are not in line order: %v", problems)
		}
	}
}

func TestOutlineMarkerStripLeavesCaptureText(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		marker string
		rest   string
		fails  bool
	}{
		{name: "trailing marker", body: "Ship auth rewrite !1  [[tt:4f2a9c11]]", marker: "4f2a9c11", rest: "Ship auth rewrite !1"},
		{name: "no marker", body: "Rate-limit headers $backlog", marker: "", rest: "Rate-limit headers $backlog"},
		{name: "brackets in title", body: "Fix [WIP] parser  [[tt:4f2a9c11]]", marker: "4f2a9c11", rest: "Fix [WIP] parser"},
		{name: "bracketed title without marker", body: "Fix parser", marker: "", rest: "Fix parser"},
		{name: "suffix without prefix", body: "Fix parser]]", fails: true},
		{name: "marker holding a space", body: "Fix parser  [[tt:4f2a 9c11]]", fails: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, rest, err := splitOutlineMarker(tt.body)
			if tt.fails {
				if err == nil {
					t.Fatalf("split accepted %q", tt.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("split: %v", err)
			}
			if marker != tt.marker {
				t.Errorf("marker = %q, want %q", marker, tt.marker)
			}
			if rest != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

// TestOutlineMarkerIndexLeavesAmbiguousShortMarkersUnresolved keeps a
// hand-authored buffer honest: the dump refuses to print colliding markers, so a
// short marker naming two tasks must be rejected rather than resolved by luck.
func TestOutlineMarkerIndexLeavesAmbiguousShortMarkersUnresolved(t *testing.T) {
	project := outlineTestProject(t)
	clash, err := models.NewGenericTask("4f2a9c11eeee", 5, "Colliding task")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = append(project.Tasks, clash)

	index := outlineMarkerIndex(project)
	if resolved, ok := index["4f2a9c11"]; ok {
		t.Fatalf("ambiguous short marker resolved to %s", resolved)
	}
	if index["4f2a9c11aaaa"] != "4f2a9c11aaaa" || index["4f2a9c11eeee"] != "4f2a9c11eeee" {
		t.Fatal("full task IDs must stay addressable when their short markers collide")
	}
}

func TestOutlineApplyErrorReusesTheApplicationLineNumber(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		line    int
		message string
	}{
		{
			name:    "prefixed",
			err:     fmt.Errorf("line 7: title: task title cannot be empty"),
			line:    7,
			message: "title: task title cannot be empty",
		},
		{
			name:    "unprefixed",
			err:     fmt.Errorf("project not found: project-9"),
			line:    1,
			message: "project not found: project-9",
		},
		{
			name:    "unparseable number",
			err:     fmt.Errorf("line seven: broken"),
			line:    1,
			message: "line seven: broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := outlineApplyError(tt.err)
			if problem.line != tt.line || problem.message != tt.message {
				t.Fatalf("got line %d %q, want line %d %q", problem.line, problem.message, tt.line, tt.message)
			}
		})
	}
}

// TestOutlineDumpScopeExcludesAnOrphanedTask is why scope comes from the dump
// walk instead of project.Tasks. A task whose ParentID names nothing is neither
// a root nor anyone's child, so it never reaches the buffer — and absence
// deletes now, so a scope built from project.Tasks would delete a line the user
// was never shown.
func TestOutlineDumpScopeExcludesAnOrphanedTask(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks = append(project.Tasks, nil)
	orphan, err := models.NewGenericTask("dd44ee55ffff", 9, "Orphaned by a dangling parent")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	missing := "no-such-task"
	orphan.ParentID = &missing
	project.Tasks = append(project.Tasks, orphan)

	buffer, scope, err := dumpOutlineBuffer(project, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if len(scope) != 4 {
		t.Fatalf("scope holds %d ids, want 4: %v", len(scope), scope)
	}
	for _, id := range scope {
		if id == orphan.ID {
			t.Fatalf("scope named the orphan the buffer never printed: %v", scope)
		}
	}
	if strings.Contains(buffer, "Orphaned by a dangling parent") {
		t.Fatalf("dump printed an unreachable task:\n%s", buffer)
	}

	lines, problems := parseOutlineBuffer(buffer, project)
	if len(problems) > 0 {
		t.Fatalf("unedited buffer was rejected: %v", problems)
	}
	if deleted := outlineDeletedTasks(project, scope, lines); len(deleted) != 0 {
		t.Fatalf("an unedited buffer asked to delete %d task(s)", len(deleted))
	}
}

// TestOutlineEditorRequestRefusesFixedOrderModes is the mode guard. Today, Next,
// Due, and Recent promote rows across section boundaries, so indentation there
// is not parentage.
func TestOutlineEditorRequestRefusesFixedOrderModes(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	outliner := app.activeOutliner()
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}

	for _, mode := range OutlinerModeCatalog() {
		outliner.primaryMode = mode.ID
		project, label, refusal := app.outlineEditorRequest()
		if mode.FixedOrder() {
			if refusal != outlineEditorModeRefusal {
				t.Errorf("%s refusal = %q, want %q", mode.Label, refusal, outlineEditorModeRefusal)
			}
			if project != nil {
				t.Errorf("%s named a project despite refusing", mode.Label)
			}
			continue
		}
		if refusal != "" {
			t.Errorf("%s refused with %q", mode.Label, refusal)
		}
		if label != mode.Label {
			t.Errorf("%s reported label %q", mode.Label, label)
		}
	}
}

// TestOutlineEditorIsInapplicableUnderVisualSelection matches the existing
// structural-operation rule: a selection claims a narrower scope than the whole
// project group the buffer would rewrite.
func TestOutlineEditorIsInapplicableUnderVisualSelection(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	descriptor, ok := app.GetOperationRegistry().Descriptor(operationCoreOutlineEditor)
	if !ok {
		t.Fatal("tui.core.outline-editor is not registered")
	}

	ctx := app.operationCommandContext()
	ctx.View = ViewCrossProjectOutliner
	if !descriptor.Applicable(ctx) {
		t.Fatal("outline edit is inapplicable in the plain Outliner")
	}
	ctx.Selection.Visual = true
	if descriptor.Applicable(ctx) {
		t.Fatal("outline edit stayed applicable under a visual selection")
	}
}

// TestOutlineEditorRequestReadsTheProjectUnderEveryRowKind is why `O` is
// classified target-free rather than rejecting on a project header: the buffer's
// scope is the project the cursor sits in, and a header names one as plainly as
// a task row does.
func TestOutlineEditorRequestReadsTheProjectUnderEveryRowKind(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	outliner := app.activeOutliner()
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}

	header, task := -1, -1
	for index, row := range outliner.rows {
		if row.IsProjectHeader && header < 0 {
			header = index
		}
		if row.isTaskRow() && task < 0 {
			task = index
		}
	}
	if header < 0 || task < 0 {
		t.Fatalf("fixture exposed header=%d task=%d", header, task)
	}

	outliner.cursor = header
	fromHeader, _, refusal := app.outlineEditorRequest()
	if refusal != "" {
		t.Fatalf("a project header refused with %q", refusal)
	}
	if fromHeader == nil {
		t.Fatal("a project header named no project")
	}
	if fromHeader.ID != outliner.rows[header].ProjectID {
		t.Errorf("header resolved %s, want %s", fromHeader.ID, outliner.rows[header].ProjectID)
	}

	outliner.cursor = task
	fromTask, _, refusal := app.outlineEditorRequest()
	if refusal != "" {
		t.Fatalf("a task row refused with %q", refusal)
	}
	if fromTask == nil || fromTask.ID != outliner.rows[task].ProjectID {
		t.Errorf("task row resolved %v, want %s", fromTask, outliner.rows[task].ProjectID)
	}
}

// TestOutlineEditorDumpsOneSnapshotWhenTheCursorProjectIsStale is why `O`
// reloads instead of dumping the row's own project pointer: the shell's copy can
// be minutes old, and pairing it with a freshly read revision would claim the
// buffer describes a tree it never saw.
func TestOutlineEditorDumpsOneSnapshotWhenTheCursorProjectIsStale(t *testing.T) {
	stale := outlineTestProject(t)
	gone, err := models.NewGenericTask("aa11bb22cccc", 5, "Removed upstream")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	stale.Tasks = append(stale.Tasks, gone)

	fresh := outlineTestProject(t)
	fresh.Tasks[0].Title = "Ship auth rewrite, reworded upstream"

	stub := &stubMutator{project: fresh, result: app.TaskResult{Revision: "rev-7"}}
	model := &AppModel{features: newFeatureCoordinators(stub)}

	project, revision, buffer, scope, err := model.outlineDumpSource(stale.ID, "Backlog")
	if err != nil {
		t.Fatalf("outlineDumpSource: %v", err)
	}
	if len(scope) != len(fresh.Tasks) {
		t.Errorf("scope holds %d ids, want the %d tasks the snapshot dumped", len(scope), len(fresh.Tasks))
	}
	if project != fresh {
		t.Fatalf("dump project = %v, want the snapshot project", project)
	}
	if revision != "rev-7" {
		t.Errorf("revision = %q, want the one read beside the project", revision)
	}
	if !strings.Contains(buffer, "Ship auth rewrite, reworded upstream") {
		t.Errorf("buffer missing the current title:\n%s", buffer)
	}
	if strings.Contains(buffer, "Removed upstream") {
		t.Errorf("buffer dumped a task only the stale cursor project held:\n%s", buffer)
	}
}

// TestOutlineEditedCarriesTheDumpRevisionAndKeepsTheBufferWhenItIsStale covers
// the other end of the round trip: the revision travels into the apply, and a
// writer that landed while the editor was open is reported as a kept buffer
// rather than a line-numbered parse failure.
func TestOutlineEditedCarriesTheDumpRevisionAndKeepsTheBufferWhenItIsStale(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	model := driver.App()

	project := outlineTestProject(t)
	buffer, scope, err := dumpOutlineBuffer(project, "Backlog", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	applied := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(applied)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Buffer: buffer,
	})
	if applied.outline.ExpectedRevision != "rev-1" {
		t.Fatalf("ExpectedRevision = %q, want the dump revision", applied.outline.ExpectedRevision)
	}

	conflicted := &stubMutator{err: &storage.RevisionConflictError{
		ProjectID: project.ID, ProjectName: project.Name, Expected: "rev-1", Actual: "rev-2",
	}}
	model.features = newFeatureCoordinators(conflicted)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Path: "/tmp/tt-outline-stale.md", Buffer: buffer,
	})
	if !strings.Contains(model.statusMessage, "Workspace changed while the editor was open") {
		t.Errorf("status = %q, want the stale-workspace refusal", model.statusMessage)
	}
	if !strings.Contains(model.statusMessage, "/tmp/tt-outline-stale.md") {
		t.Errorf("status = %q, want the kept buffer named", model.statusMessage)
	}
	if model.currentView() == ViewOutlineErrors {
		t.Error("a stale workspace opened the line-error surface")
	}
}

// TestOutlineEditedArmsTheDeleteConfirmBeforeApplying is the guard the whole
// feature rests on: a buffer that dropped a line must reach a prompt, never the
// transaction.
func TestOutlineEditedArmsTheDeleteConfirmBeforeApplying(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: outlineBufferWithoutLine(t, buffer, "Sweep dead flags"),
	})

	if stub.calls != 0 {
		t.Fatalf("the apply ran before the prompt (%d calls)", stub.calls)
	}
	if !model.outlineDeleteConfirm.IsActive() {
		t.Fatal("a removed line did not arm the confirmation")
	}
	if got, want := model.outlineDeleteConfirm.Prompt(), "Delete task #4 from Sonar?"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
	detail := model.outlineDeleteConfirm.detail
	if detail == nil || detail.TotalCount != 1 || !detail.UndoHint {
		t.Fatalf("detail = %+v, want one task with the undo hint", detail)
	}
	if len(detail.Items) != 1 || !strings.Contains(detail.Items[0], "Sweep dead flags") {
		t.Errorf("detail items = %v, want the removed task named", detail.Items)
	}
	if !strings.Contains(detail.CascadeNote, "1 task deleted") {
		t.Errorf("cascade note = %q, want the deletion count", detail.CascadeNote)
	}
	if !strings.Contains(detail.CascadeNote, "attachments:") {
		t.Errorf("cascade note = %q, want the attachment consequence", detail.CascadeNote)
	}
}

// TestOutlineDeleteConfirmDeclineKeepsTheBufferAndChangesNothing is the reason
// declining cancels the whole apply: a buffer that half landed would leave the
// user guessing which half, so the file stays and nothing commits.
func TestOutlineDeleteConfirmDeclineKeepsTheBufferAndChangesNothing(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	file, err := os.CreateTemp(t.TempDir(), "tt-outline-*.md")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	path := file.Name()
	_ = file.Close()

	edited := outlineBufferWithoutLine(t, buffer, "Sweep dead flags")
	edited = strings.Replace(edited, "Ship auth rewrite", "Ship auth rewrite v2", 1)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Path: path, Buffer: edited,
	})

	handled, _ := model.handleOutlineDeleteConfirmKey("esc")
	if !handled {
		t.Fatal("esc was not consumed by the confirmation")
	}
	if stub.calls != 0 {
		t.Fatalf("a declined confirmation still applied the buffer (%d calls)", stub.calls)
	}
	if model.outlineDeleteConfirm.IsActive() {
		t.Error("the confirmation survived the decline")
	}
	if model.pendingOutlineApply.project != nil {
		t.Error("the pending apply survived the decline")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the declined buffer was removed: %v", err)
	}
	if !strings.Contains(model.statusMessage, path) {
		t.Errorf("status = %q, want the kept buffer named", model.statusMessage)
	}
}

// TestOutlineDeleteConfirmCommitsEditsAndDeletesTogether is what keeps the
// removal in the same transaction and the same undo entry as the edits beside
// it: one request carries both.
func TestOutlineDeleteConfirmCommitsEditsAndDeletesTogether(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	edited := outlineBufferWithoutLine(t, buffer, "Sweep dead flags")
	edited = strings.Replace(edited, "Ship auth rewrite", "Ship auth rewrite v2", 1)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Buffer: edited,
	})
	if _, _ = model.handleOutlineDeleteConfirmKey("y"); stub.calls != 1 {
		t.Fatalf("the confirmed apply ran %d times, want 1", stub.calls)
	}

	req := stub.outline
	if req.ExpectedRevision != "rev-1" {
		t.Errorf("ExpectedRevision = %q, want the dump revision", req.ExpectedRevision)
	}
	if strings.Join(req.ScopeTaskIDs, ",") != strings.Join(scope, ",") {
		t.Errorf("ScopeTaskIDs = %v, want the dumped ids %v", req.ScopeTaskIDs, scope)
	}
	if len(req.DeleteTaskIDs) != 1 || req.DeleteTaskIDs[0] != "71f0b3d5dddd" {
		t.Errorf("DeleteTaskIDs = %v, want the removed task", req.DeleteTaskIDs)
	}
	if !req.ConfirmDeletions {
		t.Error("the confirmed request did not record destructive intent")
	}
	if len(req.Lines) != 3 {
		t.Fatalf("the request carries %d lines, want the 3 that survived", len(req.Lines))
	}
	if req.Lines[0].Title != "Ship auth rewrite v2" {
		t.Errorf("the edit beside the deletion was lost: %q", req.Lines[0].Title)
	}
	if model.outlineDeleteConfirm.IsActive() || model.pendingOutlineApply.project != nil {
		t.Error("the pending apply outlived the commit")
	}
}

// TestOutlineEditedAppliesWithoutAConfirmWhenNothingIsAbsent keeps the prompt
// proportional: the confirmation exists for deletions, so an ordinary edit must
// not grow one.
func TestOutlineEditedAppliesWithoutAConfirmWhenNothingIsAbsent(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: strings.Replace(buffer, "Retry budget", "Retry budget v2", 1),
	})

	if model.outlineDeleteConfirm.IsActive() {
		t.Fatal("a buffer that deleted nothing armed the confirmation")
	}
	if stub.calls != 1 {
		t.Fatalf("the apply ran %d times, want 1", stub.calls)
	}
	if len(stub.outline.DeleteTaskIDs) != 0 || stub.outline.ConfirmDeletions {
		t.Errorf("an ordinary edit carried destructive intent: %+v", stub.outline)
	}
}

// TestOutlineEditedEmptiedBufferDeletesEveryTask is the case emptiness used to
// swallow: once absence deletes, a buffer with every line removed asks for the
// whole project, which is the one request the old syntax guard could not express.
func TestOutlineEditedEmptiedBufferDeletesEveryTask(t *testing.T) {
	model, project, _, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: "# every line was removed\n\n",
	})

	if model.currentView() == ViewOutlineErrors {
		t.Fatal("an emptied buffer was still rejected as a syntax error")
	}
	if !model.outlineDeleteConfirm.IsActive() {
		t.Fatal("an emptied buffer did not arm the confirmation")
	}
	if got, want := model.outlineDeleteConfirm.Prompt(), "Delete 4 tasks from Sonar?"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}

	if _, _ = model.handleOutlineDeleteConfirmKey("y"); stub.calls != 1 {
		t.Fatalf("the confirmed apply ran %d times, want 1", stub.calls)
	}
	req := stub.outline
	if len(req.Lines) != 0 {
		t.Errorf("the request carries %d lines, want none", len(req.Lines))
	}
	if strings.Join(req.DeleteTaskIDs, ",") != strings.Join(scope, ",") {
		t.Errorf("DeleteTaskIDs = %v, want every dumped id %v", req.DeleteTaskIDs, scope)
	}
	if !req.ConfirmDeletions {
		t.Error("the confirmed request did not record destructive intent")
	}
}

// TestOutlineEditedRefusesAnEmptyBufferThatDeletesNothing keeps the truncation
// accident refused: a buffer holding no lines over a scope that held no tasks
// asks for no change at all, so it is far likelier to be a lost file.
func TestOutlineEditedRefusesAnEmptyBufferThatDeletesNothing(t *testing.T) {
	model, _, _, _ := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = newFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project:  &models.Project{ID: "project-2", Name: "Empty", Slug: "empty"},
		Revision: "rev-1",
		Buffer:   "# only a comment\n\n",
	})

	if model.currentView() != ViewOutlineErrors {
		t.Fatalf("view = %v, want the errors surface", model.currentView())
	}
	if model.outlineDeleteConfirm.IsActive() {
		t.Error("a buffer that deletes nothing armed the confirmation")
	}
	if stub.calls != 0 {
		t.Errorf("the empty buffer was applied (%d calls)", stub.calls)
	}
}

func outlineConfirmFixture(t *testing.T) (*AppModel, *models.Project, string, []string) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project := outlineTestProject(t)
	buffer, scope, err := dumpOutlineBuffer(project, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	return driver.App(), project, buffer, scope
}

func outlineBufferWithoutLine(t *testing.T, buffer, needle string) string {
	t.Helper()
	kept := make([]string, 0, strings.Count(buffer, "\n")+1)
	dropped := 0
	for _, line := range strings.Split(buffer, "\n") {
		if strings.Contains(line, needle) {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	if dropped != 1 {
		t.Fatalf("dropped %d lines matching %q, want 1", dropped, needle)
	}
	return strings.Join(kept, "\n")
}

func outlineDeepBuffer(depth int) string {
	lines := make([]string, 0, depth+1)
	for level := 0; level <= depth; level++ {
		lines = append(lines, strings.Repeat(" ", level*outlineIndentWidth)+fmt.Sprintf("Level %d", level))
	}
	return strings.Join(lines, "\n")
}

func outlineProblemsContain(problems []lineError, line int, message string) bool {
	for _, problem := range problems {
		if problem.line == line && problem.message == message {
			return true
		}
	}
	return false
}
