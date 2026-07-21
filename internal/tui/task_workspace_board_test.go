package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskWorkspaceBoardLayoutContinuityAxesAndDetail(t *testing.T) {
	controller, view, _, _, _ := workspaceViewFixture(t)
	other := TaskIdentity{ProjectID: "p2", TaskID: "other"}
	child := TaskIdentity{ProjectID: "p1", TaskID: "child"}
	if !controller.State.SetCursor(other) || !controller.State.ToggleSelection(child) {
		t.Fatal("failed to seed shared workspace identity state")
	}
	if err := controller.State.SetLayout(WorkspaceBoard); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !controller.HasMode(WorkspaceBoard) || controller.State.BoardAxis != WorkspaceBoardByStatus {
		t.Fatalf("Board mode/default axis = registered:%v axis:%v", controller.HasMode(WorkspaceBoard), controller.State.BoardAxis)
	}
	if cursor, _ := controller.State.Cursor(); cursor != other {
		t.Fatalf("Board switch changed cursor identity: %+v", cursor)
	}
	if !slices.Equal(controller.State.Selected(), []TaskIdentity{child}) {
		t.Fatalf("Board switch changed selection identity: %+v", controller.State.Selected())
	}
	if !view.boardContains(other) || len(view.boardColumns) == 0 {
		t.Fatalf("Board did not distribute the shared snapshot: %+v", view.boardColumns)
	}

	if err := controller.State.SetBoardAxis(WorkspaceBoardByPriority); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	view.moveBoardColumn(1)
	if cursor, _ := controller.State.Cursor(); cursor.TaskID != "parent" {
		t.Fatalf("horizontal Board navigation cursor = %+v, want first P2 card", cursor)
	}
	if !slices.Equal(controller.State.Selected(), []TaskIdentity{child}) {
		t.Fatalf("axis/navigation changed selection identity: %+v", controller.State.Selected())
	}

	controller.State.PreviewVisible = true
	view.SetSize(48, 14)
	rendered := view.View().Content
	for _, line := range strings.Split(rendered, "\n") {
		if width := lipgloss.Width(ansi.Strip(line)); width > 48 {
			t.Fatalf("Board line width = %d, want <= 48: %q", width, ansi.Strip(line))
		}
	}
	if !strings.Contains(ansi.Strip(rendered), "Parent task") || !strings.Contains(ansi.Strip(rendered), "Alpha") {
		t.Fatalf("Board/detail pane omitted focused task context:\n%s", ansi.Strip(rendered))
	}
}

func TestTaskWorkspaceBoardMovesUseOneSharedTargetSet(t *testing.T) {
	controller, view, mutations, _, _ := workspaceViewFixture(t)
	parent := TaskIdentity{ProjectID: "p1", TaskID: "parent"}
	child := TaskIdentity{ProjectID: "p1", TaskID: "child"}
	if err := controller.State.SetLayout(WorkspaceBoard); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	controller.State.SetCursor(parent)
	controller.State.ToggleSelection(parent)
	controller.State.ToggleSelection(child)
	view.syncBoardCursorFromState()

	cmd := view.moveBoardCard(1)
	if cmd == nil || mutations.action != "transition" || mutations.state != models.StateInProgress ||
		!slices.Equal(mutations.ids, []string{"parent", "child"}) {
		t.Fatalf("Status-axis move was not one bulk transition: action=%q state=%s ids=%v", mutations.action, mutations.state, mutations.ids)
	}

	controller.State.ClearSelection()
	controller.State.SetCursor(parent)
	if err := controller.State.SetBoardAxis(WorkspaceBoardByPriority); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	view.syncBoardCursorFromState()
	cmd = view.moveBoardCard(1)
	if cmd == nil || mutations.action != "priority" || mutations.priority != 3 || !slices.Equal(mutations.ids, []string{"parent"}) {
		t.Fatalf("Priority-axis move = action:%q priority:%d ids:%v", mutations.action, mutations.priority, mutations.ids)
	}

	if err := controller.State.SetBoardAxis(WorkspaceBoardBySection); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	view.syncBoardCursorFromState()
	cmd = view.moveBoardCard(-1)
	if cmd == nil || mutations.action != "transition" || mutations.state != models.StateNext {
		t.Fatalf("Section-axis move = action:%q state:%s", mutations.action, mutations.state)
	}
}

func TestTaskWorkspaceBoardAxesExistOnlyInViewOrganize(t *testing.T) {
	controller, view, _, _, _ := workspaceViewFixture(t)
	if err := controller.State.SetLayout(WorkspaceBoard); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	items := view.workspaceMenuItems()
	want := []string{
		"organize:board-axis:status",
		"organize:board-axis:section",
		"organize:board-axis:priority",
		"organize:board-axis:score",
	}
	for _, id := range want {
		index := slices.IndexFunc(items, func(item TaskWorkspaceMenuItem) bool { return item.ID == id })
		if index < 0 || items[index].Section != WorkspaceMenuOrganize || items[index].Apply == nil {
			t.Fatalf("View -> Organize missing Board axis %q", id)
		}
	}
	if err := items[slices.IndexFunc(items, func(item TaskWorkspaceMenuItem) bool {
		return item.ID == "organize:board-axis:score"
	})].Apply(controller.State); err != nil {
		t.Fatal(err)
	}
	if controller.State.BoardAxis != WorkspaceBoardByScore {
		t.Fatalf("Board axis = %v, want Score Buckets", controller.State.BoardAxis)
	}
}
