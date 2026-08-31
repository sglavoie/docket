package tui

import (
	"slices"
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// referencePickerFixture is one task that already names a target in another
// project, with a second open task beside that target so the picker has both
// halves to offer: a stored row and an addition candidate.
func referencePickerFixture(t *testing.T) (*AppModel, *models.Task, *models.Project, *models.Project) {
	t.Helper()
	model := newUITestApp()
	target := &models.Task{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateBacklog}
	other := &models.Task{ID: "other", ShortID: 2, Title: "Write the guide", WorkflowState: models.StateBacklog}
	remote := &models.Project{ID: "remote", Name: "Remote", Slug: "remote", Tasks: []*models.Task{target, other}}
	remote.BuildTaskTree()

	subject := &models.Task{
		ID:            "subject",
		ShortID:       1,
		Title:         "Cut the release",
		WorkflowState: models.StateBacklog,
		Dependencies:  []models.TaskReference{{ProjectID: remote.ID, TaskID: target.ID}},
	}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Tasks: []*models.Task{subject}}
	home.BuildTaskTree()

	setTestAppProjects(model, home, remote)
	model.crossOutliner.loadedProjects = []*models.Project{home, remote}
	model.crossOutliner.rows = []outlinerRow{{task: subject, ProjectID: home.ID}}
	return model, subject, home, remote
}

// referenceSuiteWorkspace is the exhaustive population the assembly and diff
// cases read. Every shape a stored reference can wear is present at once —
// open, terminal, missing from a loaded project, pointing into a project this
// view never loaded, and pointing into an archived one — because the picker's
// first half is defined by what it refuses to filter, and a fixture holding one
// shape at a time could not show that.
//
// The returned population is what a caller passes as `projects`; `gone` is
// deliberately absent from it, which is what makes the fourth stored reference
// unreadable rather than merely closed.
func referenceSuiteWorkspace(t *testing.T) (subject *models.Task, projects []*models.Project) {
	t.Helper()
	sibling := &models.Task{ID: "sibling", ShortID: 2, Title: "Agree the schema", WorkflowState: models.StateBacklog}
	closed := &models.Task{ID: "closed", ShortID: 3, Title: "Closed groundwork", WorkflowState: models.StateDone}
	shelved := &models.Task{ID: "shelved", ShortID: 4, Title: "Shelved spike", WorkflowState: models.StateCanceled}
	subject = &models.Task{
		ID: "subject", ShortID: 1, Title: "Cut the release", WorkflowState: models.StateDoing,
		Dependencies: []models.TaskReference{
			{ProjectID: "remote", TaskID: "target"},
			{ProjectID: "home", TaskID: "closed"},
			{ProjectID: "home", TaskID: "ghost"},
			{ProjectID: "gone", TaskID: "gone-1"},
			{ProjectID: "attic", TaskID: "relic"},
		},
	}
	home := &models.Project{
		ID: "home", Name: "Home", Slug: "home",
		Tasks: []*models.Task{subject, sibling, closed, shelved},
	}
	remote := &models.Project{
		ID: "remote", Name: "Remote", Slug: "remote",
		Tasks: []*models.Task{
			{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateWaiting},
			{ID: "other", ShortID: 2, Title: "Write the guide", WorkflowState: models.StateBacklog},
		},
	}
	attic := &models.Project{
		ID: "attic", Name: "Attic", Slug: "attic", Archived: true,
		Tasks: []*models.Task{
			{ID: "relic", ShortID: 1, Title: "Retire the shim", WorkflowState: models.StateBacklog},
			{ID: "spare", ShortID: 2, Title: "Sweep the attic", WorkflowState: models.StateBacklog},
		},
	}
	for _, project := range []*models.Project{home, remote, attic} {
		project.BuildTaskTree()
	}
	return subject, []*models.Project{home, remote, attic}
}

