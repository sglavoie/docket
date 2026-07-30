package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// executeDirectPath resolves one strategy key path in ctx and runs it exactly as
// handleSimplifiedInputKey does, so the preset the grammar declares is the value
// the executor receives.
func executeDirectPath(
	t *testing.T,
	model *AppModel,
	ctx CommandContext,
	path []string,
) OperationExecutionResult {
	t.Helper()
	binding, descriptor, resolution := model.operationRegistry.InputBindingForSequence(
		InputStrategySimplified,
		path,
		ctx,
	)
	if resolution != ChordTerminal {
		t.Fatalf("key path %v resolution = %v, want terminal", path, resolution)
	}
	return model.operationExecutor.Execute(OperationExecutionRequest{
		ID:       descriptor.ID,
		Context:  ctx,
		Source:   OperationSourceDirectKey,
		Value:    binding.PresetValue,
		HasValue: binding.HasPresetValue,
	})
}

func TestDirectMetadataKeysCarryTheirPresetsAndOpenTheirPickers(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	loose := project.GetTaskByID("loose")

	presets := []struct {
		path  []string
		want  OperationID
		value any
	}{
		{path: []string{"p", "0"}, want: operationTaskSetPriority, value: 0},
		{path: []string{"p", "1"}, want: operationTaskSetPriority, value: 1},
		{path: []string{"p", "5"}, want: operationTaskSetPriority, value: 5},
		{path: []string{",", "i"}, want: operationTaskSetState, value: models.StateInbox},
		{path: []string{",", "w"}, want: operationTaskSetState, value: models.StateWaiting},
		{path: []string{",", "p"}, want: operationTaskSetState, value: models.StatePaused},
		{path: []string{",", "s"}, want: operationTaskSetState, value: models.StateSomeday},
		{path: []string{",", "c"}, want: operationTaskSetState, value: models.StateCanceled},
	}

	for _, preset := range presets {
		single := taskOperationTestContext(loose, project)
		bulk := bulkTaskActionTestContext(project, loose, project.GetTaskByID("story"))
		for name, ctx := range map[string]CommandContext{"single": single, "bulk": bulk} {
			binding, descriptor, resolution := model.operationRegistry.InputBindingForSequence(
				InputStrategySimplified,
				preset.path,
				ctx,
			)
			if resolution != ChordTerminal {
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

	// The two unpresetable operations keep the existing pickers.
	for _, picker := range []struct {
		key      string
		want     OperationID
		provider ValueProviderID
	}{
		{key: "@", want: operationTaskSetDueDate, provider: ValueProviderDueDate},
		{key: "#", want: operationTaskSetTags, provider: ValueProviderTags},
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
	newApp := func(t *testing.T) (*AppModel, *models.Task) {
		t.Helper()
		app := newUITestApp()
		task := &models.Task{ID: "task", ShortID: 1, Title: "Task", WorkflowState: models.StateReady}
		project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
		project.BuildTaskTree()
		setTestAppProjects(app, project)
		app.crossOutliner.loadedProjects = []*models.Project{project}
		app.crossOutliner.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
		return app, task
	}

	t.Run("chord prefix waits and names its family", func(t *testing.T) {
		app, _ := newApp(t)
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'p', Text: "p"})
		if !handled || cmd != nil {
			t.Fatalf("p = handled %v cmd %T; want a pending prefix", handled, cmd)
		}
		row := app.pendingChordHintRow()
		for _, want := range []string{"0 clear", "1 critical", "5 lowest"} {
			if !strings.Contains(row, want) {
				t.Errorf("which-key row %q omits %q", row, want)
			}
		}
	})

	t.Run("terminal chord dispatches a mutation", func(t *testing.T) {
		app, _ := newApp(t)
		app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'p', Text: "p"})
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: '0', Text: "0"})
		if !handled || cmd == nil {
			t.Fatalf("p0 = handled %v cmd %T; want the clear-priority command", handled, cmd)
		}
		if len(app.pendingChord) != 0 {
			t.Errorf("pending chord = %v, want cleared", app.pendingChord)
		}
	})

	t.Run("state chord dispatches a transition", func(t *testing.T) {
		app, _ := newApp(t)
		app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ',', Text: ","})
		handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'w', Text: "w"})
		if !handled || cmd == nil {
			t.Fatalf(",w = handled %v cmd %T; want the waiting transition", handled, cmd)
		}
	})

	for _, picker := range []struct {
		key      rune
		provider ValueProviderID
	}{
		{key: '@', provider: ValueProviderDueDate},
		{key: '#', provider: ValueProviderTags},
	} {
		t.Run(string(picker.key)+" opens its picker", func(t *testing.T) {
			app, _ := newApp(t)
			handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: picker.key, Text: string(picker.key)})
			if !handled {
				t.Fatalf("%q was not handled", picker.key)
			}
			if cmd != nil {
				app.handleOperationOverlayMessage(cmd())
			}
			if app.operationValuePicker == nil || app.operationValuePicker.provider != picker.provider {
				t.Fatalf("%q did not open the %s picker: %#v", picker.key, picker.provider, app.operationValuePicker)
			}
		})
	}
}
