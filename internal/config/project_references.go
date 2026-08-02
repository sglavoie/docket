package config

type ProjectReferences struct {
	WasActionInbox bool                 `json:"was_action_inbox,omitempty"`
	Today          []ProjectTaskLocator `json:"today,omitempty"`
	NextOrder      []ProjectTaskLocator `json:"next_order,omitempty"`
}
