package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// The combined date picker opens in Due, so the deferral has no say in where
// its cursor starts. These cases pin the three answers that used to depend on
// it: a selection that agrees about due but not about defer still opens on the
// shared due preset, a stored deferral alone never drags the cursor into the
// defer section, and a selection that disagrees about due opens on Due's Clear
// row rather than nowhere in particular.
func TestCombinedDatePickerInitialCursorReadsTheDueFieldAlone(t *testing.T) {
	today := util.StartOfDay(time.Now())
	tomorrow := today.AddDate(0, 0, 1)
	week := today.AddDate(0, 0, 7)
	arbitrary := today.Add(37 * time.Hour)

	cases := []struct {
		name  string
		dates [][2]*time.Time // {due, deferred} per selected task
		want  listsurface.ListItemID
	}{
		{
			name:  "one task with a due preset and a deferral opens on the due preset",
			dates: [][2]*time.Time{{&today, &week}},
			want:  "date:due:today",
		},
		{
			name:  "a shared due preset survives disagreeing deferrals",
			dates: [][2]*time.Time{{&tomorrow, &week}, {&tomorrow, nil}},
			want:  "date:due:tomorrow",
		},
		{
			name:  "a deferral with no due date still opens on Due's Clear row",
			dates: [][2]*time.Time{{nil, &week}},
			want:  "date:due:clear",
		},
		{
			name:  "disagreeing due dates open on Due's Clear row",
			dates: [][2]*time.Time{{&today, nil}, {&tomorrow, nil}},
			want:  "date:due:clear",
		},
		{
			name:  "a due date no preset reproduces opens on Due's Clear row",
			dates: [][2]*time.Time{{&arbitrary, nil}},
			want:  "date:due:clear",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			groups := make([][]*models.Task, 0, len(testCase.dates))
			tasks := make([]*models.Task, 0, len(testCase.dates))
			for index, dates := range testCase.dates {
				task := newBulkPickerTestTask(t, index+1, "dated task")
				task.DueDate = dates[0]
				task.DeferredUntil = dates[1]
				tasks = append(tasks, task)
				groups = append(groups, []*models.Task{task})
			}
			env := newAppBulkPickerEnv(t, groups...)
			model := env.appModel()

			context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
			context.Capabilities.Add(operation.CapabilitySimplifiedInput)
			context.Selection.Visual = len(tasks) > 1
			for index, task := range tasks {
				project := env.projects[index]
				context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
				context.Selection.Entities = append(
					context.Selection.Entities, outliner.TaskOperationEntity(task, project),
				)
			}
			context.Entity = context.Selection.Entities[0]

			picker, err := model.newOperationValuePicker(
				operation.OperationExecutionRequest{
					ID:      operation.OperationTaskSetDueDate,
					Context: context,
				},
				operation.ValueProviderDate,
			)
			if err != nil {
				t.Fatalf("newOperationValuePicker: %v", err)
			}
			item, ok := picker.Surface().CurrentItem()
			if !ok {
				t.Fatal("the picker opened with no row under its cursor")
			}
			if item.ID != testCase.want {
				t.Fatalf("opening row = %q, want %q", item.ID, testCase.want)
			}
			// Opening a picker is not an edit: the highlight is where the cursor
			// starts, not a value anything has agreed to write.
			if ids := picker.Surface().SelectedIDs(); len(ids) != 0 {
				t.Fatalf("opening the picker selected %v, want nothing", ids)
			}
			for index, task := range tasks {
				if task.DueDate != testCase.dates[index][0] {
					t.Fatalf("opening the picker rewrote task %d's due date", index)
				}
				if task.DeferredUntil != testCase.dates[index][1] {
					t.Fatalf("opening the picker rewrote task %d's deferral", index)
				}
			}
		})
	}
}
