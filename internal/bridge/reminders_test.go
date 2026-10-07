package bridge

import (
	"slices"
	"testing"
	"time"
)

// TestBridgePlansReminders holds query.reminders to the shared attention
// rules and to explicit time-zone behavior: which tasks raise a reminder and
// when, how the plan answers acknowledgement, deferral, and changes another
// device made, and which requests it refuses. The harness clock is
// 2026-10-06T09:00Z, which is 05:00 in Toronto and 11:00 in Paris.
func TestBridgePlansReminders(t *testing.T) {
	keep := time.Local
	t.Cleanup(func() { time.Local = keep })

	t.Run("attention_rules", remindersAttentionRules)
	t.Run("time_zone_change", remindersTimeZoneChange)
	t.Run("actions_replan", remindersActionsReplan)
	t.Run("incoming_changes_replan", remindersIncomingChanges)
	t.Run("parked_and_archived_raise_nothing", remindersParkedRaiseNothing)
	t.Run("limit_and_refusals", remindersLimitAndRefusals)
}

const toronto, paris = "America/Toronto", "Europe/Paris"

func (h *harness) reminders(zone, checkInTime string, extra ...func(map[string]any)) remindersResult {
	h.t.Helper()
	payload := map[string]any{"time_zone": zone, "check_in_time": checkInTime}
	for _, apply := range extra {
		apply(payload)
	}
	var plan remindersResult
	h.ok(h.call("query.reminders", payload), &plan)
	return plan
}

// captureTitled creates one Unfiled task and returns its locator.
func (h *harness) captureTitled(fields map[string]any) locator {
	h.t.Helper()
	fields["project_id"] = h.unfiled().ID
	var captured captureResult
	h.ok(h.call("command.capture", fields), &captured)
	return task(captured.Task.ProjectID, captured.Task.TaskID)
}

func (h *harness) setCheckIn(target locator, date string, repeat int) {
	h.t.Helper()
	h.ok(h.call("command.check_in", map[string]any{"tasks": []locator{target}, "action": "set", "date": date, "repeat_days": repeat}), nil)
}

// planned indexes a plan by title and kind.
func planned(plan remindersResult) map[string]reminderView {
	byKey := map[string]reminderView{}
	for _, reminder := range plan.Reminders {
		byKey[reminder.Title+"|"+reminder.Kind] = reminder
	}
	return byKey
}

// expectPlan holds the plan's entries for the named titles to want, as
// title|kind -> fire_at, and nothing else for those titles.
func expectPlan(t *testing.T, plan remindersResult, titles []string, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	for _, reminder := range plan.Reminders {
		if slices.Contains(titles, reminder.Title) {
			got[reminder.Title+"|"+reminder.Kind] = reminder.FireAt
		}
	}
	for key, at := range want {
		if got[key] != at {
			t.Errorf("%s fires at %q, want %q", key, got[key], at)
		}
	}
	for key, at := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected reminder %s at %s", key, at)
		}
	}
	if !slices.IsSortedFunc(plan.Reminders, func(a, b reminderView) int {
		switch {
		case a.FireAt < b.FireAt:
			return -1
		case a.FireAt > b.FireAt:
			return 1
		}
		return 0
	}) {
		t.Errorf("the plan is not soonest first")
	}
}

var attentionTitles = []string{
	"Call the bank", "Owed check-in", "Morning check-in", "Deferred past its check-in", "Finished check-in",
	"Timed deadline", "Date-only deadline", "Flagged for review", "Deferred to a day", "Deadline inside a deferral",
}

