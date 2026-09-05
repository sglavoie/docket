package operation

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The four seams every host of this pipeline needs and none of them owns. Each
// is a pure reading of a descriptor, an invocation, or a result, so it belongs
// beside the types it reads rather than being copied into each cluster that
// registers an operation.

// ResultCmd converts a completed execution outcome into the Bubble Tea command
// seam.
func ResultCmd(result OperationExecutionResult) tea.Cmd {
	switch typed := result.AsMessage().(type) {
	case nil:
		return nil
	case OperationCommandResultMsg:
		return typed.Cmd
	default:
		return func() tea.Msg { return typed }
	}
}

// StringValue reads a trimmed string invocation value, which is how hosts hand
// a prompted name to a rename operation.
func StringValue(invocation OperationInvocation) string {
	if !invocation.HasValue {
		return ""
	}
	value, ok := invocation.Value.(string)
	if !ok {
		return ""
	}
	return value
}

// PendingTextValueProvider registers a text provider with the executor without
// supplying a value, so the shell opens the prompt for it.
func PendingTextValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}

// PendingPickedValueProvider registers a picked provider with the executor
// without supplying a value, so the shell constructs the provider's ListSurface
// picker from the frozen request instead. It is the picker counterpart of
// PendingTextValueProvider.
func PendingPickedValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}

// ActionSecondaryText names what a sheet row will ask for once chosen: the
// value it prompts for, the confirmation it raises, or the key that reaches it
// directly.
func ActionSecondaryText(descriptor OperationDescriptor) string {
	switch {
	case descriptor.ValueProviderID != "":
		return "choose " + strings.ReplaceAll(string(descriptor.ValueProviderID), "-", " ")
	case descriptor.Risk == OperationRiskDestructive:
		return "confirmation required"
	case descriptor.DirectKey != "":
		return descriptor.DirectKey
	default:
		return ""
	}
}

// CommandMessage wraps one command so it can travel back through the executor
// as a result rather than being run at the handler. A nil command is no
// message, which is what lets a handler return one unconditionally.
func CommandMessage(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return OperationCommandResultMsg{Cmd: cmd}
}
