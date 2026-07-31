package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

var (
	ErrOperationNotRegistered = errors.New("operation is not registered")
	ErrOperationUnavailable   = errors.New("operation is not available in the active context")
	ErrOperationHandler       = errors.New("operation has no compatible handler")
)

// OperationInvocationSource identifies the adapter that requested execution.
// Handlers do not branch on palette-specific message types.
type OperationInvocationSource string

const (
	OperationSourceCommand     OperationInvocationSource = "command"
	OperationSourceDirectKey   OperationInvocationSource = "direct-key"
	OperationSourceActionSheet OperationInvocationSource = "action-sheet"
	OperationSourceBulk        OperationInvocationSource = "bulk"
)

// OperationExecutionRequest is the complete input to centralized execution.
type OperationExecutionRequest struct {
	ID        OperationID
	Context   CommandContext
	Source    OperationInvocationSource
	Value     any
	HasValue  bool
	Confirmed bool
}

// OperationInvocation is the validated handler input.
type OperationInvocation struct {
	Descriptor OperationDescriptor
	Context    CommandContext
	Targets    ResolvedOperationTargets
	Source     OperationInvocationSource
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
	Views       []PresentationID

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

// OperationStatusPhase drives status/toast adapters without coupling handlers
// to any particular renderer.
type OperationStatusPhase string

const (
	OperationStatusStarted              OperationStatusPhase = "started"
	OperationStatusConfirmationRequired OperationStatusPhase = "confirmation-required"
	OperationStatusValueRequired        OperationStatusPhase = "value-required"
	OperationStatusCompleted            OperationStatusPhase = "completed"
	OperationStatusFailed               OperationStatusPhase = "failed"
	OperationStatusUndone               OperationStatusPhase = "undone"
	OperationStatusRedone               OperationStatusPhase = "redone"
)

type OperationStatusEvent struct {
	OperationID OperationID
	Label       string
	Phase       OperationStatusPhase
	Status      string
	Err         error
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
		return ErrorMsg{Err: result.Err}
	case result.ConfirmationRequired != nil:
		return *result.ConfirmationRequired
	case result.Message != nil:
		return result.Message
	case result.ValueRequired != nil:
		return *result.ValueRequired
	case result.Status != "" && !result.StatusReported:
		return StatusMsg(result.Status)
	default:
		return nil
	}
}

type operationUndoEntry struct {
	id     OperationID
	label  string
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

// OperationExecutor resolves descriptors, targets, values, risk policy,
// transactions, status, and undo/redo before invoking one handler.
type OperationExecutor struct {
	registry           *OperationRegistry
	resolver           OperationTargetResolver
	bindings           map[OperationID][]OperationBinding
	valueProviders     map[ValueProviderID]OperationValueProvider
	transactionFactory OperationTransactionFactory
	reporter           OperationStatusReporter
	history            *OperationHistory
}

func (m *AppModel) registerOperation(
	descriptor OperationDescriptor,
	binding OperationBinding,
) error {
	if m == nil || m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationRegistry.Register(descriptor); err != nil {
		return err
	}
	binding.OperationID = descriptor.ID
	return m.operationExecutor.Register(binding)
}

func NewOperationExecutor(registry *OperationRegistry) *OperationExecutor {
	return &OperationExecutor{
		registry:       registry,
		bindings:       make(map[OperationID][]OperationBinding),
		valueProviders: make(map[ValueProviderID]OperationValueProvider),
		history:        &OperationHistory{},
	}
}

// SetTransactionFactory installs the mutation transaction boundary.
func (executor *OperationExecutor) SetTransactionFactory(factory OperationTransactionFactory) {
	executor.transactionFactory = factory
}

// SetStatusReporter installs the renderer/status adapter.
func (executor *OperationExecutor) SetStatusReporter(reporter OperationStatusReporter) {
	executor.reporter = reporter
}

// History returns the executor-native undo/redo history.
func (executor *OperationExecutor) History() *OperationHistory {
	return executor.history
}

// RegisterValueProvider registers one shared value source.
func (executor *OperationExecutor) RegisterValueProvider(
	id ValueProviderID,
	provider OperationValueProvider,
) error {
	if id == "" || provider == nil {
		return fmt.Errorf("value provider ID and handler are required")
	}
	if _, exists := executor.valueProviders[id]; exists {
		return fmt.Errorf("value provider already registered: %s", id)
	}
	executor.valueProviders[id] = provider
	return nil
}

// Register binds executable behavior to an existing descriptor.
func (executor *OperationExecutor) Register(binding OperationBinding) error {
	if executor == nil || executor.registry == nil {
		return fmt.Errorf("operation executor requires a registry")
	}
	if binding.OperationID == "" {
		return fmt.Errorf("operation binding ID is required")
	}
	descriptor, ok := executor.registry.Descriptor(binding.OperationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrOperationNotRegistered, binding.OperationID)
	}
	if err := validateOperationCapabilities(binding.OperationID, binding.RequiredCapabilities); err != nil {
		return err
	}
	if descriptor.TargetCardinality == TargetCardinalityNone &&
		(binding.SingleHandler != nil || binding.BulkHandler != nil) {
		return fmt.Errorf(
			"operation %s resolves no targets and requires a target-free handler",
			binding.OperationID,
		)
	}
	handlerCount := 0
	for _, present := range []bool{
		binding.Handler != nil,
		binding.SingleHandler != nil,
		binding.BulkHandler != nil,
	} {
		if present {
			handlerCount++
		}
	}
	if handlerCount != 1 {
		return fmt.Errorf("operation %s requires exactly one handler kind", binding.OperationID)
	}
	binding.Views = append([]PresentationID(nil), binding.Views...)
	binding.RequiredCapabilities = cloneCapabilitySet(binding.RequiredCapabilities)
	executor.bindings[binding.OperationID] = append(executor.bindings[binding.OperationID], binding)
	return nil
}