// seedAttention captures one task per rule.
func seedAttention(h *harness) map[string]locator {
	h.t.Helper()
	// Dates are typed in the device's zone, so adopt it before typing any.
	h.reminders(toronto, "09:00")
	tasks := map[string]locator{}
	add := func(title string, fields map[string]any) locator {
		fields["title"] = title
		tasks[title] = h.captureTitled(fields)
		return tasks[title]
	}
	h.setCheckIn(add("Call the bank", map[string]any{}), "2026-10-07", 0)
	h.setCheckIn(add("Owed check-in", map[string]any{}), "2026-10-05", 0)
	h.setCheckIn(add("Morning check-in", map[string]any{}), "2026-10-06", 7)
	h.setCheckIn(add("Deferred past its check-in", map[string]any{"defer": "2026-10-08T12:00:00Z"}), "2026-10-07", 0)
	finished := add("Finished check-in", map[string]any{})
	h.setCheckIn(finished, "2026-10-07", 0)
	h.ok(h.call("command.transition", map[string]any{"task": finished, "state": "done"}), nil)
	add("Timed deadline", map[string]any{"due": "2026-10-06 17:30"})
	add("Date-only deadline", map[string]any{"due": "2026-10-07"})
	add("Flagged for review", map[string]any{"review": true})
	add("Deferred to a day", map[string]any{"defer": "2026-10-09"})
	add("Deadline inside a deferral", map[string]any{"due": "2026-10-06 17:30", "defer": "2026-10-07T00:00:00Z"})
	return tasks
}

// remindersAttentionRules: a check-in raises a reminder on its day at the
// reminder time unless it is already owed, deferred past that moment, or
// finished; only a deadline with a time of day raises one, unless a deferral
// covers it; a deferral raises one when it lapses, and a deferral to a day
// raises it that morning; the review flag has no moment and raises nothing.
func remindersAttentionRules(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.openAs("phone-device")
	tasks := seedAttention(h)
	plan := h.reminders(toronto, "09:00")
	if plan.TimeZone != toronto || plan.CheckInTime != "09:00" || plan.GeneratedAt != "2026-10-06T09:00:00Z" {
		t.Fatalf("plan header %+v", plan)
	}
	expectPlan(t, plan, attentionTitles, map[string]string{
		"Call the bank|check_in":                   "2026-10-07T13:00:00Z",
		"Morning check-in|check_in":                "2026-10-06T13:00:00Z",
		"Deferred past its check-in|deferral_ends": "2026-10-08T12:00:00Z",
		"Timed deadline|deadline":                  "2026-10-06T21:30:00Z",
		"Deferred to a day|deferral_ends":          "2026-10-09T13:00:00Z",
		"Deadline inside a deferral|deferral_ends": "2026-10-07T00:00:00Z",
	})
	byKey := planned(plan)
	bank := byKey["Call the bank|check_in"]
	if bank.ID != "check_in/"+tasks["Call the bank"].ProjectID+"/"+tasks["Call the bank"].TaskID || bank.Task != tasks["Call the bank"] {
		t.Errorf("reminder identity %+v", bank)
	}
	if !bank.CanAcknowledge || !bank.CanDefer || bank.LocalTime != "2026-10-07T09:00:00" || bank.ProjectName == "" {
		t.Errorf("check-in reminder %+v", bank)
	}
	if deadline := byKey["Timed deadline|deadline"]; deadline.CanAcknowledge || !deadline.CanDefer || deadline.LocalTime != "2026-10-06T17:30:00" {
		t.Errorf("a deadline with nothing to acknowledge offers %+v", deadline)
	}
	// A deferred task with a schedule may still be acknowledged by name.
	if deferred := byKey["Deferred past its check-in|deferral_ends"]; !deferred.CanAcknowledge {
		t.Errorf("deferral reminder over a scheduled check-in %+v", deferred)
	}
	// Planning reads; it never writes.
	before := h.revisions()
	h.reminders(toronto, "09:00")
	if h.revisions() != before {
		t.Fatal("planning reminders changed the workspace")
	}
	// The same plan twice is the same plan: ids and fingerprints are stable.
	if again := h.reminders(toronto, "09:00"); !slices.Equal(again.Reminders, plan.Reminders) {
		t.Fatal("two plans of an unchanged workspace differ")
	}
	// A later reminder time moves check-ins and day deferrals only.
	later := planned(h.reminders(toronto, "18:15"))
	if got := later["Call the bank|check_in"].FireAt; got != "2026-10-07T22:15:00Z" {
		t.Errorf("check-in at 18:15 fires %s", got)
	}
	if got := later["Timed deadline|deadline"]; got.FireAt != "2026-10-06T21:30:00Z" || got.Fingerprint != byKey["Timed deadline|deadline"].Fingerprint {
		t.Errorf("a reminder time change moved a deadline: %+v", got)
	}
}

