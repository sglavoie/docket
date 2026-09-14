package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// workspaceTagEntity returns the identity of one tag aggregated across the
// workspace.
func workspaceTagEntity(name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindTag, Scope: operation.EntityScopeGlobal, Name: name})
}

// projectTagEntity returns the identity of one tag inside one project.
//
//nolint:unparam // The project scope is half of what this helper spells; every current caller happening to name one fixture project is a property of the fixtures, not of the identity.
func projectTagEntity(projectID, name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{
		Kind: operation.EntityKindTag, Scope: operation.EntityScopeProject, ProjectID: projectID, Name: name,
	})
}

// scopeEntity returns the identity of one context scope, reusing the stable
// scope catalog ID the Scope Hub already lists rows under.
func scopeEntity(name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindScope, Name: name})
}

// modeEntity returns the identity of one Outliner catalog mode.
func modeEntity(id chrome.OutlinerModeID) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindMode, Name: string(id)})
}

// toolEntity returns the identity of one promoted or secondary tool.
func toolEntity(id string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindTool, Name: id})
}

// operationEntity returns the identity of one registered operation, letting an
// operation target another operation.
func operationEntity(id operation.OperationID) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindOperation, Name: string(id)})
}

func taskOperationTestContext(task *models.Task, project *models.Project) operation.CommandContext {
	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Entity = outliner.TaskOperationEntity(task, project)
	return context
}
