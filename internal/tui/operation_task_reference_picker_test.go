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

// pickerCheckedOrder mirrors what the picker actually hands the handler:
// selectedValue() sorts the checked payloads, so the order a user ticked rows in
// is never what taskReferencePatch walks. Tests build their checked sets through
// here so a diff assertion can never accidentally depend on check order.
func pickerCheckedOrder(locators ...string) []string {
	checked := append([]string(nil), locators...)
	sort.Strings(checked)
	return checked
}

// TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt is the assembly
// contract. The stored half leads, in stored order, whatever its targets have
// since become; the addition half follows and is exactly the population an
// addition is allowed to name.
func TestReferencePickerListsTheStoredHalfFirstAndNeverFiltersIt(t *testing.T) {
	subject, projects := referenceSuiteWorkspace(t)
	source := operationTaskReferenceListSource(subject, projects)

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
	source := operationTaskReferenceListSource(subject, projects)

	preselected := storedTaskReferenceIDs(subject)
	want := referenceIDsFor(
		"remote:target",
		"home:closed",
		"home:ghost",
		"gone:gone-1",
		"attic:relic",
	)
	if !slices.Equal(preselected, want) {
		t.Fatalf("preselection = %v, want every stored reference in stored order", preselected)
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
	source := operationTaskReferenceListSource(subject, projects)

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
	source := operationTaskReferenceListSource(subject, projects)

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
// commit can take. Add order follows the checked payloads, which selectedValue
// has already sorted; Remove order is stored order, because the removal half is
// read off the task rather than off the list.
func TestReferencePickerDiffsTheCheckedSetAgainstWhatIsStored(t *testing.T) {
	everyStored := []string{"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic"}

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
			wantRemove: []string{"home:closed", "gone:gone-1", "attic:relic"},
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
			wantRemove: everyStored,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			subject, _ := referenceSuiteWorkspace(t)
			patch := taskReferencePatch(subject, pickerCheckedOrder(testCase.checked...))
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
			checked := pickerCheckedOrder(
				"remote:target", "home:closed", "home:ghost", "gone:gone-1", "attic:relic",
			)
			patch := taskReferencePatch(subject, checked)
			if !patch.IsZero() {
				t.Fatalf("re-checking %s produced %+v, want no intent at all", locator, patch)
			}
		})
	}

	// A duplicate payload collapses rather than adding twice, which is the same
	// rule the parser applies to two aliases naming one task.
	patch := taskReferencePatch(subject, []string{"home:sibling", "home:sibling"})
	if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, []string{"home:sibling"}) {
		t.Errorf("add = %v, want one entry per target", got)
	}
}

// TestReferencePickerRefusesOffATaskRow keeps `~` inside the task grammar. A
// project header names no task, and a visual selection carries N tasks with N
// different stored sets that one checked list could not diff against, so both
// have to refuse rather than fall back to whatever the cursor happens to be on.
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

	t.Run("a visual selection refuses instead of falling back to the cursor", func(t *testing.T) {
		model, subject, home, _ := referencePickerFixture(t)
		model.crossOutliner.ReplaceSelection([]TaskIdentity{taskIdentityFor(subject, home)})

		ctx := model.operationCommandContext()
		if !ctx.Selection.Visual {
			t.Fatal("the fixture did not enter a visual selection")
		}
		if descriptorFor(t, model).Applicable(ctx) {
			t.Error("~ stayed applicable under a visual selection")
		}

		_, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: '~', Text: "~"})
		if cmd != nil {
			model.handleOperationOverlayMessage(cmd())
		}
		if model.operationValuePicker != nil {
			t.Fatal("~ opened a picker under a visual selection")
		}
	})
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
	cmd := model.setTaskReferencesCmd(taskIdentityFor(subject, home), []string{stored.Reference().Locator()})
	if cmd == nil {
		t.Fatal("an unchanged reference set produced no command at all")
	}
	if msg, ok := cmd().(InfoStatusMsg); !ok {
		t.Fatalf("an unchanged reference set = %T, want a status notice and no edit", msg)
	}
}
