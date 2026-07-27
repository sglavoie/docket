package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/textgrammar"
)

func tutorTUIQuickCaptureGrammarTopic() TutorTopic {
	stateTokens := "$" + strings.Join(textgrammar.StateNames(), " $")
	typeTokens := "^" + strings.Join(textgrammar.TaskTypeNames(), " ^")

	return TutorTopic{
		Title:    "Task Composer Grammar",
		Category: "TUI",
		Content: `# Opening the Task Composer

Press a from the Outliner to open the Task Composer. Start with a
title, then add inline metadata. Press ? to toggle the grammar
reference. Capture and search share the same canonical state names
and token definitions.

# Live Preview

The preview groups every recognized field:

  Task        Task type + title + exact $state
  Route       Target project (capture defaults to Inbox)
  Schedule    @due date and ~recurrence
  Metadata    !priority and #tags
  Aliases     Any long-form compatibility aliases used

# Canonical Sigils

  Task
    ^type       Task type: ` + typeTokens + `
                Alias: type:bug
    $state      Canonical state: ` + stateTokens + `
                Alias: state:ready

  Routing
    %project    Target project: %myproject
                Aliases: p:myproject  proj:myproject
    >parent     Parent task: >42 >Login
                Alias: parent:42

  Schedule
    @date       Due date: @tomorrow @friday @+3d
                Alias: due:fri
    ~recur      Recurrence: ~daily ~weekly ~2w ~monthly
                Alias: recur:weekly

  Metadata
    !priority   Priority: !1 !urgent !high !normal !low
                Aliases: pri:2  priority:high
    #tag        Add tags: #backend #urgent
                Alias: tag:backend

# Canonical Lifecycle

Use only $state tokens for lifecycle metadata. Every created task
has one canonical state; omission defaults to Inbox.

# Worked Examples

  Buy milk @tomorrow !2 #errand $ready
  ^bug Login spinner !1 #frontend $in_progress
  Pair with Alex ~weekly $waiting

# Search Equivalents

Structured search uses the same $state, !priority, #tag, ^type, @due,
and project token definitions. State filters are exact: $ready matches
ready tasks without collapsing into inbox or next.`,
	}
}
