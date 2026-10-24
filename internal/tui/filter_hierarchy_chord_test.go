package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/filterops"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
)

// newHierarchyFilterTestApp builds one project covering every hierarchy
// position, including a parent whose only child is done.
func newHierarchyFilterTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	epicID := "epic"
	storyID := "story"
	closedID := "closed-parent"
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: epicID, ShortID: 1, Title: "Epic", WorkflowState: models.StateBacklog},
			{ID: storyID, ShortID: 2, Title: "Story", WorkflowState: models.StateBacklog, ParentID: &epicID},
			{ID: "open-leaf", ShortID: 3, Title: "Open leaf", WorkflowState: models.StateBacklog, ParentID: &storyID},
			{ID: "done-leaf", ShortID: 4, Title: "Done leaf", WorkflowState: models.StateDone, ParentID: &storyID},
			{ID: closedID, ShortID: 5, Title: "Closed parent", WorkflowState: models.StateBacklog},
			{ID: "only-done-child", ShortID: 6, Title: "Only done child", WorkflowState: models.StateDone, ParentID: &closedID},
			{ID: "solo", ShortID: 7, Title: "Solo", WorkflowState: models.StateBacklog},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

func TestHierarchyFilterBadgeCountAndClear(t *testing.T) {
	app := newHierarchyFilterTestApp(t)
	filter := &app.crossOutliner.Filter.CommonFilterState

	result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationFilterHierarchy,
		Context:  simplifiedOperationContext(),
		Value:    query.HierarchyLeaf,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("hierarchy filter error: %v", result.Err)
	}
	if got := query.CommonFilterParts(filter); !slices.Equal(got, []string{"?leaf"}) {
		t.Errorf("badge parts = %v, want the hierarchy facet alone", got)
	}
	if got := query.CommonFilterFacetCount(filter); got != 1 {
		t.Errorf("facet count = %d, want 1", got)
	}

	pressFilterChord(t, app, "fx")
	if filter.Hierarchy != query.HierarchyAll {
		t.Errorf("fx left hierarchy = %v, want HierarchyAll", filter.Hierarchy)
	}
}

// TestFilterOperationsStayChordOnly locks the direct-grammar contract: the
// filter family refines the view in place through the f chords.
func TestFilterOperationsStayChordOnly(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()

	descriptors := filterops.FilterOperationDescriptors()
	if len(descriptors) != 14 {
		t.Fatalf("filter family declares %d operations, want 14", len(descriptors))
	}
	for _, declared := range descriptors {
		_, ok := registry.Descriptor(declared.ID)
		if !ok {
			t.Fatalf("filter operation %s is not registered", declared.ID)
		}
	}
}

func TestHierarchyFilterReadsOpenChildCensus(t *testing.T) {
	for _, tc := range []struct {
		value query.HierarchyFilter
		want  map[string]bool
	}{
		{
			value: query.HierarchyParent,
			want:  map[string]bool{"epic": true, "story": true},
		},
		{
			value: query.HierarchyChild,
			want:  map[string]bool{"story": true, "open-leaf": true},
		},
		{
			value: query.HierarchyRoot,
			want:  map[string]bool{"epic": true, "closed-parent": true, "solo": true},
		},
		{
			value: query.HierarchyLeaf,
			want:  map[string]bool{"open-leaf": true, "closed-parent": true, "solo": true},
		},
	} {
		t.Run(outliner.HierarchyFilterLabel(tc.value), func(t *testing.T) {
			app := newHierarchyFilterTestApp(t)
			result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
				ID:       operation.OperationFilterHierarchy,
				Context:  simplifiedOperationContext(),
				Value:    tc.value,
				HasValue: true,
			})
			if result.Err != nil {
				t.Fatalf("hierarchy filter error: %v", result.Err)
			}
			want := "Filtering hierarchy: " + outliner.HierarchyFilterLabel(tc.value)
			if result.Status != want {
				t.Errorf("Status = %q, want %q", result.Status, want)
			}
			if got := visibleOutlinerTaskIDs(app.crossOutliner); !maps.Equal(got, tc.want) {
				t.Errorf("visible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHierarchyFilterRejectsOutOfRangeValues(t *testing.T) {
	app := newHierarchyFilterTestApp(t)
	for _, value := range []query.HierarchyFilter{query.HierarchyAll, query.HierarchyLeaf + 1} {
		result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID:       operation.OperationFilterHierarchy,
			Context:  simplifiedOperationContext(),
			Value:    value,
			HasValue: true,
		})
		if result.Err == nil {
			t.Errorf("hierarchy value %d should be rejected", value)
		}
	}
}

func TestPriorityThresholdSemantics(t *testing.T) {
	tasks := map[int]*models.Task{
		0: {ID: "p0", Title: "Unset", WorkflowState: models.StateBacklog, Priority: 0},
		1: {ID: "p1", Title: "P1", WorkflowState: models.StateBacklog, Priority: 1},
		2: {ID: "p2", Title: "P2", WorkflowState: models.StateBacklog, Priority: 2},
		3: {ID: "p3", Title: "P3", WorkflowState: models.StateBacklog, Priority: 3},
	}
	for threshold, want := range map[int]map[int]bool{
		0: {0: true, 1: true, 2: true, 3: true},
		1: {0: false, 1: true, 2: false, 3: false},
		2: {0: false, 1: true, 2: true, 3: false},
		3: {0: false, 1: true, 2: true, 3: true},
	} {
		filter := query.NewCommonFilterState()
		filter.Priority = threshold
		for priority, pass := range want {
			if got := passesCommonFilters(tasks[priority], &filter); got != pass {
				t.Errorf("threshold %d priority %d = %v, want %v", threshold, priority, got, pass)
			}
		}
	}

	filter := query.NewCommonFilterState()
	filter.Priority = 3
	badge := query.CommonFilterParts(&filter)
	if len(badge) != 1 || badge[0] != "!3+" {
		t.Errorf("badge parts = %v, want [!3+]", badge)
	}
	if described := fmt.Sprint(badge); !strings.Contains(described, "!3+") {
		t.Errorf("badge %q should describe the threshold", described)
	}
}
