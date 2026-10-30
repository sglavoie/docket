package coordinator

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// promotionStub is a SubviewMutator double that records the promotion request and
// answers with a canned result, so the coordinator's own translation — request
// assembly in, reveal identity out — is tested without the storage stack. Every
// other method is unreachable here and says so rather than returning a zero value
// a test could mistake for a real answer.
type promotionStub struct {
	req    app.ChecklistPromotionRequest
	calls  int
	result app.ChecklistPromotionResult
	err    error
}

func (s *promotionStub) PromoteChecklistItem(
	req app.ChecklistPromotionRequest,
) (app.ChecklistPromotionResult, error) {
	s.calls++
	s.req = req
	return s.result, s.err
}

func (s *promotionStub) MakeChecklistItem(app.ChecklistDemotionRequest) (app.ChecklistDemotionResult, error) {
	panic("unexpected MakeChecklistItem")
}

func (s *promotionStub) AddComment(app.TaskLocator, string) (app.CommentResult, error) {
	panic("unexpected AddComment")
}

func (s *promotionStub) EditComment(app.TaskLocator, int, models.Comment, string) (app.CommentResult, error) {
	panic("unexpected EditComment")
}

func (s *promotionStub) RemoveComment(app.TaskLocator, int, models.Comment) (app.CommentResult, error) {
	panic("unexpected RemoveComment")
}

func (s *promotionStub) AddChecklistItem(app.TaskLocator, string) (app.ChecklistResult, error) {
	panic("unexpected AddChecklistItem")
}

func (s *promotionStub) EditChecklistItem(app.TaskLocator, int, string, []models.ChecklistItem) (app.ChecklistResult, error) {
	panic("unexpected EditChecklistItem")
}

func (s *promotionStub) RemoveChecklistItem(app.TaskLocator, int, []models.ChecklistItem) (app.ChecklistResult, error) {
	panic("unexpected RemoveChecklistItem")
}

func (s *promotionStub) ToggleChecklistItem(app.TaskLocator, int, []models.ChecklistItem) (app.ChecklistResult, error) {
	panic("unexpected ToggleChecklistItem")
}

func (s *promotionStub) ReorderChecklistItem(app.TaskLocator, int, int, []models.ChecklistItem) (app.ChecklistResult, error) {
	panic("unexpected ReorderChecklistItem")
}

func (s *promotionStub) MoveChecklistItem(app.TaskLocator, int, int, []models.ChecklistItem) (app.ChecklistResult, error) {
	panic("unexpected MoveChecklistItem")
}

// TestSubviewCoordinatorPromoteChecklistItemCarriesTheCommittedChild proves the
// coordinator forwards the frozen checklist verbatim and hands the shell the one
// thing no view can otherwise learn: the identity of the child the transaction
// minted, on the owner's own reload result.
func TestSubviewCoordinatorPromoteChecklistItemCarriesTheCommittedChild(t *testing.T) {
	stub := &promotionStub{result: app.ChecklistPromotionResult{
		Owner: app.TaskResult{TaskID: "owner", ShortID: 1, ProjectID: "p1", TransactionID: "txn-1"},
		Child: app.TaskResult{TaskID: "child", ShortID: 7, ProjectID: "p1", TransactionID: "txn-1"},
	}}
	checklist := []models.ChecklistItem{{Text: "first"}, {Text: "second", Done: true}}
	identity := taskref.TaskIdentity{ProjectID: "p1", TaskID: "owner"}

	msg := SubviewCoordinator{sub: stub}.PromoteChecklistItem(identity, 1, checklist)()

	if stub.calls != 1 {
		t.Fatalf("mutator calls = %d, want one", stub.calls)
	}
	if stub.req.Owner != identity.Locator() || stub.req.Index != 1 {
		t.Fatalf("request = %+v, want the owner locator and index 1", stub.req)
	}
	if len(stub.req.Checklist) != len(checklist) || stub.req.Checklist[1] != checklist[1] {
		t.Fatalf("request checklist = %+v, want the frozen list verbatim", stub.req.Checklist)
	}
	result, ok := msg.(TransactionResultMsg)
	if !ok {
		t.Fatalf("msg = %T, want TransactionResultMsg", msg)
	}
	if result.Action != "Make Subtask" {
		t.Fatalf("action = %q, want the user-facing label", result.Action)
	}
	if result.Result.TaskID != "owner" {
		t.Fatalf("reload result = %+v, want the owner", result.Result)
	}
	want := taskref.TaskIdentity{ProjectID: "p1", TaskID: "child"}
	if result.RevealTaskFor != want {
		t.Fatalf("RevealTaskFor = %+v, want the committed child %+v", result.RevealTaskFor, want)
	}
}

// TestSubviewCoordinatorPromoteChecklistItemRefusalCarriesNoIdentity holds the
// other half: a refused promotion is an error message and never names a child,
// so a view cannot reveal a task that was never committed.
func TestSubviewCoordinatorPromoteChecklistItemRefusalCarriesNoIdentity(t *testing.T) {
	stub := &promotionStub{err: errors.New("the checklist changed")}

	msg := SubviewCoordinator{sub: stub}.PromoteChecklistItem(
		taskref.TaskIdentity{ProjectID: "p1", TaskID: "owner"}, 0,
		[]models.ChecklistItem{{Text: "first"}},
	)()

	failure, ok := msg.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("msg = %T, want TransactionErrorMsg", msg)
	}
	if failure.Err == nil || failure.Err.Error() != "the checklist changed" {
		t.Fatalf("err = %v, want the mutator's refusal", failure.Err)
	}
}

type clockedMutator struct{ now time.Time }

func (c clockedMutator) Now() time.Time { return c.now }

// TestMutatorNowReadsTheServiceClock stamps projects a capture creates from the
// mutator's own clock when it keeps one, and falls back to the wall clock for a
// stand-in that does not.
func TestMutatorNowReadsTheServiceClock(t *testing.T) {
	fixed := time.Date(2031, time.February, 3, 4, 5, 6, 0, time.UTC)
	if got := mutatorNow(clockedMutator{now: fixed}); !got.Equal(fixed) {
		t.Fatalf("mutatorNow(clocked) = %s, want %s", got, fixed)
	}
	before := time.Now()
	if got := mutatorNow(struct{}{}); got.Before(before) {
		t.Fatalf("mutatorNow(unclocked) = %s, want the wall clock", got)
	}
}