func referenceRowIDs(source ListSource) []ListItemID {
	ids := make([]ListItemID, 0, len(source.Items))
	for _, item := range source.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func referenceRow(t *testing.T, source ListSource, locator string) ListItem {
	t.Helper()
	want := ListItemID("reference:" + locator)
	for _, item := range source.Items {
		if item.ID == want {
			return item
		}
	}
	t.Fatalf("the picker has no row for %s; rows = %v", locator, referenceRowIDs(source))
	return ListItem{}
}

func referenceIDsFor(locators ...string) []ListItemID {
	ids := make([]ListItemID, 0, len(locators))
	for _, locator := range locators {
		ids = append(ids, ListItemID("reference:"+locator))
	}
	return ids
}

// referenceMembershipAfter is the membership a picker holds once the user has
// left exactly `checked` ticked: every row the tasks opened with, moved to All
// where it was checked and None where it was not.
func referenceMembershipAfter(
	initial map[ListItemID]operationValueMembership, checked []string,
) map[ListItemID]operationValueMembership {
	current := make(map[ListItemID]operationValueMembership, len(initial)+len(checked))
	for id := range initial {
		current[id] = operationValueNone
	}
	for _, locator := range checked {
		current[ListItemID("reference:"+locator)] = operationValueAll
	}
	return current
}

// referencePickerCommit replays the whole commit path in one call: the picker
// emits the difference between the membership it preselected from `opened` and
// its final checked set, and the command merges that difference into whatever
// `current` stores at execute time. Passing the same task twice is the ordinary
// case, where nothing moved underneath the overlay; passing two different tasks
// is the concurrent one.
//
// The checked payloads are sorted first because selectedValue() sorts them, so
// no assertion here can accidentally depend on the order rows were ticked in.
func referencePickerCommit(opened, current *models.Task, checked ...string) app.DependencyPatch {
	sorted := append([]string(nil), checked...)
	sort.Strings(sorted)
	initial := taskReferenceMembership([]*models.Task{opened})
	return taskReferencePatch(current, taskReferenceSelection(
		initial, referenceMembershipAfter(initial, sorted),
	))
}

// TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt is the assembly
// contract. The stored half leads, in stored order, whatever its targets have
// since become; the addition half follows and is exactly the population an
// addition is allowed to name.
func TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	stored := referenceIDsFor(
		"remote:target",
		"home:closed",
		"home:ghost",
		"gone:gone-1",
		"attic:relic",
	)
	ids := referenceRowIDs(source)
	if len(ids) < len(stored) || !slices.Equal(ids[:len(stored)], stored) {
		t.Fatalf("the stored half is not the leading rows in stored order; rows = %v", ids)
	}

	// The addition half: every open task in every loaded non-archived project,
	// minus the source and minus the stored half. The terminal `home:shelved` and
	// the archived project's unreferenced `attic:spare` are the two exclusions
	// that would otherwise be invisible, and `home:subject` proves the source
	// cannot offer itself.
	additions := ids[len(stored):]
	if want := referenceIDsFor("home:sibling", "remote:other"); !slices.Equal(additions, want) {
		t.Errorf("addition half = %v, want %v", additions, want)
	}
}

// TestReferencePickerKeepsUnreachableStoredTargetsEnabled is the reason the
// stored half is never filtered: a reference whose target closed, vanished, or
// left the loaded population is precisely the one a user is most likely to be
// dropping, so it has to be listed, preselected, and enabled. Building the real
// picker model is the assertion that carries the most weight — SetSelectedIDs
// refuses a disabled row, so a preselection that survives construction is proof
// no stored row was quietly disabled.
func TestReferencePickerKeepsUnreachableStoredTargetsEnabled(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	want := referenceIDsFor(
		"remote:target",
		"home:closed",
		"home:ghost",
		"gone:gone-1",
		"attic:relic",
	)
	membership := taskReferenceMembership([]*models.Task{subject})
	preselected := make([]ListItemID, 0, len(membership))
	for _, id := range want {
		if membership[id] != operationValueAll {
			t.Fatalf("stored reference %s is not preselected; membership = %v", id, membership)
		}
		preselected = append(preselected, id)
	}
	if len(membership) != len(want) {
		t.Fatalf("preselection = %v, want every stored reference and nothing else", membership)
	}
	for _, id := range preselected {
		index := slices.IndexFunc(source.Items, func(item ListItem) bool { return item.ID == id })
		if index < 0 {
			t.Fatalf("preselected row %s is not in the list source", id)
		}
		if !source.Items[index].Enabled() {
			t.Errorf("stored row %s is disabled: %q", id, source.Items[index].DisabledReason)
		}
	}

	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{},
		ValueProviderTaskReference,
		"Set Dependencies",
		"",
		source,
		true,
		preselected,
	)
	if err != nil {
		t.Fatalf("the stored set could not be preselected: %v", err)
	}
	if selected := picker.surface.SelectedIDs(); !slices.Equal(selected, want) {
		t.Errorf("checked rows = %v, want the whole stored set", selected)
	}
}

