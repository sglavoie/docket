package tui

// Shared fixtures for the shell's lens tests: the registered AppModel the lens
// operations run against, the machine-state lens seeds, and the picker and
// gallery drivers. They live here rather than beside any one test because a
// dozen shell test files read them, and a helper hidden in a file that also
// declares tests is orphaned the moment that file moves.

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/route"
)

// lensOperationApp wires the production lens registrations against the richer
// two-project scope the lifecycle tests use, so an applied lens narrows a real
// projection rather than an empty one.
func lensOperationApp(t *testing.T) *AppModel {
	t.Helper()
	model := typedTestApp(t)
	if err := model.registerLensOperations(); err != nil {
		t.Fatalf("register lens operations: %v", err)
	}
	model.crossOutliner = lensLifecycleOutliner()
	model.crossOutliner.SetSize(model.viewNav.viewSize())
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, model.crossOutliner)
	return model
}

// lensOperationContext names one saved lens under the picker capability, which
// is the only context these operations are reachable from.
func lensOperationContext(entity operation.EntityIdentity) operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilityLensPicker)
	ctx.CurrentEntity = entity
	return ctx
}

func executeLens(
	model *AppModel,
	id operation.OperationID,
	entity operation.EntityIdentity,
	value string,
	confirmed bool,
) operation.OperationExecutionResult {
	request := operation.OperationExecutionRequest{
		ID:        id,
		Context:   lensOperationContext(entity),
		Confirmed: confirmed,
	}
	if value != "" {
		request.Value = value
		request.HasValue = true
	}
	return model.operations.executor.Execute(request)
}

func machineLenses(t *testing.T, model *AppModel) config.NamedLensList {
	t.Helper()
	machine := model.runtime.Machine()
	if machine == nil {
		t.Fatal("machine-local state is unavailable")
	}
	return machine.NamedLenses
}

func lensNamed(t *testing.T, model *AppModel, name string) config.NamedLens {
	t.Helper()
	for _, saved := range machineLenses(t, model) {
		if saved.Name == name {
			return saved
		}
	}
	t.Fatalf("no saved lens named %q in %v", name, lensNames(machineLenses(t, model)))
	return config.NamedLens{}
}

func lensNames(list config.NamedLensList) []string {
	names := make([]string, 0, len(list))
	for _, saved := range list {
		names = append(names, saved.Name)
	}
	return names
}

// lensLifecycleOutliner builds one two-project cross-project scope whose tasks
// span the states, priorities, and titles the lifecycle assertions narrow on.
func lensLifecycleOutliner() *outliner.OutlinerModel {
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	backlog.Title = "Needle in the backlog"
	doing := primaryRowsTask("doing", models.StateDoing)
	doing.Title = "Unrelated in flight"
	waiting := primaryRowsTask("waiting", models.StateWaiting)
	waiting.Title = "Needle held up"
	waiting.Priority = models.PriorityHigh
	alpha := primaryRowsProject("alpha", backlog, doing, waiting)

	betaWaiting := primaryRowsTask("beta-waiting", models.StateWaiting)
	betaWaiting.Title = "Beta needle"
	betaInbox := primaryRowsTask("beta-inbox", models.StateInbox)
	betaInbox.Title = "Beta triage"
	beta := primaryRowsProject("beta", betaWaiting, betaInbox)

	return scopeTestOutliner(alpha, beta)
}

// waitingLens is the starter-shaped definition the lifecycle tests apply: a
// named mode, a real facet, and a narrowing query.
func waitingLens(id, name, narrowing string) config.NamedLens {
	facets, err := models.ParseQuery("$" + string(models.StateWaiting))
	if err != nil {
		panic(err)
	}
	state := outliner.LensState{
		Mode:   chrome.ModeAll,
		Facets: facets,
		Query:  narrowing,
		Sort:   query.NewSortState(),
	}
	return state.NamedLens(id, name)
}

// lensWithFacet adds one canonical token to a saved definition's narrowing. A
// lens carries one query string rather than a filter struct, so a fixture names
// an extra facet by spelling it rather than by writing a field.
func lensWithFacet(lens config.NamedLens, token string) config.NamedLens {
	lens.Query = strings.TrimSpace(lens.Query + " " + token)
	return lens
}

// lensTestID mints the canonical lowercase hyphenated UUID form the config
// layer is the sole judge of, so the lifecycle tests exercise the same identity
// rule a persisted lens does.
func lensTestID(n int) string {
	return fmt.Sprintf("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d%02d", n)
}

