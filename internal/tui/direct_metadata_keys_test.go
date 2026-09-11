package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func TestDirectMetadataKeysCarryTheirPresetsAndOpenTheirPickers(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	loose := project.GetTaskByID("loose")

	presets := []struct {
		path  []string
		want  operation.OperationID
		value any
	}{
		{path: []string{"!", "0"}, want: operation.OperationTaskSetPriority, value: 0},
		{path: []string{"!", "1"}, want: operation.OperationTaskSetPriority, value: 1},
		{path: []string{"!", "3"}, want: operation.OperationTaskSetPriority, value: 3},
		{path: []string{"w", "i"}, want: operation.OperationTaskSetState, value: models.StateInbox},
		{path: []string{"w", "g"}, want: operation.OperationTaskSetState, value: models.StateDoing},
		{path: []string{"w", "w"}, want: operation.OperationTaskSetState, value: models.StateWaiting},
		{path: []string{"w", "p"}, want: operation.OperationTaskSetState, value: models.StatePaused},
		{path: []string{"w", "m"}, want: operation.OperationTaskSetState, value: models.StateSomeday},
		{path: []string{"w", "d"}, want: operation.OperationTaskSetState, value: models.StateDone},
		{path: []string{"w", "c"}, want: operation.OperationTaskSetState, value: models.StateCanceled},
	}

	for _, preset := range presets {
		single := taskOperationTestContext(loose, project)
		bulk := bulkTaskActionTestContext(project, loose, project.GetTaskByID("story"))
		for name, ctx := range map[string]operation.CommandContext{"single": single, "bulk": bulk} {
			binding, descriptor, resolution := model.operations.Registry.InputBindingForSequence(
				operation.InputStrategySimplified,
				preset.path,
				ctx,
			)
			if resolution != operation.ChordTerminal {
				t.Fatalf("%s %v resolution = %v, want terminal", name, preset.path, resolution)
			}
			if descriptor.ID != preset.want {
				t.Errorf("%s %v resolved %s, want %s", name, preset.path, descriptor.ID, preset.want)
			}
			if !binding.HasPresetValue || binding.PresetValue != preset.value {
				t.Errorf("%s %v preset = %v, want %v", name, preset.path, binding.PresetValue, preset.value)
			}
			result := executeDirectPath(t, model, ctx, preset.path)
			if result.Err != nil {
				t.Errorf("%s %v: %v", name, preset.path, result.Err)
				continue
			}
			// A preset chord already knows its value, so it must never stop at a picker.
			if result.ValueRequired != nil {
				t.Errorf("%s %v asked for a value it already presets", name, preset.path)
			}
			if result.Message == nil {
				t.Errorf("%s %v produced no coordinator work", name, preset.path)
			}
		}
	}

	for _, action := range []struct {
		path []string
		want operation.OperationID
	}{
		{path: []string{"w", "b"}, want: operation.OperationCoreBacklog},
		{path: []string{"w", "n"}, want: operation.OperationCoreNext},
		{path: []string{"w", "t"}, want: operation.OperationCoreToday},
	} {
		ctx := taskOperationTestContext(loose, project)
		binding, descriptor, resolution := model.operations.Registry.InputBindingForSequence(
			operation.InputStrategySimplified,
			action.path,
			ctx,
		)
		if resolution != operation.ChordTerminal || descriptor.ID != action.want {
			t.Errorf("%v resolved to %s/%v, want %s/terminal", action.path, descriptor.ID, resolution, action.want)
		}
		if binding.HasPresetValue {
			t.Errorf("%v unexpectedly carries preset %v", action.path, binding.PresetValue)
		}
	}

	// The two unpresetable metadata keys keep the existing pickers. This file is
	// scoped to the metadata keys — `!`, `w`, `@`, `#`, the workflow chord named
	// by the letter it is pressed with rather than the `$` it writes — which is why the
	// relationship keys `>`, `<`, and `~` are absent rather than forgotten: each
	// opens a picker proved in its own file, `~` in
	// operation_task_reference_picker_test.go. Adding a relationship key here
	// would make the table a second, partial copy of the direct-grammar audit
	// direct_grammar_test.go already owns exhaustively.
	for _, picker := range []struct {
		key      string
		want     operation.OperationID
		provider operation.ValueProviderID
	}{
		{key: "@", want: operation.OperationTaskSetDueDate, provider: operation.ValueProviderDate},
		{key: "#", want: operation.OperationTaskSetTags, provider: operation.ValueProviderTags},
	} {
		ctx := taskOperationTestContext(loose, project)
		result := executeDirectPath(t, model, ctx, []string{picker.key})
		if result.Err != nil {
			t.Errorf("%s: %v", picker.key, result.Err)
			continue
		}
		if result.ValueRequired == nil {
			t.Errorf("%s did not request its %s value", picker.key, picker.provider)
			continue
		}
		if result.ValueRequired.Provider != picker.provider {
			t.Errorf(
				"%s requested provider %s, want %s",
				picker.key,
				result.ValueRequired.Provider,
				picker.provider,
			)
		}
	}
}

