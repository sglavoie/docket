package lens

import "github.com/sglavoie/docket/internal/tui/operation"

// The named-lens operation IDs. They live beside the picker rather than
// beside the descriptors and handlers the shell registers, for the same reason
// the notes surface owns its own: the picker names four of them on its rows and
// dispatches two more from the action rows, so the vocabulary has to sit under
// both readers. The descriptor catalog, the value providers, and every handler
// stay in the shell, because each of those needs the live AppModel.
const (
	OperationLensApply       operation.OperationID = "tui.lens.apply"
	OperationLensCreate      operation.OperationID = "tui.lens.create"
	OperationLensAddTemplate operation.OperationID = "tui.lens.add-template"
	OperationLensRename      operation.OperationID = "tui.lens.rename"
	OperationLensDuplicate   operation.OperationID = "tui.lens.duplicate"
	OperationLensReplace     operation.OperationID = "tui.lens.replace"
	OperationLensDelete      operation.OperationID = "tui.lens.delete"
)
