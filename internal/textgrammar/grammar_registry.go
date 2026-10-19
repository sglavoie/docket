// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"github.com/sglavoie/tt/internal/util"
)

var registry = []definitionSpec{
	// Comment is parsed FIRST, ahead of every other field including attachment,
	// and that position is the whole reason a comment can hold sigil characters.
	// RE2 has no lookaround, so no other pattern can say "not inside a ;…; span";
	// peeling the comment before anything else is what keeps
	// "Fix login ;check the #auth flow;" from losing #auth to the tag rule. The
	// closing delimiter is required, so an unterminated ";note" stays prose, and
	// the leading-whitespace boundary keeps "Fix a; then b" literal. The teaching
	// surfaces read guideOrder instead, so this position never leads the guide.
	{
		Definition: Definition{
			Field: FieldComment, Group: "Comments", Trigger: ";",
			Description: "Add a comment to the new task (repeatable)",
			Examples:    []string{";ask about the SSO deadline;", ";blocked on design;"},
			Surfaces:    SurfaceCapture, Cardinality: CardinalityRepeatable,
		},
		forms:      []surfaceForm{{Text: ";comment;", Surfaces: SurfaceCapture}},
		pattern:    `(?:^|\s);(\S[^;]*);`,
		validation: validateAny,
	},
	// Attachment is parsed BEFORE tag, and that position is load-bearing rather
	// than cosmetic. RE2 has no lookaround, so the tag pattern cannot say "not
	// followed by &": #([\w][\w.:-]*) stops at the &, and the rewrite that removes
	// the tag leaves the & whitespace-preceded, which would turn
	// "Task #work&http://x/y" into a spurious attachment. Peeling & first leaves
	// that & glued to the tag, where it stays literal title text. The teaching
	// surfaces read guideOrder instead, so this position never leads the guide.
	//
	// A real path holds spaces — "~/Library/Application Support/…" is where a
	// screenshot tool writes — and a run of non-space characters stops at the
	// first one, which attached half a path and left the rest in the title. The
	// value is therefore a single-quoted span, then a double-quoted one, then a
	// run whose spaces are backslash-escaped, tried in that order. Those
	// alternatives only widen what one token may span; what a token means is
	// settled by util.NormalizePastedPathInput, the same normalizer the
	// Attachments panel runs on a paste, so a quoted path and a typed one
	// classify identically. An unterminated quote matches no quoted span, falls
	// through to the last alternative, and stays the prefix it already was.
	//
	// Search reads & as presence alone — &any and &none — and its negation sits
	// immediately ahead so the two-rune trigger is peeled whole, which also puts
	// it ahead of #tag for the reason the positive is.
	{
		Definition: Definition{
			Field: FieldRemoveAttachment, Group: "Attachments", Trigger: "-&",
			Examples: []string{"-&any"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-&any", Surfaces: SurfaceSearch}},
		pattern:           `(?:^|\s)-&(\S+)`,
		validation:        validateAttachment,
		negates:           FieldAttachment,
		searchDescription: "Exclude tasks with (-&any) or without (-&none) attachments",
	},
	{
		Definition: Definition{
			Field: FieldAttachment, Group: "Attachments", Trigger: "&",
			// The description stays the guide's longest and cannot grow: it sets the
			// aligned description column every entry is padded to, and one word more
			// drops the whole syntax guide into its stacked fallback. The quoted
			// example is what teaches the spaces rule.
			Description: "Attach a URL, file path, or code reference (repeatable)",
			Examples: []string{
				"&https://example.com/spec.pdf",
				"&internal/app/add.go:19",
				"&'~/notes/Q3 plan.pdf'",
			},
			Surfaces: SurfaceCapture | SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		// &link leads so the search refusal, which names forms[0], still names
		// the capture spelling a locator belongs to.
		forms: []surfaceForm{
			{Text: "&link", Surfaces: SurfaceCapture},
			{Text: "&any", Surfaces: SurfaceSearch},
			{Text: "&none", Surfaces: SurfaceSearch},
		},
		pattern:           `(?:^|\s)&('[^']*'|"(?:\\.|[^"\\])*"|(?:\\ |\S)+)`,
		normalize:         util.NormalizePastedPathInput,
		validation:        validateAttachment,
		searchDescription: "Match tasks with (&any) or without (&none) attachments",
	},
	// Both dependency sigils are parsed BEFORE #tag, for the reason attachment is:
	// RE2 has no lookaround, so #([\w][\w.:-]*) stops at the tilde in
	// "Task #work~foo" and the rewrite that removes the tag would leave the ~
	// whitespace-preceded, minting a dependency out of ordinary prose. Peeling the
	// tilde first leaves it glued to the tag, where it stays literal title text.
	// Removal is parsed before addition so the two-rune trigger is peeled whole;
	// the leading-whitespace boundary already keeps ~ out of "-~12", but the order
	// is what makes that independent of the boundary rule.
	{
		Definition: Definition{
			Field: FieldRemoveDependency, Group: "Dependencies", Trigger: "-~",
			Description: "Remove a dependency (repeatable)",
			Examples:    []string{"-~1", "-~project-slug:1"},
			// Capture carries the field only so it can be refused by name. It
			// declares no capture form, so the guide never advertises it there.
			Surfaces: allSurfaces, Cardinality: CardinalityRepeatable,
		},
		forms: []surfaceForm{
			{Text: "-~task", Surfaces: SurfaceEdit | SurfaceOutline | SurfaceSearch},
		},
		pattern:           `(?:^|\s)-~(\S+)`,
		validation:        validateAny,
		negates:           FieldDependency,
		searchDescription: "Exclude a direct dependency",
	},
	{
		Definition: Definition{
			Field: FieldDependency, Group: "Dependencies", Trigger: "~",
			Description: "Depend on another task (repeatable)",
			Examples:    []string{"~1", "~project-slug:1"},
			Surfaces:    allSurfaces, Cardinality: CardinalityRepeatable,
			// Clearing belongs to the two surfaces that hold a stored set. Search has
			// nothing to clear, so ~0 is guidance there rather than a form.
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear dependencies", Surfaces: SurfaceEdit | SurfaceOutline}},
		},
		forms: []surfaceForm{
			{Text: "~task", Surfaces: allSurfaces},
			{Text: "~0", Surfaces: SurfaceEdit | SurfaceOutline},
		},
		pattern:           `(?:^|\s)~(\S+)`,
		validation:        validateAny,
		searchDescription: "Match a direct dependency",
	},
	{
		Definition: Definition{
			Field: FieldRemovePriority, Group: "Priority & Tags", Trigger: "-!",
			Values: priorityNames, RetiredValues: retiredPriorityNames,
			Examples: []string{"-!1", "-!3+"}, Surfaces: SurfaceSearch,
			Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "-!1-3", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)-!([^\s]+)`, validation: validatePriority,
		negates:           FieldPriority,
		searchDescription: "Exclude a priority or threshold",
	},
	{
		Definition: Definition{
			Field: FieldPriority, Group: "Priority & Tags", Trigger: "!",
			Description: "Set priority", Values: priorityNames, RetiredValues: retiredPriorityNames,
			Examples: []string{"!1", "!2", "!3"}, Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
			Clears:      []ClearOperation{{Value: clearValue, Description: "Clear priority", Surfaces: SurfaceEdit | SurfaceSearch}},
		},
		forms: []surfaceForm{
			{Text: "!1-3", Surfaces: allSurfaces},
			{Text: "!0", Surfaces: SurfaceEdit | SurfaceSearch},
		},
		pattern: `(?:^|\s)!([^\s]+)`, validation: validatePriority,
		// The threshold is accepted in validatePriority rather than in the pattern:
		// the capture group already spans "2+", and moving the "+" into the regexp
		// would make one surface's vocabulary a property of the shape every surface
		// is peeled with. Only search reads the suffix.
		searchDescription: "Match a priority, or !N+ for that priority and above",
		searchClearText:   map[string]string{clearValue: "Match tasks with no priority"},
	},
	{
		Definition: Definition{
			Field: FieldRemoveContext, Group: "Context", Trigger: "-##",
			Examples: []string{"-##frontend", "-##client-acme"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "-##context", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)-##([\w][\w.:-]*)`, validation: validateTag,
		negates:           FieldContext,
		searchDescription: "Exclude a context prefix",
	},
	{
		Definition: Definition{
			Field: FieldContext, Group: "Context", Trigger: "##",
			Description: "Set context", Examples: []string{"##frontend", "##client-acme"},
			Surfaces: allSurfaces, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear context", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "##context", Surfaces: allSurfaces},
			{Text: "##0", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)##([\w][\w.:-]*)`, validation: validateTag,
		searchDescription: "Match a context prefix",
	},
	{
		Definition: Definition{
			Field: FieldRemoveTag, Group: "Priority & Tags", Trigger: "-#",
			Description: "Remove tag", Examples: []string{"-#old", "-#source:github"},
			Surfaces: SurfaceEdit | SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-#tag", Surfaces: SurfaceEdit | SurfaceSearch}},
		pattern:           `(?:^|\s)-#([\w][\w.:-]*)`,
		validation:        validateTag,
		negates:           FieldTag,
		searchDescription: "Exclude a tag prefix",
	},
	{
		Definition: Definition{
			Field: FieldTag, Group: "Priority & Tags", Trigger: "#",
			Description: "Add tag", Examples: []string{"#backend", "#bug", "#source:github"},
			Surfaces: allSurfaces, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "#tag", Surfaces: allSurfaces}},
		pattern:           `(?:^|\s)#([\w][\w.:-]*)`,
		validation:        validateTag,
		searchDescription: "Match a tag prefix",
	},
	{
		Definition: Definition{
			Field: FieldRemoveDefer, Group: "Schedule", Trigger: "-@@",
			Values:   deferValueNames,
			Examples: []string{"-@@tomorrow", "-@@+3d"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-@@defer", Surfaces: SurfaceSearch}},
		pattern:           `(?i)(?:^|\s)-@@(next\s+\w+|in\s+\d+\s+\w+|\S+)`,
		negates:           FieldDefer,
		searchDescription: "Exclude tasks deferred until that day",
	},
	{
		Definition: Definition{
			Field: FieldDefer, Group: "Schedule", Trigger: "@@",
			Description: "Defer task until a later instant",
			Values:      deferValueNames,
			Examples:    []string{"@@+2h", "@@tomorrow", "@@+3d"},
			Surfaces:    SurfaceCapture | SurfaceEdit | SurfaceSearch,
			Cardinality: CardinalitySingleton,
			Clears:      []ClearOperation{{Value: clearValue, Description: "Clear deferral", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "@@defer", Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch},
			{Text: "@@0", Surfaces: SurfaceEdit},
		},
		pattern:           `(?i)(?:^|\s)@@(next\s+\w+|in\s+\d+\s+\w+|\S+)`,
		searchDescription: "Match tasks deferred until that day",
	},
	{
		Definition: Definition{
			Field: FieldRemoveDue, Group: "Schedule", Trigger: "-@",
			Values:   dueValueNames,
			Examples: []string{"-@today", "-@friday", "-@+3d"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-@date", Surfaces: SurfaceSearch}},
		pattern:           `(?i)(?:^|\s)-@((?:next\s+\w+|in\s+\d+\s+\w+|[^\s@]\S*)(?:\s+\d{1,2}:\d{2})?)`,
		negates:           FieldDue,
		searchDescription: "Exclude a due bucket, day, or +Nh window",
	},
	{
		Definition: Definition{
			Field: FieldDue, Group: "Schedule", Trigger: "@",
			Description: "Set due date",
			Values:      dueValueNames,
			Examples:    []string{"@tomorrow", "@friday", "@+3d", "@next monday", "@tomorrow 15:30", "@2026-08-25 09:00"},
			Surfaces:    allSurfaces, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Clear due date", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: "@date", Surfaces: allSurfaces},
			{Text: "@0", Surfaces: SurfaceEdit},
		},
		// The date token refuses a leading @ (so @@ can never be consumed here,
		// whatever order the registry is walked in — no legal due value starts
		// with @) and may carry an optional trailing hh:mm clock time, which is
		// the only way to set a due time. The hour-offset form (+Nh) is still
		// refused downstream: hours are defer-only.
		pattern:           `(?i)(?:^|\s)@((?:next\s+\w+|in\s+\d+\s+\w+|[^\s@]\S*)(?:\s+\d{1,2}:\d{2})?)`,
		searchDescription: "Match a due bucket, exact due day, day and hh:mm time, or +Nh window",
	},
	{
		Definition: Definition{
			Field: FieldRemoveDestination, Group: "Destination", Trigger: "-$",
			Values:   destinationNames,
			Examples: prefixed(destinationNames, "-$"), Surfaces: SurfaceSearch,
			Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-$destination", Surfaces: SurfaceSearch}},
		pattern:           `(?:^|\s)-\$(\w+)`,
		negates:           FieldDestination,
		searchDescription: "Exclude a lifecycle state or planning list",
	},
	{
		Definition: Definition{
			Field: FieldDestination, Group: "Destination", Trigger: "$",
			Description: "Set lifecycle or planning destination", Values: destinationNames,
			Examples: prefixed(destinationNames, "$"), Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
		},
		forms:             []surfaceForm{{Text: "$destination", Surfaces: allSurfaces}},
		pattern:           `(?:^|\s)\$(\w+)`,
		searchDescription: "Match a lifecycle state or planning list",
	},
	// The pin negation declares no Clears, and that absence is what keeps the
	// grammar to one negation operator: validateClosedSet admits this spec's Values plus
	// its own clear values, so -* accepts "pin" alone and "-*unpin" stays prose
	// rather than minting a second, doubly negative spelling of "pinned".
	// "*unpin" survives as an input spelling of the same predicate.
	{
		Definition: Definition{
			Field: FieldRemovePinned, Group: "Pinning", Trigger: "-*",
			Values:   []string{"pin"},
			Examples: []string{"-*pin"}, Surfaces: SurfaceSearch,
			Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "-*pin", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)-\*(\S+)`, validation: validateClosedSet,
		negates:           FieldPinned,
		searchDescription: "Match unpinned tasks",
	},
	// Pin advertises only the set value. Definitions does not filter Values by
	// surface, so listing the clear value there would offer it while capturing,
	// exactly as priority keeps its 0 out of Values. An unrecognized value is not
	// consumed, because * occurs in ordinary prose and in code identifiers.
	{
		Definition: Definition{
			Field: FieldPinned, Group: "Pinning", Trigger: "*",
			Description: "Pin task", Values: []string{"pin"},
			Examples: []string{"*pin"}, Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{
				{Value: "unpin", Description: "Unpin task", Surfaces: SurfaceEdit | SurfaceSearch},
			},
		},
		forms: []surfaceForm{
			{Text: "*pin", Surfaces: allSurfaces},
			{Text: "*unpin", Surfaces: SurfaceEdit | SurfaceSearch},
		},
		pattern: `(?:^|\s)\*(\S+)`, validation: validateClosedSet,
		searchDescription: "Match pinned tasks",
		searchClearText:   map[string]string{"unpin": "Match unpinned tasks"},
	},
	// Review mirrors pinning exactly, because it is the same kind of thing: a
	// stored boolean the user sets and clears, orthogonal to the lifecycle state
	// the task is in. The negation declares no Clears for the same reason pinning's
	// does not, so "-=unreview" stays prose instead of minting a double negative.
	{
		Definition: Definition{
			Field: FieldRemoveReview, Group: "Review", Trigger: "-=",
			Values:   []string{"review"},
			Examples: []string{"-=review"}, Surfaces: SurfaceSearch,
			Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "-=review", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)-=(\S+)`, validation: validateClosedSet,
		negates:           FieldReview,
		searchDescription: "Match tasks not flagged for review",
	},
	// Review advertises only the set value, exactly as pinning does: listing the
	// clear value in Values would offer it while capturing, where it is refused by
	// name. An unrecognized value is not consumed, because = occurs in ordinary
	// prose and in code ("a=b", "-=reviewCount").
	{
		Definition: Definition{
			Field: FieldReview, Group: "Review", Trigger: "=",
			Description: "Flag task for review", Values: []string{"review"},
			Examples: []string{"=review"}, Surfaces: allSurfaces,
			Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{
				{Value: "unreview", Description: "Clear review flag", Surfaces: SurfaceEdit | SurfaceSearch},
			},
		},
		forms: []surfaceForm{
			{Text: "=review", Surfaces: allSurfaces},
			{Text: "=unreview", Surfaces: SurfaceEdit | SurfaceSearch},
		},
		pattern: `(?:^|\s)=(\S+)`, validation: validateClosedSet,
		searchDescription: "Match tasks flagged for review",
		searchClearText:   map[string]string{"unreview": "Match tasks not flagged for review"},
	},
	{
		Definition: Definition{
			Field: FieldRemoveProject, Group: "Project Routing", Trigger: "-%",
			Examples: []string{"-%project-slug"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:             []surfaceForm{{Text: "-%project", Surfaces: SurfaceSearch}},
		pattern:           `(?:^|\s)-%(\S+)`,
		negates:           FieldProject,
		searchDescription: "Drop matching scope projects",
	},
	{
		Definition: Definition{
			Field: FieldProject, Group: "Project Routing", Trigger: "%",
			Description: "Move to project", Examples: []string{"%project-slug"},
			Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch, Cardinality: CardinalitySingleton,
		},
		forms:             []surfaceForm{{Text: "%project", Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch}},
		pattern:           `(?:^|\s)%(\S+)`,
		searchDescription: "Narrow to matching scope projects",
	},
	{
		Definition: Definition{
			Field: FieldRemoveDerived, Group: "Derived", Trigger: "-?",
			Values:   derivedNames,
			Examples: []string{"-?review", "-?stale", "-?blocked", "-?completed:90d"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "-?derived", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)-\?(\S+)`, validation: validateDerived,
		negates:           FieldDerived,
		searchDescription: "Exclude current review attention or another computed property",
	},
	{
		Definition: Definition{
			Field: FieldDerived, Group: "Derived", Trigger: "?",
			Values:   derivedNames,
			Examples: []string{"?review", "?stale", "?leaf", "?blocking", "?completed:90d"},
			Surfaces: SurfaceSearch, Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "?derived", Surfaces: SurfaceSearch}},
		pattern: `(?:^|\s)\?(\S+)`, validation: validateDerived,
		searchDescription: "Match current review attention or another computed property",
	},
	{
		Definition: Definition{
			Field: FieldParent, Group: "Placement", Trigger: ">",
			Description: "Set parent task", Examples: []string{">1", ">Login"},
			Surfaces: SurfaceCapture | SurfaceEdit, Cardinality: CardinalitySingleton,
			Clears: []ClearOperation{{Value: clearValue, Description: "Move to project root", Surfaces: SurfaceEdit}},
		},
		forms: []surfaceForm{
			{Text: ">parent", Surfaces: SurfaceCapture | SurfaceEdit},
			{Text: ">0", Surfaces: SurfaceEdit},
		},
		pattern: `(?:^|\s)>(\S+)`,
	},
	// Child declares no clear operation, which is what makes <0 an ordinary
	// reference that never resolves rather than a clear. Its pattern requires the
	// same leading whitespace boundary as parent's, so embedded comparison text
	// such as a<b stays literal. The description carries the repeatability hint
	// because child sits beside singleton parent in one group with a near-identical
	// form, so the two would otherwise read as the same cardinality.
	{
		Definition: Definition{
			Field: FieldChild, Group: "Placement", Trigger: "<",
			Description: "Adopt existing task as child (repeatable)", Examples: []string{"<1", "<Login"},
			Surfaces: SurfaceCapture | SurfaceEdit, Cardinality: CardinalityRepeatable,
		},
		forms:   []surfaceForm{{Text: "<child", Surfaces: SurfaceCapture | SurfaceEdit}},
		pattern: `(?:^|\s)<(\S+)`,
	},
	// Retired sigils remain recognized only so capture can provide actionable
	// migration errors. Legacy entries are not advertised by Definitions.
	{
		Definition: Definition{
			Field: FieldTaskType, Group: "Legacy", Trigger: "^", Description: "Retired task type",
			Examples: []string{"^bug"}, Legacy: true, Surfaces: SurfaceCapture,
			Cardinality: CardinalitySingleton,
		},
		forms:   []surfaceForm{{Text: "^type", Surfaces: SurfaceCapture}},
		pattern: `(?:^|\s)\^(\S+)`,
	},
}
