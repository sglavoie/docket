package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// OperationExecutor resolves descriptors, targets, values, risk policy,
// transactions, status, and undo/redo before invoking one handler.
type OperationExecutor struct {
	registry           *operation.OperationRegistry
	resolver           operation.OperationTargetResolver
	bindings           map[operation.OperationID][]OperationBinding
	valueProviders     map[operation.ValueProviderID]OperationValueProvider
	transactionFactory OperationTransactionFactory
	reporter           OperationStatusReporter
	history            *OperationHistory
}

func NewOperationExecutor(registry *operation.OperationRegistry) *OperationExecutor {
	return &OperationExecutor{
		registry:       registry,
		bindings:       make(map[operation.OperationID][]OperationBinding),
		valueProviders: make(map[operation.ValueProviderID]OperationValueProvider),
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
	id operation.ValueProviderID,
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
	if err := operation.ValidateOperationCapabilities(binding.OperationID, binding.RequiredCapabilities); err != nil {
		return err
	}
	if descriptor.TargetCardinality == operation.TargetCardinalityNone &&
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
	binding.Views = append([]presentation.PresentationID(nil), binding.Views...)
	binding.RequiredCapabilities = operation.CloneCapabilitySet(binding.RequiredCapabilities)
	executor.bindings[binding.OperationID] = append(executor.bindings[binding.OperationID], binding)
	return nil
}

// Execute runs one operation through the centralized policy pipeline.
func (executor *OperationExecutor) Execute(request OperationExecutionRequest) OperationExecutionResult {
	descriptor, ok := executor.registry.Descriptor(request.ID)
	if !ok {
		return executor.failure(fmt.Errorf("%w: %s", ErrOperationNotRegistered, request.ID))
	}
	// Refused ahead of availability so any route into an operation the checklist
	// cursor turned off reports the one sentence rather than the generic
	// unavailable error.
	if err := operation.ChecklistCursorRejection(descriptor, request.Context); err != nil {
		return executor.failure(err)
	}
	binding, ok := executor.bindingFor(request.ID, request.Context)
	if !ok {
		return executor.failure(
			fmt.Errorf("%w: %s", ErrOperationUnavailable, request.ID),
		)
	}
	if !descriptor.Applicable(request.Context) {
		return executor.failure(
			fmt.Errorf("%w: %s", ErrOperationUnavailable, request.ID),
		)
	}
	// Refused before resolution so delegated bindings, which swallow resolver
	// errors, cannot reach a handler with an archived task.
	if err := operation.ArchivedTaskTargetRejection(descriptor, request.Context); err != nil {
		return executor.failure(err)
	}

	targets, err := executor.resolver.Resolve(descriptor, request.Context)
	if err != nil && !binding.DelegateTargetResolution {
		return executor.failure(err)
	}
	if err != nil {
		targets = operation.ResolvedOperationTargets{Source: descriptor.TargetSource}
	}
	invocation := OperationInvocation{
		Descriptor: descriptor,
		Context:    request.Context,
		Targets:    targets,
		Value:      request.Value,
		HasValue:   request.HasValue,
	}
	executor.report(OperationStatusEvent{})

	if descriptor.ValueProviderID != "" && !invocation.HasValue && !binding.DelegateValueProvider {
		provider := executor.valueProviders[descriptor.ValueProviderID]
		if provider == nil {
			return executor.failure(
				fmt.Errorf("operation %s has no registered value provider %s", request.ID, descriptor.ValueProviderID),
			)
		}
		provided := provider(invocation)
		if provided.Err != nil {
			return executor.failure(provided.Err)
		}
		if provided.HasValue {
			invocation.Value = provided.Value
			invocation.HasValue = true
		} else {
			valueRequest := &OperationValueRequestMsg{
				Request:  request,
				Provider: descriptor.ValueProviderID,
			}
			executor.report(OperationStatusEvent{})
			return OperationExecutionResult{
				Message:       provided.Pending,
				ValueRequired: valueRequest,
			}
		}
	}

	if descriptor.Risk == operation.OperationRiskDestructive &&
		!request.Confirmed &&
		!binding.DelegateConfirmation {
		confirmation := &OperationConfirmationRequestMsg{
			Request: request,
			Label:   descriptor.Label,
			Targets: targets,
		}
		confirmation.Request.Confirmed = true
		executor.report(OperationStatusEvent{})
		return OperationExecutionResult{ConfirmationRequired: confirmation}
	}

	var transaction OperationTransaction
	if descriptor.Risk != operation.OperationRiskReadOnly &&
		!binding.DelegateTransaction &&
		executor.transactionFactory != nil {
		transaction, err = executor.transactionFactory(invocation)
		if err != nil {
			return executor.failure(err)
		}
	}

	handlerResult := executeOperationBinding(binding, invocation)
	if handlerResult.Err != nil {
		if transaction != nil {
			_ = transaction.Rollback()
		}
		return executor.failure(handlerResult.Err)
	}
	if transaction != nil {
		if err := transaction.Commit(); err != nil {
			_ = transaction.Rollback()
			return executor.failure(err)
		}
	}

	// The ledger and the reporter always need a name for what ran; only the
	// handler's own words are feedback worth showing a user.
	recorded := handlerResult.Status
	if recorded == "" {
		recorded = descriptor.Label
	}
	if descriptor.Risk != operation.OperationRiskReadOnly {
		executor.history.record(operationUndoEntry{
			status: recorded,
			undo:   handlerResult.Undo,
			redo:   handlerResult.Redo,
		})
	}
	return executor.deliver(
		OperationStatusEvent{},
		handlerResult.Message,
		handlerResult.Status,
	)
}

// Undo runs the newest executor-native undo callback.
func (executor *OperationExecutor) Undo() OperationExecutionResult {
	entry, ok := executor.history.popUndo()
	if !ok {
		return executor.failure(errors.New("nothing to undo"))
	}
	message := entry.undo()
	return executor.deliver(
		OperationStatusEvent{},
		message,
		entry.status,
	)
}

// Redo runs the newest executor-native redo callback.
func (executor *OperationExecutor) Redo() OperationExecutionResult {
	entry, ok := executor.history.popRedo()
	if !ok {
		return executor.failure(errors.New("nothing to redo"))
	}
	message := entry.redo()
	return executor.deliver(
		OperationStatusEvent{},
		message,
		entry.status,
	)
}

func (executor *OperationExecutor) bindingFor(
	id operation.OperationID,
	ctx operation.CommandContext,
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
		return binding.BulkHandler(invocation, append([]operation.OperationTarget(nil), invocation.Targets.Targets...))
	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

func (executor *OperationExecutor) failure(err error) OperationExecutionResult {
	executor.report(OperationStatusEvent{})
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
