package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSectionRowForestUsesEarliestMemberRankAndSourceSiblingOrder(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	firstChild := primaryRowsTask("first-child", models.StateBacklog)
	firstChild.ParentID = &parent.ID
	secondChild := primaryRowsTask("second-child", models.StateBacklog)
	secondChild.ParentID = &parent.ID
	unrelated := primaryRowsTask("unrelated", models.StateBacklog)
	project := primaryRowsProject("project", parent, firstChild, secondChild, unrelated)
	outliner := NewOutlinerModel(project, nil, nil)

	forest := outliner.buildSectionRowForestWith([]OutlinerRow{
		{Task: secondChild},
		{Task: unrelated},
		{Task: firstChild},
		{Task: parent},
	}, sectionForestOptions{rankRootsBySubtree: true})
	var rows []OutlinerRow
	outliner.flattenSectionRowForest(forest, &rows)

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Task.ID)
	}
	if want := []string{parent.ID, secondChild.ID, firstChild.ID, unrelated.ID}; !slices.Equal(got, want) {
		t.Fatalf("flattened section = %v, want earliest subtree and source sibling order %v", got, want)
	}
	if !rows[0].HasVisibleChildren {
		t.Fatal("parent row did not expose its section-local children")
	}
	if rows[1].Breadcrumb != "" || rows[2].Breadcrumb != "" {
		t.Fatalf("nested child breadcrumbs = %q/%q, want none", rows[1].Breadcrumb, rows[2].Breadcrumb)
	}
}

func TestSectionRowForestDoesNotLinkDuplicateTaskIDsAcrossProjects(t *testing.T) {
	alphaParent := primaryRowsTask("shared-parent", models.StateBacklog)
	alphaChild := primaryRowsTask("alpha-child", models.StateBacklog)
	alphaChild.ParentID = &alphaParent.ID
	alpha := primaryRowsProject("alpha", alphaParent, alphaChild)
	betaParent := primaryRowsTask("shared-parent", models.StateBacklog)
	beta := primaryRowsProject("beta", betaParent)
	outliner, _ := primaryRowsOutliner(alpha, beta)

	forest := outliner.buildSectionRowForestWith([]OutlinerRow{
		{Task: alphaChild, ProjectID: alpha.ID, Breadcrumb: taskAncestorBreadcrumb(alphaChild)},
		{Task: betaParent, ProjectID: beta.ID},
	}, sectionForestOptions{rankRootsBySubtree: true})
	var rows []OutlinerRow
	outliner.flattenSectionRowForest(forest, &rows)

	if len(rows) != 2 || rows[0].HasVisibleChildren || rows[1].HasVisibleChildren {
		t.Fatalf("cross-project duplicate parent IDs formed an edge: %+v", rows)
	}
	if rows[0].Breadcrumb != alphaParent.Title+" ›" {
		t.Fatalf("promoted Alpha child breadcrumb = %q, want its actual parent", rows[0].Breadcrumb)
	}
}
