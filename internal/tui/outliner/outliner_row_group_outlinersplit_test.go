package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The contract these tests hold is that a grouped projection names its groups
// structurally: every row carries the section and project-group identity it
// belongs to, that identity is spelled from the collapse keys the fold state
// already stores, and nothing about it is derived from where the row landed.
// Header rows are one row kind carrying it, not its source.

// rowGroupFixture seeds two projects whose rows land in two different All-mode
// state sections, so one project is repeated across sections and every section
// holds more than one project.
type rowGroupFixture struct {
	alpha        *models.Project
	beta         *models.Project
	alphaBacklog *models.Task
	alphaWaiting *models.Task
	betaBacklog  *models.Task
}

func newRowGroupFixture() rowGroupFixture {
	alphaBacklog := primaryRowsTask("alpha-backlog", models.StateBacklog)
	alphaWaiting := primaryRowsTask("alpha-waiting", models.StateWaiting)
	betaBacklog := primaryRowsTask("beta-backlog", models.StateBacklog)
	return rowGroupFixture{
		alpha:        primaryRowsProject("alpha", alphaBacklog, alphaWaiting),
		beta:         primaryRowsProject("beta", betaBacklog),
		alphaBacklog: alphaBacklog,
		alphaWaiting: alphaWaiting,
		betaBacklog:  betaBacklog,
	}
}

func rowGroupFor(t *testing.T, m *OutlinerModel, taskID string) OutlinerRowGroup {
	t.Helper()
	row, found := rowForTaskID(m, taskID)
	if !found {
		t.Fatalf("task %q has no row", taskID)
	}
	return row.Group
}

// TestRowGroupSpellsTheFoldsThatHideIt is the base claim: a task row's group is
// exactly the two collapse keys its projection emitted for it, in the namespaces
// those two axes already own.
func TestRowGroupSpellsTheFoldsThatHideIt(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	group := rowGroupFor(t, outliner, fixture.alphaBacklog.ID)
	wantSection := stateGroupCollapsePrefix + string(models.StateGroupBacklog)
	wantProject := projectSectionCollapseKey(fixture.alpha.ID, models.StateGroupBacklog)
	if group.SectionKey != wantSection || group.ProjectKey != wantProject {
		t.Fatalf("alpha backlog group = %+v, want section %q and project %q",
			group, wantSection, wantProject)
	}
	if group.ProjectID != fixture.alpha.ID {
		t.Fatalf("alpha backlog group project = %q, want %q", group.ProjectID, fixture.alpha.ID)
	}
	if !group.ProjectGrouped() {
		t.Fatalf("cross-project All reported group %+v as not project-grouped", group)
	}
	if got := group.CollapseKeys(); !slices.Equal(got, []string{wantProject, wantSection}) {
		t.Fatalf("collapse keys = %v, want the project fold before the section fold", got)
	}
}

// TestRowGroupCollapseKeysAreTheOnesThatHideTheRow pins the pair to behavior
// rather than to spelling: each key a group names, and only those, removes the
// row when written.
func TestRowGroupCollapseKeysAreTheOnesThatHideTheRow(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	keys := rowGroupFor(t, outliner, fixture.alphaBacklog.ID).CollapseKeys()
	if len(keys) != 2 {
		t.Fatalf("collapse keys = %v, want both a project and a section fold", keys)
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			outliner.SelectOutlinerMode(chrome.ModeAll)
			outliner.Folds.setSection(key, true)
			outliner.RebuildRows()
			if _, found := rowForTaskID(outliner, fixture.alphaBacklog.ID); found {
				t.Fatalf("fold %q left the row it names on screen", key)
			}
			// The other project in that section is hidden only by the section fold,
			// never by alpha's own.
			_, betaVisible := rowForTaskID(outliner, fixture.betaBacklog.ID)
			wantBeta := key != stateGroupCollapsePrefix+string(models.StateGroupBacklog)
			if betaVisible != wantBeta {
				t.Fatalf("fold %q left beta visible = %v, want %v", key, betaVisible, wantBeta)
			}
		})
	}
}