// TestReferencePickerUnavailableRowKeepsItsLocator holds the one row that can
// say nothing about its target to saying exactly that, in the word Peek and the
// detail pane already use, over the durable locator storage holds.
func TestReferencePickerUnavailableRowKeepsItsLocator(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	// `home:ghost` is a loaded project with no such task and `gone:gone-1` is a
	// project this view never loaded. The picker cannot tell them apart and must
	// not pretend to: both keep their locator and say only `unavailable`.
	for _, locator := range []string{"home:ghost", "gone:gone-1"} {
		item := referenceRow(t, source, locator)
		if item.PrimaryText != locator {
			t.Errorf("%s label = %q, want the raw project-id:task-id", locator, item.PrimaryText)
		}
		if item.SecondaryText != dependencyDetailUnavailable {
			t.Errorf("%s status = %q, want %q", locator, item.SecondaryText, dependencyDetailUnavailable)
		}
		if item.Data != locator {
			t.Errorf("%s payload = %v, want the raw locator", locator, item.Data)
		}
		if item.CategoryID != "" {
			t.Errorf("%s was grouped under %q; an unreadable row names no project", locator, item.CategoryID)
		}
		if !item.Enabled() {
			t.Errorf("%s is disabled; an unreadable reference is still the user's to drop", locator)
		}
	}
}

// TestReferencePickerQualifiesCrossProjectRows checks the two things that keep a
// filtered list honest across a project boundary: the row is grouped under its
// project, and its search text is the one `~` autocomplete scores, so a query
// that finds a target when typed finds the same target when picked.
func TestReferencePickerQualifiesCrossProjectRows(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource([]*models.Task{subject}, projects)

	home, remote, attic := projects[0], projects[1], projects[2]
	for _, testCase := range []struct {
		locator   string
		project   *models.Project
		taskID    string
		category  ListCategoryID
		secondary string
	}{
		{locator: "remote:target", project: remote, taskID: "target", category: "remote", secondary: "⏳ #1"},
		{locator: "home:closed", project: home, taskID: "closed", category: "home", secondary: "✓ #3"},
		{locator: "attic:relic", project: attic, taskID: "relic", category: "attic", secondary: "○ #1"},
		{locator: "home:sibling", project: home, taskID: "sibling", category: "home", secondary: "○ #2"},
	} {
		item := referenceRow(t, source, testCase.locator)
		task := testCase.project.GetTaskByID(testCase.taskID)
		if item.PrimaryText != task.Title {
			t.Errorf("%s label = %q, want %q", testCase.locator, item.PrimaryText, task.Title)
		}
		if item.CategoryID != testCase.category {
			t.Errorf("%s category = %q, want %q", testCase.locator, item.CategoryID, testCase.category)
		}
		candidate := dependencyCandidate{project: testCase.project, task: task, locator: app.TaskLocator{
			ProjectID: testCase.project.ID,
			TaskID:    testCase.taskID,
		}}
		if want := []string{dependencyCandidateSearchText(candidate)}; !slices.Equal(item.Aliases, want) {
			t.Errorf("%s aliases = %v, want the ~ completion's search text %v", testCase.locator, item.Aliases, want)
		}
	}

	// Categories are emitted once per project that actually owns a row, in
	// population order, and carry the project's display name — which is what lets
	// the label itself stay unqualified.
	labels := make(map[ListCategoryID]string, len(source.Categories))
	order := make([]ListCategoryID, 0, len(source.Categories))
	for _, category := range source.Categories {
		if _, seen := labels[category.ID]; seen {
			t.Fatalf("category %q is emitted twice", category.ID)
		}
		labels[category.ID] = category.Label
		order = append(order, category.ID)
	}
	if want := []ListCategoryID{"home", "remote", "attic"}; !slices.Equal(order, want) {
		t.Errorf("categories = %v, want one per row-owning project in population order %v", order, want)
	}
	for id, want := range map[ListCategoryID]string{"home": "Home", "remote": "Remote", "attic": "Attic"} {
		if labels[id] != want {
			t.Errorf("category %q label = %q, want %q", id, labels[id], want)
		}
	}
}

