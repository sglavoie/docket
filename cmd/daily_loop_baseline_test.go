package cmd

import (
	"encoding/json"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/sglavoie/tt/internal/models"
)

func baselineFlagNames(cmd *cobra.Command) []string {
	var names []string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)
	})
	sort.Strings(names)
	return names
}

func baselineHasFlag(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Lookup(name) != nil
}

// TestDailyLoopCurrentCreationAndFilterFlags characterizes the transitional CLI
// surface. Creation/template/archive still speak section or status, while task
// list already exposes state beside status. Later public-cutover sessions replace
// this baseline with the state-only contract in testutil.
func TestDailyLoopCurrentCreationAndFilterFlags(t *testing.T) {
	archiveList := newTaskArchiveListCmd()
	tests := []struct {
		name    string
		command *cobra.Command
		present []string
		absent  []string
	}{
		{name: "task add", command: taskAddCmd, present: []string{"section"}, absent: []string{"state"}},
		{name: "template add", command: templateAddCmd, present: []string{"section"}, absent: []string{"state"}},
		{name: "task list", command: taskListCmd, present: []string{"status", "state", "sort"}},
		{name: "archive list", command: archiveList, present: []string{"status"}, absent: []string{"state"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, flag := range tc.present {
				if !baselineHasFlag(tc.command, flag) {
					t.Errorf("current flags %v are missing %q", baselineFlagNames(tc.command), flag)
				}
			}
			for _, flag := range tc.absent {
				if baselineHasFlag(tc.command, flag) {
					t.Errorf("current flags %v unexpectedly contain %q", baselineFlagNames(tc.command), flag)
				}
			}
		})
	}
}

// TestDailyLoopCurrentTaskListJSONShape pins representative legacy CLI output:
// state is present, but status and section remain public compatibility fields
// until the atomic cutover. Stage is omitted when empty.
func TestDailyLoopCurrentTaskListJSONShape(t *testing.T) {
	at := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:            "baseline-task",
		ShortID:       7,
		Type:          models.TaskTypeTask,
		Title:         "Baseline task",
		WorkflowState: models.StateNext,
		Priority:      models.PriorityDefault,
		Tags:          []string{"baseline"},
		CreatedAt:     at,
		UpdatedAt:     at,
	}

	data, err := json.Marshal(taskListJSONEntries([]*models.Task{task})[0])
	if err != nil {
		t.Fatalf("marshal task list entry: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode task list entry: %v", err)
	}

	var got []string
	for key := range fields {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{
		"created_at", "dimmed", "due_date", "has_time", "id", "parent_id",
		"parked", "pinned", "priority", "queued", "section", "short_id",
		"state", "status", "tags", "title", "type", "updated_at",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("task list JSON fields = %v, want %v", got, want)
	}
	if string(fields["status"]) != `"todo"` || string(fields["state"]) != `"next"` || string(fields["section"]) != `"Next"` {
		t.Errorf("representative lifecycle output status=%s state=%s section=%s", fields["status"], fields["state"], fields["section"])
	}
}
