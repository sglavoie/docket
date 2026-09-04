package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// registerOperation is the shell's convenience wrapper over the two registries:
// OperationRegistry.Register declares the descriptor and
// OperationExecutor.Register binds executable behavior to it. It is the only
// AppModel coupling the operation core has, which is why it sits here rather
// than in operation_executor.go beside the type-level pipeline. The pairing
// itself belongs to the operation controller; this reaches it from the fifteen
// registrars, which are AppModel methods because their handlers are.
func (m *AppModel) registerOperation(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
) error {
	if m == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	return m.operations.register(descriptor, binding)
}