// TestReferencePickerDiffsTheCheckedSetAgainstWhatIsStored walks every shape the
// commit can take. Both halves are sorted, because both are read off a
// membership map rather than off an ordered list, and a patch is a set on either
// side.
func TestReferencePickerDiffsTheCheckedSetAgainstWhatIsStored(t *testing.T) {
	everyStored := []string{"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic"}
	sortedStored := []string{"attic:relic", "gone:gone-1", "home:closed", "home:ghost", "remote:target"}

	for _, testCase := range []struct {
		name       string
		checked    []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			name:    "unchanged",
			checked: everyStored,
		},
		{
			name:    "add only",
			checked: append(append([]string(nil), everyStored...), "home:sibling", "remote:other"),
			wantAdd: []string{"home:sibling", "remote:other"},
		},
		{
			name:       "remove only",
			checked:    []string{"remote:target", "home:ghost"},
			wantRemove: []string{"attic:relic", "gone:gone-1", "home:closed"},
		},
		{
			name:       "add and remove in one commit",
			checked:    []string{"remote:target", "home:ghost", "gone:gone-1", "attic:relic", "home:sibling"},
			wantAdd:    []string{"home:sibling"},
			wantRemove: []string{"home:closed"},
		},
		{
			// Nothing checked is a legal full removal, not a refused empty value, and
			// it is spelled as Remove rather than Clear: the application layer refuses
			// Clear beside Add or Remove, and removing every stored reference already
			// says the same thing.
			name:       "everything unchecked",
			checked:    nil,
			wantRemove: sortedStored,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			subject, _ := referenceSuiteWorkspace(t)
			patch := referencePickerCommit(subject, subject, testCase.checked...)
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, testCase.wantAdd) {
				t.Errorf("add = %v, want %v", got, testCase.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, testCase.wantRemove) {
				t.Errorf("remove = %v, want %v", got, testCase.wantRemove)
			}
			if patch.Clear {
				t.Error("the picker set Clear; it only ever spells a removal")
			}
			if wantZero := testCase.wantAdd == nil && testCase.wantRemove == nil; patch.IsZero() != wantZero {
				t.Errorf("patch.IsZero() = %v, want %v: %+v", patch.IsZero(), wantZero, patch)
			}
		})
	}
}

// TestReferencePickerRecheckingAStoredTargetIsANoOp is the picker's half of the
// application layer's known-addition rule. Leaving a terminal or unreadable
// stored row checked is the ordinary case — it is how a user removes something
// else without touching it — so it has to fall out as no intent at all, never as
// an addition the resolver would then have to judge against the addition rules.
func TestReferencePickerRecheckingAStoredTargetIsANoOp(t *testing.T) {
	subject, _ := referenceSuiteWorkspace(t)
	for _, locator := range []string{"home:closed", "gone:gone-1", "attic:relic"} {
		t.Run(locator, func(t *testing.T) {
			patch := referencePickerCommit(
				subject, subject,
				"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic",
			)
			if !patch.IsZero() {
				t.Fatalf("re-checking %s produced %+v, want no intent at all", locator, patch)
			}
		})
	}

	// A duplicate payload collapses rather than adding twice, which is the same
	// rule the parser applies to two aliases naming one task.
	patch := referencePickerCommit(
		subject, subject,
		"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic",
		"home:sibling", "home:sibling",
	)
	if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, []string{"home:sibling"}) {
		t.Errorf("add = %v, want one entry per target", got)
	}
}

