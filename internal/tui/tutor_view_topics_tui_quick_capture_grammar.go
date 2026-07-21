package tui

func tutorTUIQuickCaptureGrammarTopic() TutorTopic {
	return TutorTopic{
		Title:    "Quick Capture Grammar",
		Category: "TUI",
		Content: `# Opening Quick Capture

Press Ctrl+N from any non-text control to open the Task Composer. The
placeholder reads "Type a task title…" — start
typing the title, then add sigils as needed. Press ? to toggle
the grouped syntax reference — Basics, Priority & Tags,
Schedule, Placement, Status, and Project Routing (the footer
hint reads "?: syntax").

The Linked Task Creator (blockers, dependents, subtasks,
follow-ups) shares the same calm placeholder, the same ?-gated
syntax guide, and the same sigils — except %project, which is
not offered because linked tasks always belong to the parent's
project.

# Scrollable Syntax Reference

The composer (title, context, and three-row textarea) stays
fixed while you type. The grouped "Capture Syntax" reference
opens in a bounded pane you scroll on its own: Ctrl+J/K move it
by line and PgUp/PgDn move it by page. Direct j/k keep editing
the title. Esc closes autocomplete first, then the syntax pane,
and only then cancels the capture.

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

# Palette Search Equivalents

Palette search uses field prefixes to find existing tasks,
projects, tags, sections, commands, and notes.`,
	}
}
