// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

// guideOrder is teach order, which is a different concern from the parse order
// `registry` declares. Parse order answers "which pattern peels first", and one
// field can only be disambiguated by sitting early; teach order answers "what
// does a user meet first", where the same field belongs beside its neighbors.
// The two held one list until the attachment sigil needed to be parsed ahead of
// #tag without leading the syntax guide. validateGuideOrder keeps them the same
// set, so a field added to one and forgotten in the other fails at init.
// Teach order pairs every negation immediately after the value it subtracts,
// which is the opposite of the adjacency parse order needs: a reader meets the
// thing before the removal of it, while the tokenizer must peel the longer
// trigger first. The two lists disagreeing here is the split working, not a
// drift — validateGuideOrder holds them to one set and nothing more.
var guideOrder = []Field{
	FieldPriority,
	FieldRemovePriority,
	FieldTag,
	FieldRemoveTag,
	FieldContext,
	FieldRemoveContext,
	FieldDue,
	FieldRemoveDue,
	FieldDefer,
	FieldRemoveDefer,
	FieldDestination,
	FieldRemoveDestination,
	FieldPinned,
	FieldRemovePinned,
	FieldReview,
	FieldRemoveReview,
	FieldProject,
	FieldRemoveProject,
	FieldDerived,
	FieldRemoveDerived,
	FieldParent,
	FieldChild,
	FieldDependency,
	FieldRemoveDependency,
	FieldAttachment,
	FieldRemoveAttachment,
	FieldComment,
	FieldTaskType,
}

// Definitions returns defensive copies of the canonical definitions available
// on a surface, in parse order.
//
// A field the parser recognizes on a surface but declares no form for there is
// deliberately absent: recognition without a form is how a token is refused by
// name instead of folding into the title, and advertising a formless entry would
// teach a syntax the surface does not accept. `-~task` is the one such entry
// today — capture tokenizes it only to say removal is not a capture operation.
