package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

type checkInOfferController struct {
	seen        map[string]bool
	pending     []app.TaskResult
	openContent bool
}

// checkInOfferObservedMessage is the durable commit evidence an offer follows.
// Interaction ownership tags exactly these results so the queue can tell an
// authorized continuation of the current interaction from a delayed one, and
// nothing about that tagging is the queue's: see interactionOwnership in
// app_update.go, which the explicit editor's own stale-refusal protection reads
// with no queue involved.
func checkInOfferObservedMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case coordinator.TransactionResultMsg, coordinator.BatchTransactionResultMsg,
		coordinator.TaskCreatedMsg, coordinator.TasksCapturedMsg, coordinator.TaskLineEditedMsg,
		operation.OperationCommandResultMsg, operation.OperationValueSelectedMsg,
		operation.OperationValueRequestMsg:
		return true
	default:
		return false
	}
}

func (m *AppModel) observeCheckInOfferMessage(msg tea.Msg) {
	switch msg := msg.(type) {
	case coordinator.TransactionResultMsg:
		m.queueCheckInOffer([]app.TaskResult{msg.Result})
	case coordinator.BatchTransactionResultMsg:
		m.queueCheckInOffer(msg.Tasks)
	case coordinator.TaskCreatedMsg:
		results := msg.Results
		if len(results) == 0 && msg.Result != nil {
			results = []app.TaskResult{*msg.Result}
		}
		m.queueCheckInOffer(results)
		m.checkInOffers.openContent = len(m.checkInOffers.pending) > 0 && msg.OpenContent
	case coordinator.TasksCapturedMsg:
		m.queueCheckInOffer(msg.Results)
	case coordinator.TaskLineEditedMsg:
		m.queueCheckInOffer([]app.TaskResult{{ProjectID: msg.Result.Task.ProjectID, TaskID: msg.Result.Task.TaskID,
			TransactionID: msg.Result.TransactionID, StateEffect: msg.Result.StateEffect}})
	}
}

func (m *AppModel) queueCheckInOffer(results []app.TaskResult) {
	if m.checkInOffers.seen == nil {
		m.checkInOffers.seen = make(map[string]bool)
	}
	var eligible []app.TaskResult
	transactions := make(map[string]bool)
	identities := make(map[taskref.TaskIdentity]bool)
	for _, result := range results {
		if result.TransactionID == "" || m.checkInOffers.seen[result.TransactionID] {
			continue
		}
		transactions[result.TransactionID] = true
		identity := taskref.TaskIdentity{ProjectID: result.ProjectID, TaskID: result.TaskID}
		if result.StateEffect.OffersCheckIn() && identity.Valid() && !identities[identity] {
			identities[identity] = true
			eligible = append(eligible, result)
		}
	}
	for id := range transactions {
		m.checkInOffers.seen[id] = true
	}
	if len(eligible) > 0 {
		m.checkInOffers.pending = eligible
	}
}

func (m *AppModel) drainCheckInOffer() tea.Cmd {
	if len(m.checkInOffers.pending) == 0 || m.checkInOffers.openContent || m.checkInOfferCovered() {
		return nil
	}
	if m.crossOutliner != nil && m.crossOutliner.QuickEdit.IsActive() {
		return nil
	}
	results := m.checkInOffers.pending
	m.checkInOffers.pending = nil
	context := m.operationCommandContext()
	context.CheckInOffer = true
	context.Selection = operation.SelectionContext{Visual: true}
	context.Entity = operation.EntityContext{}
	context.CurrentEntity = operation.EntityIdentity{}
	// Re-resolve only this commit's subset. Refresh never widens it, even if
	// another task entered active work or the cursor relocated in the meantime.
	reloaded := make(map[string]bool)
	for _, result := range results {
		identity := taskref.TaskIdentity{ProjectID: result.ProjectID, TaskID: result.TaskID}
		loaded, checked := reloaded[identity.ProjectID]
		if !checked {
			loaded = m.reloadProjectByID(identity.ProjectID)
			reloaded[identity.ProjectID] = loaded
		}
		if !loaded {
			continue
		}
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil || !task.CheckIn().IsEmpty() ||
			(task.State() != models.StateDoing && task.State() != models.StateWaiting) {
			continue
		}
		context.Selection.Tasks = append(context.Selection.Tasks, identity)
		context.Selection.Entities = append(context.Selection.Entities, outliner.TaskOperationEntity(task, project))
	}
	if len(context.Selection.Tasks) == 0 {
		return nil
	}
	context.Selection.VisibleCount = len(context.Selection.Tasks)
	request := operation.OperationExecutionRequest{ID: operation.OperationTaskSetCheckIn, Context: context}
	return m.operations.OpenValueStep(m, operation.OperationValueRequestMsg{Request: request, Provider: operation.ValueProviderCheckIn})
}

// Partial capture retains its composer under the optional editor. Every other
// nested overlay keeps ownership until it closes; input can cancel the pending offer.
func (m *AppModel) checkInOfferCovered() bool {
	if m.operations.OverlayMounted() || m.lenses.Mounted() {
		return true
	}
	for _, overlay := range m.viewNav.navigation.Overlays.Overlays() {
		if overlay.Kind() != route.OverlayComposer {
			return true
		}
	}
	return false
}

// forgetInteractionScopedWork drops the offer the retired interaction queued.
// The queue is work one interaction owns, so it does not outlive it; this is the
// whole of what retiring an interaction costs the offer, and Session 10 retires
// the queue without touching the ownership count that outlives it.
func (m *AppModel) forgetInteractionScopedWork() {
	m.checkInOffers.pending = nil
	m.checkInOffers.openContent = false
}