// concurrentReferencePickerFixture is one task storing two references, with a
// third target beside them to check and a same-project task the picker never
// preselects. Two stored rows is the minimum that can show a concurrent change
// landing on one of them while the user edits the other.
func concurrentReferencePickerFixture(t *testing.T) (*AppModel, *models.Task, *models.Project) {
	t.Helper()
	model := newUITestApp()
	remote := &models.Project{ID: "remote", Name: "Remote", Slug: "remote", Tasks: []*models.Task{
		{ID: "target", ShortID: 1, Title: "Ship the API", WorkflowState: models.StateBacklog},
		{ID: "legacy", ShortID: 2, Title: "Retire the shim", WorkflowState: models.StateBacklog},
		{ID: "other", ShortID: 3, Title: "Write the guide", WorkflowState: models.StateBacklog},
	}}
	home := republishedReferenceHome(t, "remote:target", "remote:legacy")
	for _, project := range []*models.Project{home, remote} {
		project.BuildTaskTree()
	}
	setTestAppProjects(model, home, remote)
	model.crossOutliner.loadedProjects = []*models.Project{home, remote}
	subject := home.GetTaskByID("subject")
	model.crossOutliner.rows = []outlinerRow{{task: subject, ProjectID: home.ID}}
	return model, subject, home
}

// republishedReferenceHome builds the home project as a freshly loaded snapshot
// would carry it, with the subject's stored references set to exactly the given
// locators. Adopting one of these is what a cross-project load does to the shell
// cache, and it is deliberately a whole new task tree: adoptCrossProjectSnapshot
// overwrites the cached project in place, so nothing the picker captured earlier
// is updated by it.
func republishedReferenceHome(t *testing.T, locators ...string) *models.Project {
	t.Helper()
	references := make([]models.TaskReference, 0, len(locators))
	for _, raw := range locators {
		locator, ok := taskReferenceLocator(raw)
		if !ok {
			t.Fatalf("%q is not a locator the picker could ever carry", raw)
		}
		references = append(references, locator.Reference())
	}
	home := &models.Project{ID: "home", Name: "Home", Slug: "home", Tasks: []*models.Task{
		{
			ID: "subject", ShortID: 1, Title: "Cut the release",
			WorkflowState: models.StateBacklog, Dependencies: references,
		},
		{ID: "chore", ShortID: 2, Title: "Update the changelog", WorkflowState: models.StateBacklog},
	}}
	home.BuildTaskTree()
	return home
}

