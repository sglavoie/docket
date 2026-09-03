package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestValidateOperationOnePathRejectsBothPathsAndUnreachableDirectOnly(t *testing.T) {
	const surfaceOperation = operation.OperationID("surface.example")

	t.Run("duplicate placement", func(t *testing.T) {
		bound := operation.InputStrategy{
			ID:       operation.InputStrategyProjectHub,
			Bindings: []operation.OperationKeyBinding{{OperationID: surfaceOperation, Keys: []string{"Z"}}},
		}
		err := operation.ValidateOperationOnePath([]operation.OperationManifestEntry{{ID: surfaceOperation}}, bound)
		if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
			t.Fatalf("error = %v, want duplicate-placement rejection", err)
		}
	})

	t.Run("unreachable direct-only operation", func(t *testing.T) {
		unbound := operation.InputStrategy{ID: operation.InputStrategyProjectHub}
		menu := []operation.OperationManifestEntry{{ID: surfaceOperation, DirectOnly: true}}
		err := operation.ValidateOperationOnePath(menu, unbound)
		if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
			t.Fatalf("error = %v, want unreachable-direct rejection", err)
		}
	})
}