// remindersTimeZoneChange: wall-clock reminders follow the device to its new
// zone and keep their wall-clock time; a deferral keeps its instant. The id
// survives a move and the fingerprint names it, so a client replaces rather
// than duplicates.
func remindersTimeZoneChange(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.openAs("phone-device")
	seedAttention(h)
	home := planned(h.reminders(toronto, "09:00"))
	abroad := h.reminders(paris, "09:00")
	if abroad.TimeZone != paris {
		t.Fatalf("plan zone %s", abroad.TimeZone)
	}
	expectPlan(t, abroad, attentionTitles, map[string]string{
		"Call the bank|check_in":                   "2026-10-07T07:00:00Z",
		"Deferred past its check-in|deferral_ends": "2026-10-08T12:00:00Z",
		"Timed deadline|deadline":                  "2026-10-06T15:30:00Z",
		"Deferred to a day|deferral_ends":          "2026-10-09T04:00:00Z",
		"Deadline inside a deferral|deferral_ends": "2026-10-07T00:00:00Z",
	})
	// "Morning check-in" was owed at 09:00 Paris (07:00Z) before the plan's
	// instant (11:00 Paris), so it raises nothing new today; its repeat is
	// only moved by an acknowledgement, never by a plan. The day deferral
	// typed in Toronto is an instant, midnight there and 06:00 here, so it
	// fires at that instant rather than at the Paris reminder time.
	moved := planned(abroad)
	if got := moved["Morning check-in|check_in"]; got.FireAt != "" {
		t.Fatalf("an owed check-in raised %+v", got)
	}
	for _, key := range []string{"Call the bank|check_in", "Timed deadline|deadline"} {
		if moved[key].ID != home[key].ID || moved[key].Fingerprint == home[key].Fingerprint {
			t.Errorf("%s: id %s -> %s, fingerprint %s -> %s", key, home[key].ID, moved[key].ID, home[key].Fingerprint, moved[key].Fingerprint)
		}
	}
	for _, key := range []string{"Deferred past its check-in|deferral_ends", "Deadline inside a deferral|deferral_ends"} {
		if moved[key].Fingerprint != home[key].Fingerprint {
			t.Errorf("an instant moved with the zone: %+v", moved[key])
		}
	}
	if got := moved["Timed deadline|deadline"].LocalTime; got != "2026-10-06T17:30:00" {
		t.Errorf("deadline local time in Paris %s", got)
	}
	// Coming home restores the original plan.
	if back := planned(h.reminders(toronto, "09:00")); back["Timed deadline|deadline"] != home["Timed deadline|deadline"] {
		t.Errorf("returning home planned %+v", back["Timed deadline|deadline"])
	}
}