// TestReferencePickerPreservesConcurrentReferenceChanges is the merge contract.
// The picker overlay is not modal against the workspace: an accepted
// cross-project load republishes task trees through adoptCrossProjectSnapshot
// while `~` is still open, and setTaskReferencesCmd then resolves the task again
// against that newer state. A commit that spelled the whole checked set would
// read every reference the user never touched as an assertion — dropping one the
// load added and resurrecting one it removed — so the picker emits only what the
// user changed and the command merges that into the fresh state.
func TestReferencePickerPreservesConcurrentReferenceChanges(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		republish  []string
		wantAdd    []string
		wantRemove []string
	}{
		{
			// `home:chore` arrives while the overlay is up. It was never on a row the
			// user could uncheck, so the commit must not spell it as a removal.
			name:       "a reference added under the picker survives the commit",
			republish:  []string{"remote:target", "remote:legacy", "home:chore"},
			wantAdd:    []string{"remote:other"},
			wantRemove: []string{"remote:legacy"},
		},
		{
			// `remote:target` leaves while the overlay is up. It is still checked
			// because the user never touched it, so the commit must not read that
			// stale check as an addition and put it back.
			name:       "a reference removed under the picker stays removed",
			republish:  []string{"remote:legacy"},
			wantAdd:    []string{"remote:other"},
			wantRemove: []string{"remote:legacy"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			model, subject, home := concurrentReferencePickerFixture(t)
			identity := taskIdentityFor(subject, home)

			_, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
			if cmd != nil {
				model.handleOperationOverlayMessage(cmd())
			}
			picker := model.operationValuePicker
			if picker == nil || picker.provider != ValueProviderTaskReference {
				t.Fatalf("~ did not open the reference picker: %#v", picker)
			}

			// The user's whole edit: check `remote:other`, uncheck `remote:legacy`,
			// leave `remote:target` exactly as the picker preselected it. Both moves
			// go through the toggle the keyboard uses, because membership rather than
			// the checked set is what the commit is a difference against.
			for _, id := range referenceIDsFor("remote:other", "remote:legacy") {
				if !picker.surface.SetCursorToID(id) {
					t.Fatalf("the picker has no row for %s", id)
				}
				if err := picker.toggleCurrentMultiValue(); err != nil {
					t.Fatalf("toggling %s: %v", id, err)
				}
			}

			// Only now does the load land, after the picker was built and edited and
			// before Enter — the window the whole contract is about.
			model.adoptCrossProjectSnapshot([]*models.Project{republishedReferenceHome(t, testCase.republish...)})
			fresh, project := model.taskAndProjectForIdentity(identity)
			if fresh == nil || project == nil {
				t.Fatal("the republished snapshot left the task unresolvable")
			}
			gotStored := make([]string, 0, len(fresh.Dependencies))
			for _, reference := range fresh.Dependencies {
				gotStored = append(gotStored, reference.Locator())
			}
			if !slices.Equal(gotStored, testCase.republish) {
				t.Fatalf("the fresh task stores %v, want the republished %v", gotStored, testCase.republish)
			}

			_, done, msg := picker.handleKey(syntheticSpecialKey(tea.KeyEnter))
			if !done {
				t.Fatal("Enter did not close the picker")
			}
			selected, ok := msg.(OperationValueSelectedMsg)
			if !ok {
				t.Fatalf("Enter emitted %T, want a value selection", msg)
			}
			value, ok := selected.Value.(OperationTaskReferenceValue)
			if !ok {
				t.Fatalf("the picker emitted %T, want the reference difference", selected.Value)
			}
			if !slices.Equal(value.Add, []string{"remote:other"}) {
				t.Errorf("emitted add = %v, want the one row the user checked", value.Add)
			}
			if !slices.Equal(value.Remove, []string{"remote:legacy"}) {
				t.Errorf("emitted remove = %v, want the one row the user unchecked", value.Remove)
			}

			patch := taskReferencePatch(fresh, value)
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, testCase.wantAdd) {
				t.Errorf("add = %v, want %v", got, testCase.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, testCase.wantRemove) {
				t.Errorf("remove = %v, want %v", got, testCase.wantRemove)
			}
			if patch.Clear {
				t.Error("the picker set Clear; it only ever spells a removal")
			}
		})
	}
}

