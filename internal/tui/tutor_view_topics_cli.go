package tui

// tutorCLITopics returns tutor topics for CLI workflows.
func tutorCLITopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Blockers",
			Category: "CLI",
			Content: `# Adding Blockers

  tt blocker add "Waiting for API access"

# Listing and Viewing

  tt blocker                   # List all blockers
  tt blocker list              # Same as above
  tt blocker show abc12345     # Show blocker details

# Resolving and Deleting

  tt blocker resolve abc12345  # Mark as resolved
  tt blocker delete abc12345   # Remove entirely

# Editing

  tt blocker edit abc12345`,
		},
		{
			Title:    "Board (Actions)",
			Category: "CLI",
			Content: `# What Is the Action Board

The action board manages lightweight action items from external
sources like Slack, GitHub, or email. Actions are stored as tasks
in a dedicated action inbox project.

# Adding Actions

  tt board add "Review PR #432" --url https://... --source github
  tt board add "Reply to thread" --source slack --priority 2
  tt bd add "Quick note" --priority 1

# Managing Actions

  tt board                     # List open actions
  tt board list --all          # Include done/dismissed
  tt board done 1              # Mark action done
  tt board dismiss 2           # Dismiss as irrelevant
  tt board reopen 3            # Reopen dismissed action`,
		},
		{
			Title:    "Contexts",
			Category: "CLI",
			Content: `# Custom Contexts

tt context manages custom contexts: named, hand-picked groups
of projects stored per machine.

  tt context new work
  tt context list
  tt context show work

# Managing Projects in Custom Contexts

  tt context add work project-alpha
  tt context remove work project-alpha
  tt context edit work
  tt context delete work

# Context Labels

Every project also carries one context label, set at creation
or with tt project edit. Labels live in the workspace catalog
and sync with it; custom contexts stay machine-local.

  tt project new "Client Site" --context work
  tt project edit --context personal

# Applying A Context

Applying a context is a session-only TUI scope change, so
there is no CLI switch command. Press C in the Outliner.`,
		},
		{
			Title:    "Comments",
			Category: "CLI",
			Content: `# Adding Comments

  tt task comment add 5 "Found the root cause"
  tt task comment add 5 "Tried approach X, didn't work"
  tt t c a 5 "Quick note"

# Listing Comments

  tt task comment list 5

# Resolving and Editing

  tt task comment resolve 5 0
  tt task comment edit 5 0 "Updated note"

# Removing Comments

  tt task comment remove 5 0`,
		},
		{
			Title:    "Focus",
			Category: "CLI",
			Content: `# Actionable Tasks Across Projects

  tt focus                # All actionable tasks
  tt focus queue          # Show the focus work queue

# Ranked Suggestions

The suggestion engine scores tasks based on priority, due dates,
status, activity recency, unblocking potential, and scoring profiles.

  tt focus suggest        # Top 10 suggestions
  tt focus suggest -n 5   # Top 5 suggestions

# Next (the on-deck shortlist)

Next is your standing, ordered shortlist of what to work on next.
A task is on Next exactly when its workflow state is StateNext, so
queueing a task moves it into that state rather than setting a
separate flag. The persisted order index is ordering-only: it
records the sequence of Next tasks and never decides membership.

  tt focus queue          # List the ordered Next shortlist
  tt task next <id>       # Put a task on Next (StateNext)

Next is distinct from Today (the daily plan) and from the
ranked Backlog; a task is in at most one of Today or Next.
Manage Next from the Outliner.`,
		},
		{
			Title:    "Notes",
			Category: "CLI",
			Content: `# Managing Notes

  tt note add "Sprint Planning Notes"
  tt note list
  tt note show <id>

# Global Notes

  tt note add --global "Architecture Decisions"`,
		},
		{
			Title:    "Projects",
			Category: "CLI",
			Content: `# Creating Projects

Every task in tt belongs to a project. Create your first project
and start organizing work immediately.

  tt project new "My Project"
  tt project new "Work Tasks" --context work

# Selecting Projects

Switch between projects to focus on different areas of work. The
selected project is used by all task commands.

  tt select
  tt project list

# Pinning and Archiving

Pin frequently-used projects so they appear at the top of the
Outliner. Archive completed projects to keep things tidy.

  tt project pin "My Project"
  tt project archive "Old Project"`,
		},
		{
			Title:    "Quick Capture",
			Category: "CLI",
			Content: `# Inline Syntax

The tt add command accepts inline modifiers. Priority is set with !
and tags with #.

  tt add "Fix login bug !1 #backend"
  tt add "Update docs !3 #frontend"
  tt add "Research caching #perf"

# Sections in TUI Quick Capture

Inside the TUI Quick Capture input, use / followed by a GTD section.
This sigil is not parsed by the tt add command.

  Fix login bug !1 #backend /inbox
  Research caching #perf /next

# Priority Shorthand

Priority ranges from 1 (highest) to 5 (lowest). Use ! followed by
the number anywhere in the task title.

  tt add "Critical fix !1"
  tt add "Nice to have !4"

# Tags and Sections

Tags help you categorize tasks across projects. Sections follow
GTD methodology for workflow organization. In the CLI, set a section
with tt task add --section.

  Deploy service #ops #urgent /review
  tt task add "Deploy service" --tag ops --tag urgent --section Next`,
		},
		{
			Title:    "Scoring",
			Category: "CLI",
			Content: `# Managing Scoring Profiles

  tt score profile list
  tt score profile add
  tt score profile show <name>
  tt score profile edit <name>
  tt score profile delete <name>

# Assigning Profiles to Projects

  tt score assign <profile-name>
  tt score unassign

# Setting Dimension Values

  tt score set <task-id> <dimension> <value>
  tt score show <task-id>

# Ranking Tasks

  tt score rank`,
		},
		{
			Title:    "Search",
			Category: "CLI",
			Content: `# Searching Tasks

Search across all tasks in the current project or globally
across all projects.

  tt search "login bug"
  tt search --all "authentication"

# Filtering Options

Combine search with filters to narrow results by status,
priority, tag, section, or parked state.

  tt task list --status todo --tag backend
  tt task list --parked
  tt task list --priority 1`,
		},
		{
			Title:    "Workflow States",
			Category: "CLI",
			Content: `# Setting a Workflow State

  tt task state in_progress 5
  tt task state waiting 12
  tt task state review web:#3

# Planning Shortcuts

  tt task ready 5
  tt task next 12`,
		},
		{
			Title:    "Task Management",
			Category: "CLI",
			Content: `# Creating Tasks

Add tasks with type and parent options for hierarchy.

  tt task add --type epic "User Authentication"
  tt task add --type story "Login Flow" --parent <epic-id>
  tt task add --type bug "Fix timeout" --parent <story-id>

# Status Transitions

  tt task start <id>
  tt task complete <id>
  tt task cancel <id>
  tt task block <id>

# Priority and Workflow State

  tt task priority <id> 2
  tt task state ready <id>
  tt task state next <id>

# Dependencies

  tt task depends <task-id> <blocker-id>

# Snooze

  tt task snooze <id> 2d
  tt task snooze <id> 1w`,
		},
		{
			Title:    "Templates",
			Category: "CLI",
			Content: `# Initialize Default Templates

  tt template init

# Managing Templates

  tt template list
  tt template show def-bug-report
  tt template toggle def-bug-report
  tt template delete my-template

# Creating Tasks from Templates

  tt template use def-bug-report "Login fails on mobile"

# Adding Custom Templates

  tt template add -n "My Workflow" --type task --item "Step 1" --item "Step 2"`,
		},
		{
			Title:    "Workflow Rules",
			Category: "CLI",
			Content: `# Managing Workflow Rules

Create, list, and delete workflow rules from the command line.

  tt workflow add
  tt workflow list
  tt workflow delete <id>`,
		},
	}
}
