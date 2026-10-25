package storage

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/transaction"
)

// CaptureSubmission identifies one host submission independently of a socket
// connection. Digest binds it to the exact draft, not a reparsed relative date.
type CaptureSubmission struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (c CaptureSubmission) Validate() error {
	id, err := uuid.Parse(c.ID)
	if err != nil || id.String() != c.ID {
		return fmt.Errorf("capture submission needs a canonical UUID")
	}
	digest, err := hex.DecodeString(c.Digest)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != c.Digest {
		return fmt.Errorf("capture submission needs a SHA-256 draft digest")
	}
	return nil
}

// CaptureReceipt is committed with the tasks, but is neither undoable nor
// exported to exchange sync. A retry must not resurrect an undone/deleted task.
type CaptureReceipt struct {
	CaptureSubmission
	Version       int      `json:"version"`
	TransactionID string   `json:"transaction_id"`
	Titles        []string `json:"titles"`
}

// CaptureAlreadyCommitted carries the original outcome when another request
// committed this submission while the caller was preparing its snapshot.
type CaptureAlreadyCommitted struct{ Receipt CaptureReceipt }

func (e *CaptureAlreadyCommitted) Error() string { return "capture was already committed" }

func captureReceiptPath(transactionsDir, id string) string {
	return filepath.Join(filepath.Dir(transactionsDir), "control-captures", id+".json")
}

// readCaptureReceipt reads only immutable committed evidence. Missing evidence
// is not proof that a request failed: CommitMulti rechecks under its workspace
// lock after recovery before admitting any retry.
func (s *CommitService) readCaptureReceipt(submission CaptureSubmission) (*CaptureReceipt, error) {
	if err := submission.Validate(); err != nil {
		return nil, err
	}
	path := captureReceiptPath(s.transactionsDir, submission.ID)
	if !pathWithinApprovedRoots(path, s.roots) {
		return nil, fmt.Errorf("capture receipt: %w", transaction.ErrTargetOutsideRoot)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading capture receipt: %w", err)
	}
	var receipt CaptureReceipt
	if err = json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("reading capture receipt: %w", err)
	}
	if receipt.Version != 1 || receipt.ID != submission.ID || receipt.TransactionID == "" || len(receipt.Titles) == 0 {
		return nil, fmt.Errorf("capture receipt is invalid")
	}
	if receipt.Digest != submission.Digest {
		return nil, fmt.Errorf("capture identity was already used for a different draft")
	}
	return &receipt, nil
}

func (s *CommitService) captureReceiptOp(receipt *CaptureReceipt, txnID string) (transaction.Op, error) {
	stored := *receipt
	stored.Version = 1
	stored.TransactionID = txnID
	data, err := json.Marshal(stored)
	if err != nil {
		return transaction.Op{}, err
	}
	return transaction.Op{Kind: transaction.OpWrite, Target: captureReceiptPath(s.transactionsDir, stored.ID), Data: append(data, '\n'), Mode: FilePermissions}, nil
}

// LookupCaptureReceipt waits for any live commit and recovers interrupted work
// before deciding whether a submission landed.
func (s *CommitService) LookupCaptureReceipt(submission CaptureSubmission) (*CaptureReceipt, error) {
	if err := submission.Validate(); err != nil {
		return nil, err
	}
	var receipt *CaptureReceipt
	err := s.withWorkspaceCommitLock(func() error {
		if _, err := s.recoverLocked(s.journal()); err != nil {
			return err
		}
		var err error
		receipt, err = s.readCaptureReceipt(submission)
		return err
	})
	return receipt, err
}

func validateCaptureReceipts(changesets []ChangeSet) error {
	for i, cs := range changesets {
		if cs.Capture == nil {
			continue
		}
		if i != 0 {
			return fmt.Errorf("storage: capture receipt must be on the first changeset")
		}
		if err := cs.Capture.Validate(); err != nil {
			return err
		}
		if len(cs.Capture.Titles) == 0 {
			return fmt.Errorf("storage: capture receipt has no tasks")
		}
	}
	return nil
}
