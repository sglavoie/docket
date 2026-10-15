package valuepicker

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func nowLineRefs(tasks ...*models.Task) []taskref.TaskProjectRef {
	refs := make([]taskref.TaskProjectRef, 0, len(tasks))
	for _, task := range tasks {
		refs = append(refs, taskref.TaskProjectRef{Task: task})
	}
	return refs
}

func TestDateFieldsNowLineNamesOneTasksStoredValues(t *testing.T) {
	now := time.Date(2026, 10, 15, 10, 0, 0, 0, time.Local)
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.Local)
	deferral := time.Date(2026, 10, 16, 9, 0, 0, 0, time.Local)
	checkIn := time.Date(2026, 10, 22, 0, 0, 0, 0, time.Local)
	lapsed := now.Add(-time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"nothing stored", &models.Task{}, ""},
		{
			"every field",
			&models.Task{DueDate: &due, DeferredUntil: &deferral, NextCheckIn: &checkIn, CheckInRepeatDays: 7},
			"Now: due Tue Oct 20 · defer Fri Oct 16 09:00 · check-in Thu Oct 22 every 7d",
		},
		{
			"a lapsed deferral and a one-off check-in",
			&models.Task{DeferredUntil: &lapsed, NextCheckIn: &checkIn},
			"Now: due none · defer none · check-in Thu Oct 22",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := DateFieldsNowLine(nowLineRefs(testCase.task), now); got != testCase.want {
				t.Fatalf("now line = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A selection names what it shares and says mixed where it disagrees, so one
// target's value is never presented as everyone's.
func TestDateFieldsNowLineSaysMixedWhereASelectionDisagrees(t *testing.T) {
	now := time.Date(2026, 10, 15, 10, 0, 0, 0, time.Local)
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.Local)
	checkIn := time.Date(2026, 10, 22, 0, 0, 0, 0, time.Local)

	got := DateFieldsNowLine(nowLineRefs(
		&models.Task{DueDate: &due, NextCheckIn: &checkIn, CheckInRepeatDays: 7},
		&models.Task{DueDate: &due, NextCheckIn: &checkIn, CheckInRepeatDays: 14},
	), now)
	if want := "Now: due Tue Oct 20 · defer none · check-in mixed"; got != want {
		t.Fatalf("now line = %q, want %q", got, want)
	}
	// Agreement is judged on the instant, not on the printed words: a later year
	// can print the same weekday and day.
	later := due.AddDate(11, 0, 0) // Tue Oct 20 2037
	got = DateFieldsNowLine(nowLineRefs(&models.Task{DueDate: &due}, &models.Task{DueDate: &later}), now)
	if !strings.HasPrefix(got, "Now: due mixed") {
		t.Fatalf("now line = %q, want the due field mixed", got)
	}
}

// The line sits under the title, and the list gives up the rows it wraps to
// rather than the overlay growing past its budget.
func TestDatePickerContextLineRendersAndIsReserved(t *testing.T) {
	picker := newDatePickerForTest(t, "")
	without := picker.RenderWithin(60, 20)
	picker.SetContextLine("Now: due none · defer none · check-in Thu Oct 22 every 7d")
	with := picker.RenderWithin(60, 20)
	plain := strings.Join(strings.Fields(ansi.Strip(with)), " ")
	if !strings.Contains(plain, "Now: due none") || !strings.Contains(plain, "every 7d") {
		t.Fatalf("context line missing or cut:\n%s", ansi.Strip(with))
	}
	if got, want := strings.Count(with, "\n"), strings.Count(without, "\n"); got != want {
		t.Fatalf("the context line grew the overlay from %d to %d rows", want+1, got+1)
	}
}

func dateItemByID(t *testing.T, picker *OperationValuePickerModel, id listsurface.ListItemID) listsurface.ListItem {
	t.Helper()
	for _, item := range picker.Surface().Items() {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("no row %q", id)
	return listsurface.ListItem{}
}

// An empty field keeps its Clear row listed but refuses it by name, the picker
// opens on Today, and Tab steps over a disabled Clear onto the section's first
// committable row.
func TestEmptyFieldClearRowsAreDisabledAndTabStepsOverThem(t *testing.T) {
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetDueDate},
		operation.ValueProviderDate,
		"Set Due, Defer, or Check-In",
		"Task",
		OperationDateListSource("", DateFieldsSet{}),
		false,
		OperationDateSelectedID(nil, true),
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "today"); got != want {
		t.Fatalf("opening row = %q, want %q", got, want)
	}
	for _, deferred := range []bool{false, true} {
		item := dateItemByID(t, picker, DateListItemID(deferred, "clear"))
		if item.Enabled() || item.DisabledReason != DateClearNothingSetReason {
			t.Fatalf("clear row (deferred=%v) reason = %q, want disabled with %q", deferred, item.DisabledReason, DateClearNothingSetReason)
		}
	}
	dismissed, result := pressKey(picker, "0")
	message, isStatus := result.(status.StatusMsg)
	if dismissed || !isStatus || !strings.Contains(string(message), DateClearNothingSetReason) {
		t.Fatalf("0 on an empty due field = %#v (dismissed %v), want a refusal", result, dismissed)
	}
	pressKey(picker, "tab")
	if got, want := currentDateRowID(t, picker), DateListItemID(true, "1h"); got != want {
		t.Fatalf("tab into Defer = %q, want the first enabled row %q", got, want)
	}
	pressKey(picker, "shift+tab")
	if got, want := currentDateRowID(t, picker), DateListItemID(false, "today"); got != want {
		t.Fatalf("shift+tab into Due = %q, want %q", got, want)
	}
}

// A field one target of a selection holds keeps its Clear row committable.
func TestClearRowStaysEnabledWhenItsFieldHoldsAValue(t *testing.T) {
	source := OperationDateListSource("", DateFieldsSet{Due: true})
	for _, item := range source.Items {
		switch item.ID {
		case DateListItemID(false, "clear"):
			if !item.Enabled() {
				t.Fatalf("due clear disabled with a due date set: %q", item.DisabledReason)
			}
		case DateListItemID(true, "clear"):
			if item.Enabled() {
				t.Fatal("defer clear enabled with no deferral set")
			}
		}
	}
}
