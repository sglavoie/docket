package valuepicker

import (
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// The picker keeps its state private and answers these three questions instead,
// because the shell that opens one only ever needs to know which provider it is
// showing, what it is titled, and which list surface it is driving.

// Provider names the value provider this picker was opened for.
func (model *OperationValuePickerModel) Provider() operation.ValueProviderID {
	if model == nil {
		return ""
	}
	return model.provider
}

// ExecutionRequest is the operation this picker is supplying a value for. It is
// what lets the shell rebuild an equivalent picker over refreshed data — the
// rows a vocabulary rename rewrote — without re-entering the executor and
// asking for the value step a second time.
func (model *OperationValuePickerModel) ExecutionRequest() operation.OperationExecutionRequest {
	if model == nil {
		return operation.OperationExecutionRequest{}
	}
	return model.request
}

// Title is the picker's header line.
func (model *OperationValuePickerModel) Title() string {
	if model == nil {
		return ""
	}
	return model.title
}

// Surface is the list the picker draws and moves its cursor over.
func (model *OperationValuePickerModel) Surface() *listsurface.ListSurfaceModel {
	if model == nil {
		return nil
	}
	return model.surface
}

// Subtitle is the line under the header, which says what accepting a row does.
func (model *OperationValuePickerModel) Subtitle() string {
	if model == nil {
		return ""
	}
	return model.subtitle
}

// Multi reports whether this picker accepts more than one row.
func (model *OperationValuePickerModel) Multi() bool {
	return model != nil && model.multi
}

// Searching reports whether the list surface is taking a query.
func (model *OperationValuePickerModel) Searching() bool {
	return model != nil && model.searching
}

// SetSearching puts the picker in or out of its query-taking state.
func (model *OperationValuePickerModel) SetSearching(searching bool) {
	if model == nil {
		return
	}
	model.searching = searching
}

// TagAdder is the inline add mode the tag and context rows type into.
func (model *OperationValuePickerModel) TagAdder() *listsurface.TagAdder {
	if model == nil {
		return nil
	}
	return &model.tagAdder
}

// InitialMembership is what the targets agreed on for a row when the picker
// opened, which is the baseline a commit diffs against.
func (model *OperationValuePickerModel) InitialMembership(id listsurface.ListItemID) OperationValueMembership {
	if model == nil {
		return OperationValueNone
	}
	return model.membershipInitial[id]
}

// CurrentMembership is what the picker says about a row now.
func (model *OperationValuePickerModel) CurrentMembership(id listsurface.ListItemID) OperationValueMembership {
	if model == nil {
		return OperationValueNone
	}
	return model.membershipCurrent[id]
}
