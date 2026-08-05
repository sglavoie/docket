package tui

func GlobalNoteEntity(filename string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindNote, Scope: EntityScopeGlobal, Name: filename})
}

func ProjectNoteEntity(projectID, filename string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind: EntityKindNote, Scope: EntityScopeProject, ProjectID: projectID, Name: filename,
	})
}

func WorkspaceTagEntity(name string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindTag, Scope: EntityScopeGlobal, Name: name})
}

func ProjectTagEntity(projectID, name string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind: EntityKindTag, Scope: EntityScopeProject, ProjectID: projectID, Name: name,
	})
}

func ScopeEntity(name string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindScope, Name: name})
}

func ModeEntity(id OutlinerModeID) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindMode, Name: string(id)})
}

func ToolEntity(id string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindTool, Name: id})
}

func OperationEntity(id OperationID) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindOperation, Name: string(id)})
}
