package bridge

import (
	"errors"

	"github.com/sglavoie/docket/internal/storage"
)

// A receipt names the exact transaction it can undo. Selection and restore
// share the ledger's lock, so a later action cannot be undone in its place.
func (b *Bridge) commandUndo(request Request) (any, error) {
	return b.restoreHistory(request, false)
}

func (b *Bridge) commandRedo(request Request) (any, error) {
	return b.restoreHistory(request, true)
}

func (b *Bridge) restoreHistory(request Request, redo bool) (any, error) {
	var payload struct {
		TransactionID string `json:"transaction_id"`
	}
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.TransactionID == "" {
		return nil, newError(CodeInvalidRequest, "transaction_id is required")
	}
	restore := b.workspace.runtime.Ledger().UndoTransactionWithEntry
	verb := "undone"
	if redo {
		restore = b.workspace.runtime.Ledger().RedoTransactionWithEntry
		verb = "redone"
	}
	entry, action, err := restore(storage.GlobalScope(), payload.TransactionID)
	if errors.Is(err, storage.ErrUndoSuperseded) || errors.Is(err, storage.ErrNoUndoHistory) || errors.Is(err, storage.ErrNoRedoHistory) {
		return nil, newError(CodeRefused, "This action can no longer be %s here because the workspace history has changed. Refresh Change History and try again.", verb)
	}
	if err != nil {
		return nil, err
	}
	// Do not turn a committed restore into a reported failure with a fallible
	// reread. The next query refreshes the runtime from the restored workspace.
	return struct {
		TransactionID string `json:"transaction_id"`
		Action        string `json:"action"`
	}{entry.TransactionID, action}, nil
}
