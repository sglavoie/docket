package operation

import (
	"strings"
	"testing"
)

func TestValidateOperationOnePathRejectsBothPathsAndUnreachableDirectOnly(t *testing.T) {
	const surfaceOperation = OperationID("surface.example")

	t.Run("duplicate placement", func(t *testing.T) {
		bound := InputStrategy{
			ID:       InputStrategyProjectHub,
			Bindings: []OperationKeyBinding{{OperationID: surfaceOperation, Keys: []string{"Z"}}},
		}
		err := ValidateOperationOnePath([]OperationManifestEntry{{ID: surfaceOperation}}, bound)
		if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
			t.Fatalf("error = %v, want duplicate-placement rejection", err)
		}
	})

	t.Run("unreachable direct-only operation", func(t *testing.T) {
		unbound := InputStrategy{ID: InputStrategyProjectHub}
		menu := []OperationManifestEntry{{ID: surfaceOperation, DirectOnly: true}}
		err := ValidateOperationOnePath(menu, unbound)
		if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
			t.Fatalf("error = %v, want unreachable-direct rejection", err)
		}
	})
}