// TestRowGroupSeparatesOneProjectRepeatedAcrossSections is the repeated-project
// case: alpha owns a group in Backlog and another in Waiting, and the two are
// different identities sharing one project.
func TestRowGroupSeparatesOneProjectRepeatedAcrossSections(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	backlog := rowGroupFor(t, outliner, fixture.alphaBacklog.ID)
	waiting := rowGroupFor(t, outliner, fixture.alphaWaiting.ID)
	if backlog == waiting {
		t.Fatalf("alpha's Backlog and Waiting rows share one group %+v", backlog)
	}
	if backlog.ProjectID != waiting.ProjectID {
		t.Fatalf("the two groups name projects %q and %q, want the same owner",
			backlog.ProjectID, waiting.ProjectID)
	}
	if backlog.SectionKey == waiting.SectionKey || backlog.ProjectKey == waiting.ProjectKey {
		t.Fatalf("groups %+v and %+v share a fold key across a section boundary", backlog, waiting)
	}

	groups := outliner.VisibleProjectGroups()
	if len(groups) != 3 {
		t.Fatalf("visible project groups = %+v, want one per project per section", groups)
	}
	owners := make([]string, 0, len(groups))
	for _, group := range groups {
		owners = append(owners, group.ProjectID)
	}
	if !slices.Equal(owners, []string{fixture.alpha.ID, fixture.beta.ID, fixture.alpha.ID}) {
		t.Fatalf("group owners = %v, want alpha repeated across the two sections", owners)
	}
}

// TestRowGroupMatchesTheHeaderAbovePeriod holds the header to the group rather
// than the other way round: every project header carries the identity its
// members carry, and every section header carries the section half alone.
func TestRowGroupMatchesTheHeaderAbovePeriod(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	var section, project OutlinerRowGroup
	for _, row := range outliner.Rows {
		switch {
		case row.IsSectionHeader:
			section = row.Group
			project = OutlinerRowGroup{}
			if row.Group.ProjectGrouped() {
				t.Fatalf("section header %q carries a project half: %+v", row.SectionName, row.Group)
			}
			if row.Group.SectionKey != row.EffectiveCollapseKey() {
				t.Fatalf("section header %q group = %+v, want its own fold key %q",
					row.SectionName, row.Group, row.EffectiveCollapseKey())
			}
		case row.IsProjectHeader && row.IsCollapsibleHeader():
			project = row.Group
			if row.Group.SectionKey != section.SectionKey {
				t.Fatalf("project header %q group = %+v, want section %q",
					row.ProjectID, row.Group, section.SectionKey)
			}
			if row.Group.ProjectKey != row.EffectiveCollapseKey() {
				t.Fatalf("project header %q group = %+v, want its own fold key %q",
					row.ProjectID, row.Group, row.EffectiveCollapseKey())
			}
		case row.IsTaskRow():
			if row.Group != project {
				t.Fatalf("task %q group = %+v, want the header group %+v",
					row.Task.ID, row.Group, project)
			}
		}
	}
}

// TestRowGroupKeepsSubtreeAndChecklistRowsWithTheirOwner covers the two row
// kinds that descend from a task: a child row belongs to its ancestor's group,
// and a checklist item answers the same membership question its owner does.
func TestRowGroupKeepsSubtreeAndChecklistRowsWithTheirOwner(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Checklist = []models.ChecklistItem{{Text: "step one"}}
	child := primaryRowsTask("child", models.StateBacklog)
	parentID := parent.ID
	child.ParentID = &parentID
	alpha := primaryRowsProject("alpha", parent, child)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateBacklog))

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	outliner.Folds.SetChecklist(taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}, true)
	outliner.RebuildRows()

	owner := rowGroupFor(t, outliner, parent.ID)
	if !owner.ProjectGrouped() {
		t.Fatalf("parent group %+v is not project-grouped", owner)
	}
	if got := rowGroupFor(t, outliner, child.ID); got != owner {
		t.Fatalf("child group = %+v, want the ancestor group %+v", got, owner)
	}
	checklistRows := 0
	for _, row := range outliner.Rows {
		if !row.IsChecklistItem {
			continue
		}
		checklistRows++
		if row.Group != owner {
			t.Fatalf("checklist row group = %+v, want the owner group %+v", row.Group, owner)
		}
	}
	if checklistRows != 1 {
		t.Fatalf("checklist rows = %d, want the one expanded item", checklistRows)
	}
}

