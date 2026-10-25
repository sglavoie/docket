package bridge

import (
	"errors"

	"github.com/sglavoie/docket/internal/engine"
)

// conflictListPayload optionally names the devices the transport read from
// the exchange's advertisements, so versions are attributed by name.
type conflictListPayload struct {
	DeviceNames map[string]string `json:"device_names"`
}

type conflictShowPayload struct {
	ConflictID  string            `json:"conflict_id"`
	DeviceNames map[string]string `json:"device_names"`
}

type conflictListResult struct {
	Conflicts []engine.ConflictView `json:"conflicts"`
}

type conflictShowResult struct {
	Conflict engine.ConflictView `json:"conflict"`
}

// conflictList reads every open sync conflict with its base, its competing
// versions, who wrote each, and what choosing one would also settle.
func (b *Bridge) conflictList(request Request) (any, error) {
	var payload conflictListPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if _, err := b.syncCommits(); err != nil {
		return nil, err
	}
	views, err := b.workspace.runtime.SyncConflicts(names(payload.DeviceNames))
	if err != nil {
		return nil, err
	}
	return conflictListResult{Conflicts: views}, nil
}

// conflictShow reads one open conflict. A conflict that closed, or whose
// bytes an edit changed since it was listed, is not_found: its id named what
// it held.
func (b *Bridge) conflictShow(request Request) (any, error) {
	var payload conflictShowPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.ConflictID == "" {
		return nil, newError(CodeInvalidRequest, "conflict_id is required")
	}
	if _, err := b.syncCommits(); err != nil {
		return nil, err
	}
	views, err := b.workspace.runtime.SyncConflicts(names(payload.DeviceNames))
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		if view.ID == payload.ConflictID {
			return conflictShowResult{Conflict: view}, nil
		}
	}
	return nil, newError(CodeNotFound, "conflict %s is no longer open", payload.ConflictID).with("conflict_id", payload.ConflictID)
}

// conflictResolve commits a resolution through the engine the desktop's
// `tt sync resolve` uses. A conflict that changed since it was read refuses
// stale_revision, writes nothing, and reports the conflicts open now.
func (b *Bridge) conflictResolve(request Request) (any, error) {
	var payload engine.ResolveRequest
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.ConflictID == "" {
		return nil, newError(CodeInvalidRequest, "conflict_id is required")
	}
	if _, err := b.syncCommits(); err != nil {
		return nil, err
	}
	result, err := b.workspace.runtime.ResolveSyncConflict(payload)
	if err != nil {
		var refusal *Error
		ok := errors.As(classify(err), &refusal)
		if ok && refusal.Code == CodeStaleRevision {
			if open, listErr := b.workspace.runtime.SyncConflicts(nil); listErr == nil {
				ids := make([]string, 0, len(open))
				for _, view := range open {
					ids = append(ids, view.ID)
				}
				refusal = refusal.with("conflicts", ids)
			}
		}
		if ok {
			return nil, refusal
		}
		return nil, err
	}
	return result, nil
}

// syncDiagnostics gathers the engine's half of a diagnostics export: status,
// outbox, and conflict structure without content. The transport adds its own
// report before a person shares the file.
func (b *Bridge) syncDiagnostics(request Request) (any, error) {
	var payload conflictListPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if _, err := b.syncCommits(); err != nil {
		return nil, err
	}
	return b.workspace.runtime.SyncDiagnostics(names(payload.DeviceNames))
}

func names(given map[string]string) map[string]string {
	if given == nil {
		return map[string]string{}
	}
	return given
}