// TestReferencePickerRefusesOffATaskRow keeps `~` inside the task grammar: a
// project header names no task, so it refuses rather than falling back to
// whatever the cursor happens to be on. A visual selection is the other case and
// no longer a refusal — it edits every selected task, which
// TestReferencePickerEditsAWholeSelectionTriState holds.
func TestReferencePickerRefusesOffATaskRow(t *testing.T) {
	descriptorFor := func(t *testing.T, model *AppModel) OperationDescriptor {
		t.Helper()
		descriptor, ok := model.operationRegistry.Descriptor(operationTaskReferences)
		if !ok {
			t.Fatal("tui.task.references is not registered")
		}
		return descriptor
	}

	t.Run("a project header row resolves no target", func(t *testing.T) {
		model, _, home, _ := referencePickerFixture(t)
		model.crossOutliner.rows = append(
			[]outlinerRow{{IsProjectHeader: true, ProjectID: home.ID, SectionName: home.Name}},
			model.crossOutliner.rows...,
		)
		model.crossOutliner.cursor = 0

		ctx := model.operationCommandContext()
		if !ctx.Entity.HasProjectHeaderCursor || ctx.Entity.TaskID != "" {
			t.Fatalf("the fixture cursor is not on a project header row: %+v", ctx.Entity)
		}
		descriptor := descriptorFor(t, model)
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, ctx); err == nil {
			t.Fatal("~ resolved a target from a project header cursor")
		} else if err.Error() != operationTargetRejectionMessage(OperationCategoryTask) {
			t.Errorf("rejection = %q, want the shared sentence", err)
		}

		handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
		if cmd != nil {
			model.handleOperationOverlayMessage(cmd())
		}
		if model.operationValuePicker != nil {
			t.Fatalf("~ opened a picker from a project header row (handled = %v)", handled)
		}
	})

	t.Run("a visual selection is the selection, never the cursor", func(t *testing.T) {
		model, subject, home, _ := referencePickerFixture(t)
		model.crossOutliner.ReplaceSelection([]TaskIdentity{taskIdentityFor(subject, home)})

		ctx := model.operationCommandContext()
		if !ctx.Selection.Visual {
			t.Fatal("the fixture did not enter a visual selection")
		}
		if !descriptorFor(t, model).Applicable(ctx) {
			t.Fatal("~ stopped applying under a visual selection")
		}
		targets, err := (OperationTargetResolver{}).Resolve(descriptorFor(t, model), ctx)
		if err != nil {
			t.Fatalf("~ resolved no target under a visual selection: %v", err)
		}
		if len(targets.Targets) != 1 || targets.Targets[0].Task.TaskID != subject.ID {
			t.Fatalf("targets = %+v, want the selected task alone", targets.Targets)
		}
	})
}

// TestReferencePickerEditsAWholeSelectionTriState is the bulk contract. The
// documented objection to bulk here — N tasks carrying N different stored sets —
// is an objection to a replacement picker, so the answer is the tri-state one
// tags already use: a reference everybody stores opens checked, one only part of
// the selection stores opens indeterminate and belongs to neither half of the
// emitted patch, and a selected task is never offered as a target of another.
func TestReferencePickerEditsAWholeSelectionTriState(t *testing.T) {
	model, subject, home, remote := referencePickerFixture(t)
	second := &models.Task{
		ID: "second", ShortID: 2, Title: "Draft the notes", WorkflowState: models.StateBacklog,
		Dependencies: []models.TaskReference{
			{ProjectID: remote.ID, TaskID: "target"},
			{ProjectID: remote.ID, TaskID: "other"},
		},
	}
	home.Tasks = append(home.Tasks, second)
	home.BuildTaskTree()
	setTestAppProjects(model, home, remote)
	model.crossOutliner.loadedProjects = []*models.Project{home, remote}
	model.crossOutliner.rows = []outlinerRow{
		{task: subject, ProjectID: home.ID},
		{task: second, ProjectID: home.ID},
	}
	model.crossOutliner.ReplaceSelection([]TaskIdentity{
		taskIdentityFor(subject, home), taskIdentityFor(second, home),
	})

	_, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
	if cmd != nil {
		model.handleOperationOverlayMessage(cmd())
	}
	picker := model.operationValuePicker
	if picker == nil || picker.provider != ValueProviderTaskReference {
		t.Fatalf("~ did not open the reference picker over a selection: %#v", picker)
	}

	shared := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "target"})
	partial := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "other"})
	if got := picker.membershipInitial[shared]; got != operationValueAll {
		t.Errorf("the reference both tasks store opened at %v, want checked", got)
	}
	if got := picker.membershipInitial[partial]; got != operationValueSome {
		t.Errorf("the reference one task stores opened at %v, want indeterminate", got)
	}
	for _, item := range picker.surface.Items() {
		if item.ID == partial && !item.Indeterminate {
			t.Error("the row only part of the selection stores is not drawn indeterminate")
		}
		if locator, ok := taskReferenceRowLocator(item.ID); ok && locator.TaskID == second.ID {
			t.Errorf("the picker offered %s, a task inside the selection", item.ID)
		}
	}

	// Touching nothing but the shared row leaves the row the tasks disagree about
	// out of both halves, which is what keeps them disagreeing.
	if !picker.surface.SetCursorToID(shared) {
		t.Fatalf("the picker has no row for %s", shared)
	}
	if err := picker.toggleCurrentMultiValue(); err != nil {
		t.Fatalf("toggling the shared row: %v", err)
	}
	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	selection, ok := value.(OperationTaskReferenceValue)
	if !ok {
		t.Fatalf("the picker emitted %T, want a reference difference", value)
	}
	if len(selection.Add) != 0 {
		t.Errorf("Add = %v, want nothing added", selection.Add)
	}
	if !slices.Equal(selection.Remove, []string{"remote:target"}) {
		t.Errorf("Remove = %v, want the unchecked shared reference alone", selection.Remove)
	}
	patch := taskReferenceBulkPatch(selection)
	if len(patch.Add) != 0 || len(patch.Remove) != 1 || patch.Clear {
		t.Fatalf("bulk patch = %+v, want one removal and nothing else", patch)
	}
}

