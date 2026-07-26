package actionplan

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/sglavoie/tt/internal/models"
)

func TestActionPlanValidationRejectsTerminalControlSequences(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"seven-bit CSI": "clear\x1b[2Jspoof",
		"seven-bit OSC": "title\x1b]0;spoof\x07",
		"eight-bit CSI": "clear\u009b2Jspoof",
		"eight-bit OSC": "title\u009d0;spoof\u009c",
	}
	for name, summary := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan := ActionPlan{Version: Version, Summary: summary, Actions: []Action{}}
			err := Validate(plan)
			if err == nil {
				t.Fatal("Validate unexpectedly accepted terminal control sequence")
			}
			if !strings.Contains(err.Error(), "summary contains unsafe terminal control character") {
				t.Fatalf("Validate error = %q, want unsafe summary control error", err)
			}
		})
	}
}

func TestDisplaySafetyValidationCoversEveryProviderDisplayField(t *testing.T) {
	t.Parallel()

	title := func(value string) *string { return &value }
	taskType := func(value models.TaskType) *models.TaskType { return &value }
	state := func(value models.WorkflowState) *models.WorkflowState { return &value }
	tests := map[string]struct {
		plan      ActionPlan
		wantField string
	}{
		"summary CSI": {
			plan:      ActionPlan{Version: Version, Summary: "clear\x1b[2J", Actions: []Action{}},
			wantField: "summary",
		},
		"action type format": {
			plan:      ActionPlan{Version: Version, Actions: []Action{{Type: ActionType("create_task\u202e")}}},
			wantField: "type",
		},
		"project reference C1": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionCreateTask, ProjectID: "project\u009b", Title: title("safe"),
			}}},
			wantField: "project_id",
		},
		"task reference bidi": {
			plan:      ActionPlan{Version: Version, Actions: []Action{{Type: ActionDeleteTask, TaskID: "task\u2066"}}},
			wantField: "task_id",
		},
		"title OSC": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionCreateTask, ProjectID: "project", Title: title("title\x1b]0;spoof\x07"),
			}}},
			wantField: "title",
		},
		"description BEL": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionEditTask, TaskID: "task", Description: title("ring\x07bell"),
			}}},
			wantField: "description",
		},
		"description carriage return": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionEditTask, TaskID: "task", Description: title("replace\rline"),
			}}},
			wantField: "description",
		},
		"task type format": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionCreateTask, ProjectID: "project", Title: title("safe"), TaskType: taskType(models.TaskType("task\u202e")),
			}}},
			wantField: "task_type",
		},
		"state format": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionSetState, TaskID: "task", State: state(models.WorkflowState("ready\u2069")),
			}}},
			wantField: "state",
		},
		"due text format": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionSetDueDate, TaskID: "task", Due: title("2026-07-14\u200e"),
			}}},
			wantField: "due",
		},
		"tag bidi": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionSetTags, TaskID: "task", Tags: []string{"safe\u202e"},
			}}},
			wantField: "tags[0]",
		},
		"blocker reference format": {
			plan: ActionPlan{Version: Version, Actions: []Action{{
				Type: ActionAddDependency, TaskID: "task", BlockerTaskID: "blocker\u2067",
			}}},
			wantField: "blocker_task_id",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tc.plan)
			if err == nil {
				t.Fatal("Validate unexpectedly accepted unsafe display text")
			}
			if !strings.Contains(err.Error(), tc.wantField+" contains unsafe terminal") {
				t.Fatalf("Validate error = %q, want unsafe %s error", err, tc.wantField)
			}
		})
	}
}

func TestDisplaySafetyAllowsMultilineMarkdownDescription(t *testing.T) {
	t.Parallel()

	description := "## Notes\n\t- keep **Markdown** and emoji ✅"
	plan := ActionPlan{Version: Version, Actions: []Action{{
		Type: ActionEditTask, TaskID: "task", Description: &description,
	}}}
	if err := Validate(plan); err != nil {
		t.Fatalf("Validate rejected safe multiline description: %v", err)
	}
}

func TestCreateTaskStateIsOptionalAndValidated(t *testing.T) {
	t.Parallel()

	title := "Create me"
	valid := models.StateNext
	for _, action := range []Action{
		{Type: ActionCreateTask, ProjectID: "project", Title: &title},
		{Type: ActionCreateTask, ProjectID: "project", Title: &title, State: &valid},
	} {
		if err := Validate(ActionPlan{Version: Version, Actions: []Action{action}}); err != nil {
			t.Fatalf("Validate rejected create_task state contract: %v", err)
		}
	}

	invalid := models.WorkflowState("backlog")
	err := Validate(ActionPlan{Version: Version, Actions: []Action{{
		Type: ActionCreateTask, ProjectID: "project", Title: &title, State: &invalid,
	}}})
	if err == nil || !strings.Contains(err.Error(), `invalid state "backlog"`) {
		t.Fatalf("Validate error = %v, want invalid state rejection", err)
	}
}

func TestCreateTaskRejectsLegacyLifecycleFields(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"section", "status"} {
		payload := `{"version":2,"summary":"","actions":[{"type":"create_task","project_id":"project","title":"Create me","state":"ready","` + field + `":"legacy"}]}`
		var plan ActionPlan
		err := json.Unmarshal([]byte(payload), &plan)
		if err == nil || !strings.Contains(err.Error(), `unknown field "`+field+`"`) {
			t.Fatalf("%s payload error = %v, want unknown-field rejection", field, err)
		}
	}
}

func TestTerminalSafeDisplayTextEscapesControlsAndBidi(t *testing.T) {
	t.Parallel()

	unsafe := "CSI\x1b[2J OSC\x1b]0;x\x07 CR\r C1\u009b bidi\u202e"
	for _, mode := range []DisplayTextMode{DisplayTextSingleLine, DisplayTextMultiline} {
		got := TerminalSafeDisplayText(unsafe, mode)
		for _, r := range got {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				t.Fatalf("mode %d rendered unsafe U+%04X: %q", mode, r, got)
			}
		}
		for _, want := range []string{`\x1b[2J`, `\x1b]0;x\a`, `\r`, `\u009b`, `\u202e`} {
			if !strings.Contains(got, want) {
				t.Fatalf("mode %d output = %q, want visible escape %q", mode, got, want)
			}
		}
	}

	text := "first\n\tsecond"
	if got := TerminalSafeDisplayText(text, DisplayTextSingleLine); got != `first\n\tsecond` {
		t.Fatalf("single-line output = %q", got)
	}
	if got := TerminalSafeDisplayText(text, DisplayTextMultiline); got != text {
		t.Fatalf("multiline output = %q, want %q", got, text)
	}
}