// Execute runs one operation through the centralized policy pipeline.
func (executor *OperationExecutor) Execute(request OperationExecutionRequest) OperationExecutionResult {
	descriptor, ok := executor.registry.Descriptor(request.ID)
	if !ok {
		return executor.failure(request.ID, "", fmt.Errorf("%w: %s", ErrOperationNotRegistered, request.ID))
	}
	binding, ok := executor.bindingFor(request.ID, request.Context)
	if !ok {
		return executor.failure(
			request.ID,
			descriptor.Label,
			fmt.Errorf("%w: %s", ErrOperationUnavailable, request.ID),
		)
	}
	if !descriptor.Applicable(request.Context) {
		return executor.failure(
			request.ID,
			descriptor.Label,
			fmt.Errorf("%w: %s", ErrOperationUnavailable, request.ID),
		)
	}

	targets, err := executor.resolver.Resolve(descriptor, request.Context)
	if err != nil && !binding.DelegateTargetResolution {
		return executor.failure(request.ID, descriptor.Label, err)
	}
	if err != nil {
		targets = ResolvedOperationTargets{Source: descriptor.TargetSource}
	}
	invocation := OperationInvocation{
		Descriptor: descriptor,
		Context:    request.Context,
		Targets:    targets,
		Source:     request.Source,
		Value:      request.Value,
		HasValue:   request.HasValue,
	}
	executor.report(OperationStatusEvent{
		OperationID: request.ID,
		Label:       descriptor.Label,
		Phase:       OperationStatusStarted,
	})

	if descriptor.ValueProviderID != "" && !invocation.HasValue && !binding.DelegateValueProvider {
		provider := executor.valueProviders[descriptor.ValueProviderID]
		if provider == nil {
			return executor.failure(
				request.ID,
				descriptor.Label,
				fmt.Errorf("operation %s has no registered value provider %s", request.ID, descriptor.ValueProviderID),
			)
		}
		provided := provider(invocation)
		if provided.Err != nil {
			return executor.failure(request.ID, descriptor.Label, provided.Err)
		}
		if provided.HasValue {
			invocation.Value = provided.Value
			invocation.HasValue = true
		} else {
			valueRequest := &OperationValueRequestMsg{
				Request:  request,
				Provider: descriptor.ValueProviderID,
			}
			executor.report(OperationStatusEvent{
				OperationID: request.ID,
				Label:       descriptor.Label,
				Phase:       OperationStatusValueRequired,
			})
			return OperationExecutionResult{
				Message:       provided.Pending,
				ValueRequired: valueRequest,
			}
		}
	}

	if descriptor.Risk == OperationRiskDestructive &&
		!request.Confirmed &&
		!binding.DelegateConfirmation {
		confirmation := &OperationConfirmationRequestMsg{
			Request: request,
			Label:   descriptor.Label,
			Targets: targets,
		}
		confirmation.Request.Confirmed = true
		executor.report(OperationStatusEvent{
			OperationID: request.ID,
			Label:       descriptor.Label,
			Phase:       OperationStatusConfirmationRequired,
		})
		return OperationExecutionResult{ConfirmationRequired: confirmation}
	}

	var transaction OperationTransaction
	if descriptor.Risk != OperationRiskReadOnly &&
		!binding.DelegateTransaction &&
		executor.transactionFactory != nil {
		transaction, err = executor.transactionFactory(invocation)
		if err != nil {
			return executor.failure(request.ID, descriptor.Label, err)
		}
	}

	handlerResult := executeOperationBinding(binding, invocation)
	if handlerResult.Err != nil {
		if transaction != nil {
			_ = transaction.Rollback()
		}
		return executor.failure(request.ID, descriptor.Label, handlerResult.Err)
	}
	if transaction != nil {
		if err := transaction.Commit(); err != nil {
			_ = transaction.Rollback()
			return executor.failure(request.ID, descriptor.Label, err)
		}
	}

	// The ledger and the reporter always need a name for what ran; only the
	// handler's own words are feedback worth showing a user.
	recorded := handlerResult.Status
	if recorded == "" {
		recorded = descriptor.Label
	}
	if descriptor.Risk != OperationRiskReadOnly {
		executor.history.record(operationUndoEntry{
			id:     descriptor.ID,
			label:  descriptor.Label,
			status: recorded,
			undo:   handlerResult.Undo,
			redo:   handlerResult.Redo,
		})
	}
	return executor.deliver(
		OperationStatusEvent{
			OperationID: descriptor.ID,
			Label:       descriptor.Label,
			Phase:       OperationStatusCompleted,
			Status:      recorded,
		},
		handlerResult.Message,
		handlerResult.Status,
	)
}

