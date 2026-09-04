package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// registerOperation is the shell's convenience wrapper over the two registries:
// OperationRegistry.Register declares the descriptor and
// OperationExecutor.Register binds executable behavior to it. It is the only
// AppModel coupling the operation core has, which is why it sits here rather
// than in operation_executor.go beside the type-level pipeline.
func (m *AppModel) registerOperation(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
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
