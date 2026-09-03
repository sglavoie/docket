package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

var (
	ErrOperationNotRegistered = errors.New("operation is not registered")
	ErrOperationUnavailable   = errors.New("operation is not available in the active context")
	ErrOperationHandler       = errors.New("operation has no compatible handler")
)

// OperationExecutionRequest is the complete input to centralized execution.
type OperationExecutionRequest struct {
	ID        OperationID
	Context   CommandContext
	Value     any
	HasValue  bool
	Confirmed bool
}

// OperationInvocation is the validated handler input.
type OperationInvocation struct {
	Descriptor OperationDescriptor
	Context    CommandContext
	Targets    ResolvedOperationTargets
	Value      any
	HasValue   bool
}

// OperationHandlerResult is the handler's palette-independent result.
// Undo/Redo are recorded only after a successful mutation transaction commits.
type OperationHandlerResult struct {
	Message tea.Msg
	Status  string
	Err     error
	Undo    func() tea.Msg
	Redo    func() tea.Msg
}

type OperationHandler func(OperationInvocation) OperationHandlerResult
type SingleOperationHandler func(OperationInvocation, OperationTarget) OperationHandlerResult
type BulkOperationHandler func(OperationInvocation, []OperationTarget) OperationHandlerResult

// OperationBinding binds one descriptor to context-specific execution. Legacy
// command adapters delegate policies that their existing message flow already
// owns; new bindings use centralized target/value/confirmation/transaction
// policy by default.
type OperationBinding struct {
	OperationID OperationID
	Views       []presentation.PresentationID

	RequiredCapabilities CapabilitySet
	Applicability        OperationApplicability

	Handler       OperationHandler
	SingleHandler SingleOperationHandler
	BulkHandler   BulkOperationHandler

	DelegateTargetResolution bool
	DelegateValueProvider    bool
	DelegateConfirmation     bool
	DelegateTransaction      bool
}

// OperationValueProviderResult either supplies a value, returns a generic
// picker/prompt message, or reports an error.
type OperationValueProviderResult struct {
	Value    any
	HasValue bool
	Pending  tea.Msg
	Err      error
}

type OperationValueProvider func(OperationInvocation) OperationValueProviderResult

// OperationTransaction is the central commit/rollback boundary for a mutation.
type OperationTransaction interface {
	Commit() error
	Rollback() error
}

type OperationTransactionFactory func(OperationInvocation) (OperationTransaction, error)

type OperationStatusEvent struct {
	// OwnsUserFeedback reports that this event's Status is the outcome's only
	// user-visible feedback. A reporter that shows anything else duplicates a
	// status the message seam already delivers.
	OwnsUserFeedback bool
}

type OperationStatusReporter func(OperationStatusEvent)

// OperationConfirmationRequestMsg is renderer-neutral. Action sheets, direct
// keys and retained list surfaces can present it with their own chrome and
// resubmit Request after setting Confirmed.
type OperationConfirmationRequestMsg struct {
	Request OperationExecutionRequest
	Label   string
	Targets ResolvedOperationTargets
}

// OperationValueRequestMsg is returned when a registered provider needs UI
// input but did not supply its own generic prompt message.
type OperationValueRequestMsg struct {
	Request  OperationExecutionRequest
	Provider ValueProviderID
}

// OperationExecutionResult is the adapter-facing outcome. Status holds the
// handler's own words, and StatusReported records that an installed reporter
// already showed them.
type OperationExecutionResult struct {
	Message              tea.Msg
	Status               string
	StatusReported       bool
	Err                  error
	ConfirmationRequired *OperationConfirmationRequestMsg
	ValueRequired        *OperationValueRequestMsg
}

// OperationCommandResultMsg carries a Bubble Tea command through the
// renderer-neutral operation pipeline.
type OperationCommandResultMsg struct {
	Cmd tea.Cmd
}

func (msg OperationCommandResultMsg) String() string { return "operation command result" }

func (msg OperationCommandResultMsg) Type() string { return "operation_command_result" }

// AsMessage converts an execution outcome to the existing Bubble Tea message
// seam while preserving centralized errors and generic pending requests. A
// handler whose whole outcome is a status reaches the user here, because the
// status reporter is optional and a message always outranks it.
func (result OperationExecutionResult) AsMessage() tea.Msg {
	switch {
	case result.Err != nil:
		return status.ErrorMsg{Err: result.Err}
	case result.ConfirmationRequired != nil:
		return *result.ConfirmationRequired
	case result.Message != nil:
		return result.Message
	case result.ValueRequired != nil:
		return *result.ValueRequired
	case result.Status != "" && !result.StatusReported:
		return status.StatusMsg(result.Status)
	default:
		return nil
	}
}

type operationUndoEntry struct {
	status string
	undo   func() tea.Msg
	redo   func() tea.Msg
}

// OperationHistory owns executor-native undo/redo callbacks. Existing durable
// workspace-ledger commands remain delegated compatibility bindings.
type OperationHistory struct {
	undo []operationUndoEntry
	redo []operationUndoEntry
}

func (history *OperationHistory) record(entry operationUndoEntry) {
	if history == nil || entry.undo == nil || entry.redo == nil {
		return
	}
	history.undo = append(history.undo, entry)
	history.redo = nil
}

func (history *OperationHistory) popUndo() (operationUndoEntry, bool) {
	if history == nil || len(history.undo) == 0 {
		return operationUndoEntry{}, false
	}
	index := len(history.undo) - 1
	entry := history.undo[index]
	history.undo = history.undo[:index]
	history.redo = append(history.redo, entry)
	return entry, true
}

func (history *OperationHistory) popRedo() (operationUndoEntry, bool) {
	if history == nil || len(history.redo) == 0 {
		return operationUndoEntry{}, false
	}
	index := len(history.redo) - 1
	entry := history.redo[index]
	history.redo = history.redo[:index]
	history.undo = append(history.undo, entry)
	return entry, true
}
