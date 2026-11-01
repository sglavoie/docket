package operationhost

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// OpenSearchRecovery uses the shared action sheet and executor. These view
// operations never resolve task targets, even when the result list is empty.
func (c *Controller) OpenSearchRecovery(shell Shell, ctx operation.CommandContext) error {
	if shell.ActiveOutliner() == nil || !ctx.Search.Ranked || ctx.Search.Query == "" {
		return fmt.Errorf("enter a ranked task search with / first")
	}
	ids := []operation.OperationID{
		operation.OperationCoreSearchFinished,
		operation.OperationCoreSearchDeferred,
		operation.OperationCoreQueryReadout,
	}
	sheet, err := listsurface.NewContextActionSheetModel(c.Registry, c.Executor, ctx, ids, listsurface.ContextActionSheetConfig{
		Host: listsurface.ContextActionHostOutliner, Title: "Search Recovery",
		ItemIDPrefix: "search-recovery:",
		SecondaryFor: func(descriptor operation.OperationDescriptor) string {
			switch descriptor.ID {
			case operation.OperationCoreSearchFinished:
				return fmt.Sprintf("%d · ?completed", ctx.Search.Completed)
			case operation.OperationCoreSearchDeferred:
				return fmt.Sprintf("%d · ?deferred", ctx.Search.Deferred)
			default:
				return "scope · filters"
			}
		},
	})
	if err != nil {
		return err
	}
	c.ActionSheet = sheet
	return nil
}
