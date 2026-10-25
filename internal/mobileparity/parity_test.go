// Package mobileparity_test holds the desktop and the phone to one durable
// outcome. Each scenario runs twice over two private copies of the
// representative workspace under the same clock and identity sequence: once the
// way the desktop does it — the composer's capture reading, the inline editor's
// quick-edit grammar, the resume prompt, the workflow chord's planning and
// lifecycle calls — and once through the mobile bridge's JSON protocol. The two
// workspaces must then hold byte-identical task Markdown and planning state, and
// a refusal must refuse on both sides and change nothing on either.
package mobileparity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestDesktopAndMobileCaptureAndEditAlike(t *testing.T) {
	t.Run("shorthand_capture", parityShorthandCapture)
	t.Run("shorthand_preview_matches_the_capture", parityShorthandPreviewMatchesTheCapture)
	t.Run("shorthand_refusal_names_the_line_and_commits_nothing", parityShorthandRefusalNamesTheLineAndCommitsNothing)
	t.Run("structured_capture", parityStructuredCapture)
	t.Run("daily_edit", parityDailyEdit)
	t.Run("deferral_routes_to_backlog", parityDeferralRoutesToBacklog)
	t.Run("today_next_exclusivity", parityTodayNextExclusivity)
	t.Run("completion_clears_planning", parityCompletionClearsPlanning)
	t.Run("lifecycle_refusals", parityLifecycleRefusals)
}

func parityShorthandCapture(t *testing.T) {
	p := newPair(t)
	text := "Plan the offsite !1 #ops ##work @2026-10-20 *pin =review $next\n\n" +
		"Book the venue %atlas-launch #ops $today\n" +
		"Renew passport @@2026-10-09 ;bring old photo;"
	results, err := p.desktop.capture(text)
	if err != nil || len(results) != 3 {
		t.Fatalf("desktop capture = %v, %v", results, err)
	}
	var captured struct {
		Tasks []struct {
			Title string `json:"title"`
			State string `json:"state"`
		} `json:"tasks"`
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": text}, &captured); err != nil {
		t.Fatalf("mobile capture: %v", err)
	}
	if len(captured.Tasks) != 3 || captured.Tasks[0].Title != "Plan the offsite" || captured.Tasks[2].State != "backlog" {
		t.Fatalf("mobile captured %+v", captured.Tasks)
	}
	p.requireSameWorkspace()
}

