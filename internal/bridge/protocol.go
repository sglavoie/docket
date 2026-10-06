// Package bridge is the versioned request/response protocol the mobile app
// speaks to the engine: one JSON request in, one JSON response out, decoded
// strictly and answered through the shared headless runtime. The gomobile
// binding in mobile/ttbridge is a two-function shell over Call; nothing here
// depends on gomobile, so the protocol is tested as ordinary Go.
//
// docs/mobile/bridge-contract.md is the contract this package implements.
package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Version is the highest bridge_version this engine speaks.
const Version = 1

// Error codes, exactly as the contract spells them.
const (
	CodeInvalidRequest     = "invalid_request"
	CodeUnsupportedVersion = "unsupported_version"
	CodeUnknownOp          = "unknown_op"
	CodeWorkspaceMismatch  = "workspace_mismatch"
	CodeNotOpen            = "not_open"
	CodeNotFound           = "not_found"
	CodeValidation         = "validation"
	CodeRefused            = "refused"
	CodeStaleRevision      = "stale_revision"
	CodeConflict           = "conflict"
	CodeLocked             = "locked"
	CodeMigrationRequired  = "migration_required"
	CodeSchemaTooNew       = "schema_too_new"
	CodeIO                 = "io"
	CodeInternal           = "internal"
)

// retryableCodes is the contract's Retryable column.
var retryableCodes = map[string]bool{
	CodeStaleRevision: true,
	CodeLocked:        true,
	CodeIO:            true,
}

// Request is one decoded bridge request. Payload stays raw until the op that
// owns it decodes it, as strictly as the envelope.
type Request struct {
	BridgeVersion int             `json:"bridge_version"`
	RequestID     string          `json:"request_id"`
	Op            string          `json:"op"`
	Workspace     string          `json:"workspace,omitempty"`
	Expect        *Expect         `json:"expect,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// Expect is a mutation's revision preconditions: canonical project revisions by
// project id and raw domain artifact revisions by domain name.
type Expect struct {
	Projects map[string]string `json:"projects,omitempty"`
	Domains  map[string]string `json:"domains,omitempty"`
}

// Response is one bridge response. Exactly one of Result and Error is set.
type Response struct {
	BridgeVersion int    `json:"bridge_version"`
	RequestID     string `json:"request_id"`
	OK            bool   `json:"ok"`
	Result        any    `json:"result,omitempty"`
	Error         *Error `json:"error,omitempty"`
}

// Error is a structured failure. It is also a Go error, so an op returns one
// directly when it already knows the code.
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func newError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Retryable: retryableCodes[code]}
}

func (e *Error) with(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// decodeStrict decodes exactly one JSON value into dst, refusing unknown fields
// at every level and trailing data.
func decodeStrict(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}

// decodePayload decodes an op's payload strictly; an absent payload decodes as
// the zero value, which the op's own validation then judges.
func decodePayload(raw json.RawMessage, dst any) *Error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := decodeStrict(raw, dst); err != nil {
		return newError(CodeInvalidRequest, "payload: %v", err)
	}
	return nil
}
