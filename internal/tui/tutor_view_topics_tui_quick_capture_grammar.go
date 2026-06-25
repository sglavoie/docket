package tui

func tutorTUIQuickCaptureGrammarTopic() TutorTopic {
	return TutorTopic{
		Title:    "Quick Capture Grammar",
		Category: "TUI",
		Content: `# Opening Quick Capture

Press ;i from any view to open the Quick Capture overlay. The
placeholder reads "Type a task title…" — start
typing the title, then add sigils as needed. Press ? to toggle
the full syntax reference (the footer hint reads "?: syntax").

The Linked Task Creator (blockers, dependents, subtasks,
follow-ups) shares the same calm placeholder, the same ?-gated
syntax guide, and the same sigils — except %project, which is
not offered because linked tasks always belong to the parent's
project.

# Live Preview Groups

As you type, the preview below the input organizes the parsed
task into labeled groups so you can see exactly which sigil
populates which field:

  Task        Task type + title (and any $status override)
  Route       /section (defaults to Inbox) and → ProjectName
              (global capture)
  Schedule    @due date and ~recurrence
  Metadata    !priority and #tags
  Aliases     Any long-form aliases you used (e.g. due:fri)

The Aliases row is a dedicated line — long-form aliases never
appear as an inline suffix on the title.

# Sigils By Group

  Task group
    ^type       Task type: ^epic ^story ^task ^bug ^idea
                Alias: type:bug
    $status     Initial status: $start $blocked $done $review
                $standby $hold $maybe $canceled
                Alias: status:done

  Route group
    /section    Section (defaults to Inbox if omitted):
                /inbox /next /waiting /someday
                Alias: sec:next
    %project    Target project (slug or name): %myproject
                Alias: p:myproject  (omitted in global capture
                and in the Linked Task Creator)

  Schedule group
    @date       Due date: @tomorrow @friday @+3d
                Alias: due:fri  due:3d
    ~recur      Recurrence: ~daily ~weekly ~2w ~monthly
                Alias: recur:weekly

  Metadata group
    !N/!word    Priority (1-5): !1 !urgent !high !normal !low
                Alias: pri:2
    #tag        Add a tag (multiple allowed): #backend #urgent
                Alias: tag:backend

  Hierarchy (no dedicated group; affects parenting)
    >parent     Parent task (short ID or title prefix): >42 >Login
                Alias: parent:42

# Worked Examples

  Buy milk @tomorrow !2 #errand
    Task: Buy milk · Route: Inbox (default)
    Schedule: tomorrow · Metadata: !2 #errand

  ^bug Login spinner !1 #frontend @eod $start
    Task: bug "Login spinner" (status: in_progress)
    Route: Inbox (default) · Schedule: end of day
    Metadata: !1 #frontend

  Pair with Alex ~weekly /waiting
    Task: Pair with Alex · Route: /waiting
    Schedule: weekly recurrence

# Global Search Equivalents

Global search ($status #tag @due ^type) uses the same sigil
prefixes for filtering, but targets existing tasks rather than
creating new ones. Press ? in Global Search to see the full
filter syntax reference.`,
	}
}