// remindersActionsReplan: the notification actions are ordinary bridge
// commands, and the next plan reflects what they committed. Acknowledging a
// one-off check-in clears it and a repeating one moves on; deferring for an
// hour or to a day suppresses the check-in and plans the wake-up; an exact
// instant is accepted for a deferral.
func remindersActionsReplan(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.openAs("phone-device")
	tasks := seedAttention(h)
	h.clock.advance(5 * time.Hour) // 14:00Z, 10:00 Toronto: the morning check-in is owed.

	h.ok(h.call("command.check_in", map[string]any{"tasks": []locator{tasks["Morning check-in"]}, "action": "acknowledge"}), nil)
	h.ok(h.call("command.edit", map[string]any{"task": tasks["Call the bank"], "defer": "+1h"}), nil)
	h.ok(h.call("command.edit", map[string]any{"task": tasks["Timed deadline"], "defer": "2026-10-07"}), nil)
	h.ok(h.call("command.edit", map[string]any{"task": tasks["Flagged for review"], "defer": "2026-10-06T14:00:30Z"}), nil)

	expectPlan(t, h.reminders(toronto, "09:00"), attentionTitles, map[string]string{
		// Acknowledged at 10:00 on the 6th, a 7-day repeat is next owed on the 13th.
		"Morning check-in|check_in": "2026-10-13T13:00:00Z",
		// Deferred an hour: back at 15:00Z, and its check-in on the 7th is still ahead.
		"Call the bank|deferral_ends": "2026-10-06T15:00:00Z",
		"Call the bank|check_in":      "2026-10-07T13:00:00Z",
		// Deferred to the next day: covers the 17:30 deadline, wakes that morning.
		"Timed deadline|deferral_ends":             "2026-10-07T13:00:00Z",
		"Flagged for review|deferral_ends":         "2026-10-06T14:00:30Z",
		"Deferred past its check-in|deferral_ends": "2026-10-08T12:00:00Z",
		"Deferred to a day|deferral_ends":          "2026-10-09T13:00:00Z",
		"Deadline inside a deferral|deferral_ends": "2026-10-07T00:00:00Z",
	})
	// Acknowledging a one-off check-in clears it.
	h.ok(h.call("command.check_in", map[string]any{"tasks": []locator{tasks["Call the bank"]}, "action": "acknowledge"}), nil)
	if got := planned(h.reminders(toronto, "09:00"))["Call the bank|check_in"]; got.ID != "" {
		t.Fatalf("an acknowledged one-off still raises %+v", got)
	}
	// A task with nothing to acknowledge refuses the acknowledgement and
	// writes nothing, which is why its notification does not offer it.
	h.unchanged("acknowledging a plain deadline", func() {
		_ = h.refused(h.call("command.check_in", map[string]any{"tasks": []locator{tasks["Date-only deadline"]}, "action": "acknowledge"}), CodeValidation)
	})
	h.rejects(h.call("command.edit", map[string]any{"task": tasks["Timed deadline"], "defer": "not a time"}), CodeValidation)
}

// remindersIncomingChanges: another device's changes reach the plan once
// they are imported, with nothing to do on the phone but replan.
func remindersIncomingChanges(t *testing.T) {
	p := newPair(t)
	signing := task(atlasProject, signingTask)
	notes := task(atlasProject, screenshotsTask)
	before := planned(p.phone.reminders(toronto, "08:30"))

	p.desktop.setCheckIn(signing, "2026-10-08", 0)
	p.desktop.ok(p.desktop.call("command.edit", map[string]any{"task": notes, "due": "2026-10-07 16:45"}), nil)
	// Not imported yet: the phone plans what it holds.
	if got := planned(p.phone.reminders(toronto, "08:30")); len(got) != len(before) {
		t.Fatalf("the phone planned changes it has not imported: %v", got)
	}
	p.settle()
	after := p.phone.reminders(toronto, "08:30")
	byKey := planned(after)
	var signingTitle, notesTitle string
	for _, reminder := range after.Reminders {
		switch reminder.Task {
		case signing:
			signingTitle = reminder.Title
		case notes:
			notesTitle = reminder.Title
		}
	}
	if got := byKey[signingTitle+"|check_in"].FireAt; got != "2026-10-08T12:30:00Z" {
		t.Fatalf("imported check-in fires %q", got)
	}
	if got := byKey[notesTitle+"|deadline"].FireAt; got != "2026-10-07T20:45:00Z" {
		t.Fatalf("imported deadline fires %q", got)
	}

	// The desktop finishes the work: once imported, both reminders go.
	p.desktop.ok(p.desktop.call("command.transition", map[string]any{"task": notes, "state": "done"}), nil)
	p.desktop.ok(p.desktop.call("command.check_in", map[string]any{"tasks": []locator{signing}, "action": "clear"}), nil)
	p.settle()
	for _, reminder := range p.phone.reminders(toronto, "08:30").Reminders {
		if reminder.Task == signing || reminder.Task == notes {
			t.Fatalf("a reminder survived the imported change: %+v", reminder)
		}
	}
}

