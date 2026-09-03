package tui

import (
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func WorkspaceTagEntity(name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindTag, Scope: operation.EntityScopeGlobal, Name: name})
}

func ProjectTagEntity(projectID, name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{
		Kind: operation.EntityKindTag, Scope: operation.EntityScopeProject, ProjectID: projectID, Name: name,
	})
}

func ScopeEntity(name string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindScope, Name: name})
}

func ModeEntity(id chrome.OutlinerModeID) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindMode, Name: string(id)})
}

func ToolEntity(id string) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindTool, Name: id})
}

func OperationEntity(id operation.OperationID) operation.EntityIdentity {
	return operation.NormalizeEntityIdentity(operation.EntityIdentity{Kind: operation.EntityKindOperation, Name: string(id)})
}
