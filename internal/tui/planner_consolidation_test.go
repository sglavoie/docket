package tui

import (
	"testing"
)

func suggestedCommandNames(palette *CommandPaletteModel) []string {
	var names []string
	for _, item := range palette.filteredItems {
		if item.IsSuggested && item.Item.Command != nil {
			names = append(names, item.Item.Command.Name)
		}
	}
	return names
}

func TestCompletedWorkRemainsReachableOutsidePlanner(t *testing.T) {
	if perspective, ok := OutlinerCompleted.perspectiveID(); !ok || perspective != PerspectiveCompleted {
		t.Fatalf("Completed focus = perspective %d/%v, want %d", perspective, ok, PerspectiveCompleted)
	}

	// Activity is an Outliner computed mode: its legacy presentation resolves to
	// the global Task Workspace rather than a standalone review route.
	route, ok, err := routeForPresentation(ViewRecentActivity, "", "")
	if err != nil || !ok {
		t.Fatalf("Review Activity route: ok=%v err=%v", ok, err)
	}
	params, ok := route.Params().(TaskWorkspaceParams)
	if !ok || route.ID() != TaskWorkspace || params.Scope != AppEntityScope() {
		t.Fatalf("Recent Activity route = %s %#v, want global TaskWorkspace", route.ID(), route.Params())
	}
}