func applyLensOrFatal(t *testing.T, outlinerView *outliner.OutlinerModel, lens config.NamedLens) tea.Cmd {
	t.Helper()
	cmd, err := outlinerView.ApplyNamedLens(lens)
	if err != nil {
		t.Fatalf("ApplyNamedLens(%q): %v", lens.Name, err)
	}
	return cmd
}

// openLensTemplateGallery opens the gallery through the real path: the executor
// asks for a value, and the shell's value step builds the overlay. Nothing here
// names a task, which is the point — the catalog is global.
func openLensTemplateGallery(t *testing.T, model *AppModel) *OperationValuePickerModel {
	t.Helper()
	result := executeLens(model, lens.OperationLensAddTemplate, operation.EntityIdentity{}, "", false)
	if result.ValueRequired == nil {
		t.Fatalf("add template did not ask for a value: %#v", result)
	}
	if cmd := model.openOperationValueStep(*result.ValueRequired); cmd != nil {
		cmd()
	}
	if model.err != nil {
		t.Fatalf("opening the gallery failed: %v", model.err)
	}
	if model.operations.valuePicker == nil {
		t.Fatal("the gallery did not open")
	}
	return model.operations.valuePicker
}

// setMachineLenses replaces the machine-local saved list in memory, which is the
// same seam a commit publishes through.
func setMachineLenses(t *testing.T, model *AppModel, lenses config.NamedLensList) {
	t.Helper()
	machine := cloneMachineState(model.runtime.Machine())
	machine.NamedLenses = lenses
	if err := machine.Validate(); err != nil {
		t.Fatalf("the test machine state is invalid: %v", err)
	}
	model.runtime.SetMachineInMemory(machine)
}

func lensTemplateByName(t *testing.T, name string) config.NamedLens {
	t.Helper()
	for _, template := range config.BuiltinNamedLensTemplates() {
		if template.Name == name {
			return template
		}
	}
	t.Fatalf("no built-in template is named %q", name)
	return config.NamedLens{}
}

// galleryRow finds one template's row by its stable ID rather than by position,
// so an ordering change fails the ordering test alone.
func galleryRow(t *testing.T, picker *OperationValuePickerModel, id string) listsurface.ListItem {
	t.Helper()
	for _, item := range picker.surface.Items() {
		if item.ID == lensTemplateItemID(id) {
			return item
		}
	}
	t.Fatalf("the gallery has no row for template %s", id)
	return listsurface.ListItem{}
}

func lensPickerApp(t *testing.T) *AppModel {
	t.Helper()
	model := lensOperationApp(t)
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	return model
}

// drainLensPickerCmd runs the overlay protocol to quiescence. Only messages the
// operation-overlay plumbing owns are dispatched: a status or reload message is
// the routed view's business and would drag unrelated machinery into a test
// about overlay nesting.
func drainLensPickerCmd(t *testing.T, model *AppModel, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 64 {
			t.Fatal("lens overlay messages did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(msg)...)
		default:
			if handled, follow := model.handleOperationOverlayMessage(msg); handled {
				queue = append(queue, follow)
			}
		}
	}
}

// pressLens sends each named key through the shell's global router, exactly as
// a keystroke arrives in the running app.
func pressLens(t *testing.T, model *AppModel, keys ...string) {
	t.Helper()
	for _, name := range keys {
		key, err := parseUITestKey(name)
		if err != nil {
			t.Fatalf("parse key %q: %v", name, err)
		}
		handled, cmd := model.handleGlobalKeyPress(key)
		if !handled {
			t.Fatalf("key %q was not handled by any overlay", name)
		}
		drainLensPickerCmd(t, model, cmd)
	}
}

func typeLens(t *testing.T, model *AppModel, text string) {
	t.Helper()
	for _, r := range text {
		pressLens(t, model, string(r))
	}
}

func lensPickerRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	items := model.lensPicker.Surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

func lensPickerCursorTitle(t *testing.T, model *AppModel) string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	item, ok := model.lensPicker.Surface.CurrentItem()
	if !ok {
		t.Fatal("the lens picker has no current row")
	}
	return item.PrimaryText
}

func sheetRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.operations.actionSheet == nil {
		t.Fatal("no action sheet is mounted")
	}
	items := model.operations.actionSheet.Surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

// selectLensRow parks the picker cursor on one saved lens by name so a test can
// name its target without depending on how many keys reach it.
func selectLensRow(t *testing.T, model *AppModel, name string) config.NamedLens {
	t.Helper()
	lens := lensNamed(t, model, name)
	if !model.lensPicker.SelectLens(lens.ID) {
		t.Fatalf("lens %q is not listed in the picker", name)
	}
	return lens
}
