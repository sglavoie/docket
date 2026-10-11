package bridge

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
)

type reorderPayload struct {
	Task  locator   `json:"task"`
	Kind  string    `json:"kind"`
	Order []locator `json:"order"`
}

// commandReorder commits a complete drag in one transaction. Planning moves
// permute only the named members' slots, preserving hidden and parked tasks.
func (b *Bridge) commandReorder(request Request) (any, error) {
	var payload reorderPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	order, err := targetsOf("order", payload.Order)
	if err != nil {
		return nil, err
	}
	seen := make(map[app.TaskLocator]bool, len(order))
	for _, target := range order {
		if seen[target] {
			return nil, newError(CodeValidation, "a task may appear only once in the order")
		}
		seen[target] = true
	}
	if len(order) < 2 || !seen[payload.Task.app()] {
		return nil, newError(CodeValidation, "order needs at least two tasks, including the moved task")
	}
	if err := b.workspace.runtime.Refresh(); err != nil {
		return nil, err
	}
	return b.commitMany(func(service *app.Service) (manyEffect, error) {
		effect := manyEffect{targets: order}
		switch payload.Kind {
		case "siblings":
			result, err := service.ReorderSiblingOrder(payload.Task.app(), order)
			effect.transactionID = result.TransactionID
			return effect, err
		case "today", "next":
			planning := b.workspace.runtime.State().Planning()
			members := planning.GetToday()
			if payload.Kind == "next" {
				members = planning.GetNext()
			}
			merged, err := mergePlanningOrder(members, order)
			if err != nil {
				return effect, err
			}
			horizon := app.PlanningHorizonToday
			if payload.Kind == "next" {
				horizon = app.PlanningHorizonNext
			}
			effect.transactionID, err = service.ReorderPlanning(horizon, merged)
			return effect, err
		default:
			return effect, newError(CodeInvalidRequest, "kind must be today, next, or siblings")
		}
	})
}

func mergePlanningOrder(members []config.ProjectTaskLocator, order []app.TaskLocator) ([]app.TaskLocator, error) {
	named := make(map[app.TaskLocator]bool, len(order))
	for _, target := range order {
		named[target] = true
	}
	merged := make([]app.TaskLocator, len(members))
	next := 0
	for index, member := range members {
		target := app.TaskLocator{ProjectID: member.ProjectID, TaskID: member.TaskID}
		if named[target] {
			merged[index] = order[next]
			next++
		} else {
			merged[index] = target
		}
	}
	if next != len(order) {
		return nil, newError(CodeValidation, "the planning list changed; reload it before reordering")
	}
	return merged, nil
}