// TestRowGroupCarriesNoProjectHalfInSingleProjectForms locks the other side of
// the emitter's one predicate: a projection that draws no project headers is not
// project-grouped, so its rows name the section alone and p finds no group.
func TestRowGroupCarriesNoProjectHalfInSingleProjectForms(t *testing.T) {
	fixture := newRowGroupFixture()

	scoped, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	scoped.PushProjectScope(fixture.alpha.ID)
	scoped.SelectOutlinerMode(chrome.ModeAll)

	single := NewOutlinerModel(fixture.alpha, nil, nil)
	single.SelectOutlinerMode(chrome.ModeAll)

	for name, outliner := range map[string]*OutlinerModel{"scoped": scoped, "single": single} {
		t.Run(name, func(t *testing.T) {
			group := rowGroupFor(t, outliner, fixture.alphaBacklog.ID)
			if group.ProjectGrouped() || group.ProjectID != "" {
				t.Fatalf("%s group = %+v, want no project half", name, group)
			}
			wantSection := stateGroupCollapsePrefix + string(models.StateGroupBacklog)
			if group.SectionKey != wantSection {
				t.Fatalf("%s group section = %q, want %q", name, group.SectionKey, wantSection)
			}
			if got := group.CollapseKeys(); !slices.Equal(got, []string{wantSection}) {
				t.Fatalf("%s collapse keys = %v, want the section fold alone", name, got)
			}
			if groups := outliner.VisibleProjectGroups(); len(groups) != 0 {
				t.Fatalf("%s reported project groups %+v in a single-project form", name, groups)
			}
		})
	}
}

// TestRowGroupIsStableAcrossRebuildsAndFolds is the "never a row index" claim:
// the identity a row carries is unchanged when the rows above it move.
func TestRowGroupIsStableAcrossRebuildsAndFolds(t *testing.T) {
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	before, beforeIndex := rowGroupFor(t, outliner, fixture.alphaWaiting.ID), -1
	for index, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == fixture.alphaWaiting.ID {
			beforeIndex = index
		}
	}

	// Folding the Backlog section moves every Waiting row up without changing
	// which group any of them belongs to.
	outliner.Folds.setSection(stateGroupCollapsePrefix+string(models.StateGroupBacklog), true)
	outliner.RebuildRows()

	afterIndex := -1
	for index, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == fixture.alphaWaiting.ID {
			afterIndex = index
		}
	}
	if afterIndex == beforeIndex {
		t.Fatalf("the Waiting row never moved, so this proves nothing about index independence")
	}
	if after := rowGroupFor(t, outliner, fixture.alphaWaiting.ID); after != before {
		t.Fatalf("group after the reshape = %+v, want the unchanged %+v", after, before)
	}
}

// TestPlanningLaneAndPlanningModeShareOneProjectFold documents the overlap that
// keeps the mode-transition anchor group-free: All's Today lane and the Today
// mode name one project's group with the same fold key while carrying different
// section halves. A group is therefore an identity within one projection, never
// a handle to carry across two, which is why restoreModeCursor holds none.
func TestPlanningLaneAndPlanningModeShareOneProjectFold(t *testing.T) {
	alphaTask := primaryRowsTask("alpha-task", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaTask)
	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	beta := primaryRowsProject("beta", betaTask)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: alpha.ID, TaskID: alphaTask.ID}})

	outliner.SelectOutlinerMode(chrome.ModeAll)
	lane := rowGroupFor(t, outliner, alphaTask.ID)
	if lane.SectionKey != todaySectionCollapseKey {
		t.Fatalf("All's Today lane group = %+v, want section %q", lane, todaySectionCollapseKey)
	}

	outliner.SelectOutlinerMode(chrome.ModeToday)
	mode := rowGroupFor(t, outliner, alphaTask.ID)
	if mode.SectionKey != "" {
		t.Fatalf("Today mode group = %+v, want no section half in a mode drawing no section header", mode)
	}
	if mode.ProjectKey != lane.ProjectKey {
		t.Fatalf("project folds = %q and %q, want the one planning key both surfaces already share",
			mode.ProjectKey, lane.ProjectKey)
	}
	if mode == lane {
		t.Fatalf("the lane group and the mode group compare equal: %+v", lane)
	}
	if got := mode.CollapseKeys(); !slices.Equal(got, []string{mode.ProjectKey}) {
		t.Fatalf("Today mode collapse keys = %v, want the project fold alone", got)
	}
}