// Undo runs the newest executor-native undo callback.
func (executor *OperationExecutor) Undo() OperationExecutionResult {
	entry, ok := executor.history.popUndo()
	if !ok {
		return executor.failure("", "Undo", errors.New("nothing to undo"))
	}
	message := entry.undo()
	return executor.deliver(
		OperationStatusEvent{
			OperationID: entry.id,
			Label:       entry.label,
			Phase:       OperationStatusUndone,
			Status:      entry.status,
		},
		message,
		entry.status,
	)
}

// Redo runs the newest executor-native redo callback.
func (executor *OperationExecutor) Redo() OperationExecutionResult {
	entry, ok := executor.history.popRedo()
	if !ok {
		return executor.failure("", "Redo", errors.New("nothing to redo"))
	}
	message := entry.redo()
	return executor.deliver(
		OperationStatusEvent{
			OperationID: entry.id,
			Label:       entry.label,
			Phase:       OperationStatusRedone,
			Status:      entry.status,
		},
		message,
		entry.status,
	)
}

func (executor *OperationExecutor) bindingFor(
	id OperationID,
	ctx CommandContext,
) (OperationBinding, bool) {
	var global *OperationBinding
	var scoped *OperationBinding
	for i := range executor.bindings[id] {
		binding := &executor.bindings[id][i]
		if !ctx.Capabilities.HasAll(binding.RequiredCapabilities) {
			continue
		}
		if binding.Applicability != nil && !binding.Applicability(ctx) {
			continue
		}
		if len(binding.Views) == 0 {
			if global == nil {
				global = binding
			}
			continue
		}
		for _, view := range binding.Views {
			if view == ctx.View {
				scoped = binding
				break
			}
		}
	}
	if scoped != nil {
		return *scoped, true
	}
	if global != nil {
		return *global, true
	}
	return OperationBinding{}, false
}

func executeOperationBinding(
	binding OperationBinding,
	invocation OperationInvocation,
) OperationHandlerResult {
	switch {
	case binding.Handler != nil:
		return binding.Handler(invocation)
	case binding.SingleHandler != nil && len(invocation.Targets.Targets) == 1:
		return binding.SingleHandler(invocation, invocation.Targets.Targets[0])
	case binding.BulkHandler != nil && len(invocation.Targets.Targets) > 0:
		return binding.BulkHandler(invocation, append([]OperationTarget(nil), invocation.Targets.Targets...))
	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

func (executor *OperationExecutor) failure(
	id OperationID,
	label string,
	err error,
) OperationExecutionResult {
	executor.report(OperationStatusEvent{
		OperationID: id,
		Label:       label,
		Phase:       OperationStatusFailed,
		Err:         err,
	})
	return OperationExecutionResult{Err: err}
}

// deliver names the one user-visible owner of a finished outcome. A handler
// message outranks the reporter, so the reporter learns it must stay quiet and
// the result stops claiming its status was already shown.
func (executor *OperationExecutor) deliver(
	event OperationStatusEvent,
	message tea.Msg,
	status string,
) OperationExecutionResult {
	event.OwnsUserFeedback = message == nil && status != ""
	executor.report(event)
	return OperationExecutionResult{
		Message:        message,
		Status:         status,
		StatusReported: executor.reporter != nil && event.OwnsUserFeedback,
	}
}

func (executor *OperationExecutor) report(event OperationStatusEvent) {
	if executor != nil && executor.reporter != nil {
		executor.reporter(event)
	}
}
