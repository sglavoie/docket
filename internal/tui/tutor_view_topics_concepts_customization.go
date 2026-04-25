package tui

func tutorConceptTopicsCustomization() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Tags",
			Category: "Concepts",
			Content: `# What Are Tags

Tags are lightweight labels for cross-cutting categorization.
A task can have multiple tags, and tags work across projects.

# Use Cases

Use tags for themes that cut across your project hierarchy:
#backend, #frontend, #urgent, #tech-debt, #design. Tags complement
sections and priority by adding a free-form dimension to task
organization.`,
		},
		{
			Title:    "Task Types",
			Category: "Concepts",
			Content: `# Type Hierarchy

tt supports five task types organized in a hierarchy. Epics are
the largest unit, containing stories, tasks, bugs, and ideas.
Stories can contain tasks, ideas, and bugs. Tasks, bugs, and
ideas are leaf nodes that cannot contain children.

# When to Use Each

Epics represent large features or initiatives spanning weeks.
Stories are user-facing deliverables within an epic. Tasks are
concrete work items. Bugs track defects. Ideas capture future
possibilities without commitment.

See Type-Specific Attributes for per-type metadata.`,
		},
		{
			Title:    "Templates",
			Category: "Concepts",
			Content: `# What Are Templates

Templates are pre-configured task structures for common scenarios.
A template specifies type, priority, tags, section, attributes,
and optional checklist items that are applied when creating a task.

# Default Templates

tt ships with default templates for common workflows: Bug Report,
Production Incident, Feature Request, API Endpoint, UI Component,
Code Review Checklist, Release Checklist, and Sprint Planning.
Initialize them with tt template init.

# Custom Templates

Create your own templates for repeatable work patterns. Templates
can be enabled or disabled without deleting them.`,
		},
		{
			Title:    "Type-Specific Attributes",
			Category: "Concepts",
			Content: `# What Are Type-Specific Attributes

Each task type has optional attributes tailored to its purpose.
These provide structured metadata beyond the common fields.

# Bug Attributes

severity (critical, major, minor, cosmetic), reproducible
(yes, no, sometimes), and workaround (free text).

# Idea Attributes

category (feature, improvement, refactor, research), effort
(small, medium, large, unknown), value (low, medium, high),
and validated (boolean).

# Task Attributes

effort (small, medium, large, unknown) and work type
(development, documentation, testing, devops, design).

# Story Attributes

story points (integer) and persona (free text).

# Epic Attributes

scope (defined, evolving, undefined), milestone (free text),
and success metrics (free text).`,
		},
		{
			Title:    "Workflow Rules",
			Category: "Concepts",
			Content: `# What Are Workflow Rules

Workflow rules automate repetitive actions. When a trigger
condition is met, the associated action executes automatically.
Triggers can be state changes or events.

# Practical Examples

Auto-move tasks to Active section when started:

  Trigger: status changes to in_progress
  Action: set section to active

Auto-tag completed tasks:

  Trigger: status changes to done
  Action: add tag "completed"`,
		},
	}
}