// remindersParkedRaiseNothing: parked and archived projects are out of the
// task workspace, so their work raises nothing until it returns.
func remindersParkedRaiseNothing(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.openAs("phone-device")
	target := h.captureTitled(map[string]any{"title": "Parked deadline", "due": "2026-10-06 20:00"})
	unfiled := h.unfiled().ID
	project := h.call("command.project", map[string]any{"action": "create", "name": "Garden"})
	var created struct {
		Project projectView `json:"project"`
	}
	h.ok(project, &created)
	h.ok(h.call("command.bulk", map[string]any{"tasks": []locator{target}, "action": "move", "project_id": created.Project.ID}), nil)
	moved := planned(h.reminders(toronto, "09:00"))["Parked deadline|deadline"]
	if moved.Task.ProjectID != created.Project.ID {
		t.Fatalf("moved deadline %+v (unfiled %s)", moved, unfiled)
	}
	h.ok(h.call("command.project", map[string]any{"action": "set_tags", "project_id": created.Project.ID, "tags": []string{"parked"}}), nil)
	if got := planned(h.reminders(toronto, "09:00"))["Parked deadline|deadline"]; got.ID != "" {
		t.Fatalf("a parked project raised %+v", got)
	}
	h.ok(h.call("command.project", map[string]any{"action": "set_tags", "project_id": created.Project.ID, "tags": []string{}}), nil)
	h.ok(h.call("command.project", map[string]any{"action": "archive", "project_id": created.Project.ID}), nil)
	if got := planned(h.reminders(toronto, "09:00"))["Parked deadline|deadline"]; got.ID != "" {
		t.Fatalf("an archived project raised %+v", got)
	}
}

// remindersLimitAndRefusals: the plan is bounded and says when it was cut;
// a request without a zone or with an impossible time is refused.
func remindersLimitAndRefusals(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.openAs("phone-device")
	seedAttention(h)
	full := h.reminders(toronto, "09:00")
	cut := h.reminders(toronto, "09:00", func(p map[string]any) { p["limit"] = 2 })
	if !cut.Truncated || len(cut.Reminders) != 2 || !slices.Equal(cut.Reminders, full.Reminders[:2]) {
		t.Fatalf("limit 2 planned %d (truncated %v)", len(cut.Reminders), cut.Truncated)
	}
	if full.Truncated {
		t.Fatal("the default limit cut the plan")
	}
	near := h.reminders(toronto, "09:00", func(p map[string]any) { p["horizon_days"] = 1 })
	for _, reminder := range near.Reminders {
		if reminder.FireAt > "2026-10-07T09:00:00Z" {
			t.Fatalf("a one-day horizon planned %+v", reminder)
		}
	}
	h.rejects(h.call("query.reminders", map[string]any{"check_in_time": "09:00"}), CodeInvalidRequest)
	h.rejects(h.call("query.reminders", map[string]any{"time_zone": "Mars/Olympus", "check_in_time": "09:00"}), CodeInvalidRequest)
	_ = h.refused(h.call("query.reminders", map[string]any{"time_zone": toronto, "check_in_time": "25:00"}), CodeValidation)
	h.rejects(h.call("query.reminders", map[string]any{"time_zone": toronto, "check_in_time": "09:00", "limit": 65}), CodeInvalidRequest)
}