func parityShorthandPreviewMatchesTheCapture(t *testing.T) {
	p := newPair(t)
	var preview struct {
		Lines []struct {
			Title       string   `json:"title"`
			ProjectName string   `json:"project_name"`
			Plan        string   `json:"plan"`
			State       string   `json:"state"`
			Priority    int      `json:"priority"`
			Tags        []string `json:"tags"`
		} `json:"lines"`
		Errors []struct {
			Line    int    `json:"line"`
			Column  int    `json:"column"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := p.mobile.call("query.capture_preview", map[string]any{"text": "Ship it !2 #ops $next\nFile taxes %atlas-launch"}, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 0 || len(preview.Lines) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	first, second := preview.Lines[0], preview.Lines[1]
	if first.Title != "Ship it" || first.Plan != "next" || first.State != "backlog" || first.Priority != 2 || len(first.Tags) != 1 {
		t.Fatalf("first preview line = %+v", first)
	}
	if second.ProjectName != "Atlas Launch" || second.State != "inbox" || second.Plan != "" {
		t.Fatalf("second preview line = %+v", second)
	}
	before := fingerprint(t, p.mobile.dataDir)
	if err := p.mobile.call("query.capture_preview", map[string]any{"text": "Fine\nBroken @notadate"}, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 1 || preview.Errors[0].Line != 2 || preview.Errors[0].Column != 7 || len(preview.Lines) != 0 {
		t.Fatalf("refused preview = %+v", preview)
	}
	if fingerprint(t, p.mobile.dataDir) != before {
		t.Fatal("a preview wrote to the workspace")
	}
}

func parityShorthandRefusalNamesTheLineAndCommitsNothing(t *testing.T) {
	p := newPair(t)
	text := "Fine line\nBad line $ready"
	desktopBefore, mobileBefore := fingerprint(t, p.desktop.dataDir), fingerprint(t, p.mobile.dataDir)
	_, desktopErr := p.desktop.capture(text)
	err := p.mobile.call("command.capture", map[string]any{"text": text}, nil)
	var refusal *wireError
	if desktopErr == nil || !errors.As(err, &refusal) || refusal.Code != "validation" {
		t.Fatalf("desktop %v, mobile %v: both must refuse", desktopErr, err)
	}
	lines, _ := refusal.Details["lines"].([]any)
	if len(lines) != 1 || lines[0].(map[string]any)["line"].(float64) != 2 ||
		!strings.Contains(desktopErr.Error(), lines[0].(map[string]any)["message"].(string)) {
		t.Fatalf("mobile line errors %v do not name the desktop's refusal %v", refusal.Details, desktopErr)
	}
	if fingerprint(t, p.desktop.dataDir) != desktopBefore || fingerprint(t, p.mobile.dataDir) != mobileBefore {
		t.Fatal("a refused capture wrote to a workspace")
	}
}

func parityStructuredCapture(t *testing.T) {
	p := newPair(t)
	results, err := p.desktop.capture("Call the plumber !2 #home ##phone @2026-10-21 *pin =review $backlog")
	if err != nil {
		t.Fatal(err)
	}
	created := app.TaskLocator{ProjectID: results[0].ProjectID, TaskID: results[0].TaskID}
	if err := p.desktop.describe(created, "Ask about the boiler."); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.runtime.Service().SetResumeNote(created, "left a voicemail"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{
		"title": "Call the plumber", "priority": 2, "tags": []string{"home"}, "context": "phone",
		"due": "2026-10-21", "pinned": true, "review": true, "destination": "backlog",
		"description": "Ask about the boiler.", "resume_note": "left a voicemail",
	}, nil); err != nil {
		t.Fatalf("mobile structured capture: %v", err)
	}
	p.requireSameWorkspace()
}

func parityDailyEdit(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	if err := p.desktop.quickEdit(target, "Call the bank about the mortgage !1 #money ##phone @2026-10-12 *pin =review"); err != nil {
		t.Fatalf("desktop quick edit: %v", err)
	}
	if err := p.desktop.describe(target, "Ask for the fixed rate."); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.runtime.Service().SetResumeNote(target, "have the account number ready"); err != nil {
		t.Fatal(err)
	}
	var edited struct {
		Changed bool `json:"changed"`
	}
	if err := p.mobile.call("command.edit", map[string]any{
		"task": wire(target), "title": "Call the bank about the mortgage", "priority": 1,
		"tags": []string{"money"}, "context": "phone", "due": "2026-10-12", "pinned": true, "review": true,
		"description": "Ask for the fixed rate.", "resume_note": "have the account number ready",
	}, &edited); err != nil || !edited.Changed {
		t.Fatalf("mobile edit = changed %v, %v", edited.Changed, err)
	}
	p.requireSameWorkspace()

	// Clearing goes through the same use case on both sides.
	if err := p.desktop.quickEdit(target, "Call the bank about the mortgage ##0 @0 -#money"); err != nil {
		t.Fatalf("desktop clearing edit: %v", err)
	}
	if err := p.mobile.call("command.edit", map[string]any{
		"task": wire(target), "context": "", "due": "", "tags": []string{},
	}, nil); err != nil {
		t.Fatalf("mobile clearing edit: %v", err)
	}
	p.requireSameWorkspace()

	// An edit naming only what is already stored commits nothing.
	before := fingerprint(t, p.mobile.dataDir)
	if err := p.mobile.call("command.edit", map[string]any{"task": wire(target), "priority": 1}, &edited); err != nil || edited.Changed {
		t.Fatalf("no-op edit = changed %v, %v", edited.Changed, err)
	}
	if fingerprint(t, p.mobile.dataDir) != before {
		t.Fatal("a no-op edit wrote to the workspace")
	}
}

func parityDeferralRoutesToBacklog(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	if err := p.desktop.quickEdit(target, "Call the bank @@2026-10-09"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.edit", map[string]any{"task": wire(target), "defer": "2026-10-09"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	if ref := p.desktop.lookup(target); ref.Task.State() != models.StateBacklog || ref.Task.DeferredUntil == nil {
		t.Fatalf("deferred task = %s until %v", ref.Task.State(), ref.Task.DeferredUntil)
	}
}

func parityTodayNextExclusivity(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	for _, action := range []app.PlanningAction{app.PlanningAddToday, app.PlanningAddNext, app.PlanningAddToday} {
		if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: action}); err != nil {
			t.Fatalf("desktop %s: %v", action, err)
		}
		if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": string(action)}, nil); err != nil {
			t.Fatalf("mobile %s: %v", action, err)
		}
		p.requireSameWorkspace()
		today := planning(t, p.desktop.runtime, models.QueryModeToday)
		next := planning(t, p.desktop.runtime, models.QueryModeNext)
		inToday, inNext := contains(today, "Call the bank"), contains(next, "Call the bank")
		if inToday == inNext || inToday != (action == app.PlanningAddToday) {
			t.Fatalf("after %s: today %v next %v", action, today, next)
		}
	}
	// Today is a membership, not a state: Doing keeps its state there, while
	// Next sends a task back to Backlog.
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.StateDoing}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": "doing"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: app.PlanningRemoveToday}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": "remove_today"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
}

func parityCompletionClearsPlanning(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: app.PlanningAddToday}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": "add_today"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.StateDone}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": "done"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	if today := planning(t, p.desktop.runtime, models.QueryModeToday); contains(today, "Call the bank") {
		t.Fatalf("a completed task stayed in Today: %v", today)
	}
	ref := p.desktop.lookup(target)
	if ref.Task.State() != models.StateDone || ref.Task.CompletedAt == nil {
		t.Fatalf("completed task = %s at %v", ref.Task.State(), ref.Task.CompletedAt)
	}
	// Reopening goes back through the one lifecycle primitive on both sides.
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.ReopenTargetState}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": string(models.ReopenTargetState)}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
}

func parityLifecycleRefusals(t *testing.T) {
	p := newPair(t)
	results, err := p.desktop.capture("Move house")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.capture("Pack the books >Move"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": "Move house"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": "Pack the books >Move"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	parent := app.TaskLocator{ProjectID: results[0].ProjectID, TaskID: results[0].TaskID}
	bank := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	// A parent cannot finish over open work.
	_, desktopErr := service.Transition(app.StateRequest{Locator: parent, Target: models.StateDone})
	refusal := p.refusal("command.transition", map[string]any{"task": wire(parent), "state": "done"})
	var blocked *app.ChildrenBlockedError
	if !errors.As(desktopErr, &blocked) || refusal.Details["kind"] != "incomplete_children" ||
		!strings.Contains(refusal.Message, blocked.Error()) {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}
	children, _ := refusal.Details["children"].([]any)
	if len(children) != 1 || children[0].(map[string]any)["title"] != "Pack the books" {
		t.Fatalf("mobile names blocking children %v", refusal.Details["children"])
	}
	if p.desktop.lookup(parent).Task.IsTerminalState() {
		t.Fatal("a refused completion finished the parent")
	}
	p.requireSameWorkspace()

	// A finished task must be reopened before it moves anywhere else.
	if _, err := service.Transition(app.StateRequest{Locator: bank, Target: models.StateCanceled}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(bank), "state": "canceled"}, nil); err != nil {
		t.Fatal(err)
	}
	desktopBefore, mobileBefore := fingerprint(t, p.desktop.dataDir), fingerprint(t, p.mobile.dataDir)
	_, desktopErr = service.Transition(app.StateRequest{Locator: bank, Target: models.StateDoing})
	refusal = p.refusal("command.transition", map[string]any{"task": wire(bank), "state": "doing"})
	var transition *app.TransitionError
	if !errors.As(desktopErr, &transition) || refusal.Details["kind"] != "invalid_transition" ||
		refusal.Details["from"] != "canceled" || refusal.Details["to"] != "doing" || refusal.Details["reason"] != transition.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}

	// A finished task joins no plan.
	_, desktopErr = service.ApplyPlanning(app.PlanningRequest{Locator: bank, Action: app.PlanningAddNext})
	refusal = p.refusal("command.plan", map[string]any{"task": wire(bank), "action": "add_next"})
	var invalid *app.InvalidInputError
	if !errors.As(desktopErr, &invalid) || refusal.Details["kind"] != "invalid_input" ||
		refusal.Details["field"] != invalid.Field || refusal.Details["reason"] != invalid.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}

	// A field the shared gate refuses is refused on both sides.
	tooLong := strings.Repeat("x", 241)
	_, desktopErr = service.SetResumeNote(parent, tooLong)
	refusal = p.refusal("command.edit", map[string]any{"task": wire(parent), "resume_note": tooLong})
	if !errors.As(desktopErr, &invalid) || refusal.Details["field"] != "resume note" || refusal.Details["reason"] != invalid.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}
	if fingerprint(t, p.desktop.dataDir) != desktopBefore || fingerprint(t, p.mobile.dataDir) != mobileBefore {
		t.Fatal("a refusal wrote to a workspace")
	}
	p.requireSameWorkspace()
}