func TestDirectMetadataKeysDispatchFromTheOutliner(t *testing.T) {
	newApp := func(t *testing.T) *AppModel {
		t.Helper()
		app := newUITestApp()
		task := &models.Task{ID: "task", ShortID: 1, Title: "Task", WorkflowState: models.StateBacklog}
		project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
		project.BuildTaskTree()
		setTestAppProjects(app, project)
		app.crossOutliner.Population.Projects = []*models.Project{project}
		app.crossOutliner.Rows = []outliner.OutlinerRow{{Task: task, ProjectID: project.ID}}
		return app
	}

	t.Run("chord prefix waits and names its family", func(t *testing.T) {
		app := newApp(t)
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
		if !handled || cmd != nil {
			t.Fatalf("! = handled %v cmd %T; want a pending prefix", handled, cmd)
		}
		row := pendingChordHintRowForTest(app)
		for _, want := range []string{"0 unset", "1 high", "3 low"} {
			if !strings.Contains(row, want) {
				t.Errorf("which-key row %q omits %q", row, want)
			}
		}
	})

	t.Run("terminal chord dispatches a mutation", func(t *testing.T) {
		app := newApp(t)
		app.handleGlobalKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: '0', Text: "0"})
		if !handled || cmd == nil {
			t.Fatalf("!0 = handled %v cmd %T; want the clear-priority command", handled, cmd)
		}
		if len(app.operations.Chord) != 0 {
			t.Errorf("pending chord = %v, want cleared", app.operations.Chord)
		}
	})

	t.Run("state prefix shows every action and dispatches a transition", func(t *testing.T) {
		app := newApp(t)
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'w', Text: "w"})
		if !handled || cmd != nil {
			t.Fatalf("w = handled %v cmd %T; want a pending prefix", handled, cmd)
		}
		row := pendingChordHintRowForTest(app)
		for _, want := range []string{
			"i inbox",
			"b backlog",
			"n next",
			"t today",
			"g doing",
			"w waiting",
			"p paused",
			"m someday",
			"d done",
			"c canceled",
		} {
			if !strings.Contains(row, want) {
				t.Errorf("which-key row %q omits %q", row, want)
			}
		}

		handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'w', Text: "w"})
		if !handled || cmd == nil {
			t.Fatalf("ww = handled %v cmd %T; want the waiting transition", handled, cmd)
		}
	})

	// The metadata keys that open a picker, for the same reason as the table
	// above: `>`, `<`, and `~` are relationship keys and prove their live
	// dispatch in their own files.
	for _, picker := range []struct {
		key      rune
		provider operation.ValueProviderID
	}{
		{key: '@', provider: operation.ValueProviderDate},
		{key: '#', provider: operation.ValueProviderTags},
	} {
		t.Run(string(picker.key)+" opens its picker", func(t *testing.T) {
			app := newApp(t)
			handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: picker.key, Text: string(picker.key)})
			if !handled {
				t.Fatalf("%q was not handled", picker.key)
			}
			if cmd != nil {
				app.handleOperationOverlayMessage(cmd())
			}
			if app.operations.ValuePicker == nil || app.operations.ValuePicker.Provider() != picker.provider {
				t.Fatalf("%q did not open the %s picker: %#v", picker.key, picker.provider, app.operations.ValuePicker)
			}
		})
	}
}