// TestReferencePickerOpensOnTildeWithTheStoredSetChecked is the session's
// behavioral assertion: `~` on a task row reaches the picker through the real
// key path, and what the task already stores arrives checked, so unchecking a
// row is the removal the sigil's `-~task` form spells.
func TestReferencePickerOpensOnTildeWithTheStoredSetChecked(t *testing.T) {
	model, _, _, remote := referencePickerFixture(t)

	handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
	if !handled {
		t.Fatal("~ was not handled on a task row")
	}
	if cmd != nil {
		model.handleOperationOverlayMessage(cmd())
	}
	picker := model.operationValuePicker
	if picker == nil || picker.provider != ValueProviderTaskReference {
		t.Fatalf("~ did not open the reference picker: %#v", picker)
	}

	storedRow := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "target"})
	candidateRow := taskReferenceRowID(app.TaskLocator{ProjectID: remote.ID, TaskID: "other"})
	ids := make([]ListItemID, 0, len(picker.surface.Items()))
	for _, item := range picker.surface.Items() {
		ids = append(ids, item.ID)
	}
	if !slices.Contains(ids, storedRow) {
		t.Fatalf("the picker has no row for the stored reference; rows = %v", ids)
	}
	if !slices.Contains(ids, candidateRow) {
		t.Fatalf("the picker offers no addition candidate; rows = %v", ids)
	}
	if selected := picker.surface.SelectedIDs(); !slices.Equal(selected, []ListItemID{storedRow}) {
		t.Errorf("checked rows = %v, want the stored reference alone", selected)
	}
}

// TestReferencePickerCommitsNothingWhenTheCheckedSetIsUnchanged is the other
// half of the wiring: leaving the preselection alone must not open an empty
// transaction, because that would still write a ledger entry `u` could take
// back for an edit that changed nothing.
func TestReferencePickerCommitsNothingWhenTheCheckedSetIsUnchanged(t *testing.T) {
	model, subject, home, remote := referencePickerFixture(t)

	stored := app.TaskLocator{ProjectID: remote.ID, TaskID: "target"}
	initial := taskReferenceMembership([]*models.Task{subject})
	value := taskReferenceSelection(
		initial, referenceMembershipAfter(initial, []string{stored.Reference().Locator()}),
	)
	if value.Add != nil || value.Remove != nil {
		t.Fatalf("leaving the preselection alone emitted %+v, want no intent at all", value)
	}
	cmd := model.setTaskReferencesCmd(taskIdentityFor(subject, home), value)
	if cmd == nil {
		t.Fatal("an unchanged reference set produced no command at all")
	}
	if msg, ok := cmd().(InfoStatusMsg); !ok {
		t.Fatalf("an unchanged reference set = %T, want a status notice and no edit", msg)
	}
}
