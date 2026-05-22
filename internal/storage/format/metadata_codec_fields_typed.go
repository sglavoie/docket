package format

import (
	"strconv"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func metadataTypedFieldCodecs() []metadataFieldCodec {
	return []metadataFieldCodec{
		typedMetadataField("bug.reproducible", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				reproducible := models.BugReproducible(value)
				if !reproducible.IsValid() {
					addValidationWarning(warnings, "bug.reproducible", "invalid bug.reproducible %q, ignoring", value)
					return
				}
				task.EnsureBugAttrs().Reproducible = reproducible
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Reproducible != "" {
					appendMetadataPart(parts, "bug.reproducible", string(task.BugAttrs.Reproducible))
				}
			},
		),
		typedMetadataField("bug.severity", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				severity := models.BugSeverity(value)
				if !severity.IsValid() {
					addValidationWarning(warnings, "bug.severity", "invalid bug.severity %q, ignoring", value)
					return
				}
				task.EnsureBugAttrs().Severity = severity
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Severity != "" {
					appendMetadataPart(parts, "bug.severity", string(task.BugAttrs.Severity))
				}
			},
		),
		typedMetadataField("bug.workaround", models.TaskTypeBug,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureBugAttrs().Workaround = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Workaround != "" {
					appendMetadataPart(parts, "bug.workaround", escapeNewlines(task.BugAttrs.Workaround))
				}
			},
		),
		typedMetadataField("idea.category", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				category := models.IdeaCategory(value)
				if !category.IsValid() {
					addValidationWarning(warnings, "idea.category", "invalid idea.category %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Category = category
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Category != "" {
					appendMetadataPart(parts, "idea.category", string(task.IdeaAttrs.Category))
				}
			},
		),
		typedMetadataField("idea.effort", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				effort := models.EffortEstimate(value)
				if !effort.IsValid() {
					addValidationWarning(warnings, "idea.effort", "invalid idea.effort %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Effort = effort
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Effort != "" {
					appendMetadataPart(parts, "idea.effort", string(task.IdeaAttrs.Effort))
				}
			},
		),
		typedMetadataField("idea.value", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				ideaValue := models.IdeaValue(value)
				if !ideaValue.IsValid() {
					addValidationWarning(warnings, "idea.value", "invalid idea.value %q, ignoring", value)
					return
				}
				task.EnsureIdeaAttrs().Value = ideaValue
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Value != "" {
					appendMetadataPart(parts, "idea.value", string(task.IdeaAttrs.Value))
				}
			},
		),
		typedMetadataField("idea.validated", models.TaskTypeIdea,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureIdeaAttrs().Validated = value == "true"
			},
			func(task *models.Task, parts *[]string) {
				if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Validated {
					appendMetadataPart(parts, "idea.validated", "true")
				}
			},
		),
		typedMetadataField("task.effort", models.TaskTypeTask,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				effort := models.EffortEstimate(value)
				if !effort.IsValid() {
					addValidationWarning(warnings, "task.effort", "invalid task.effort %q, ignoring", value)
					return
				}
				task.EnsureTaskAttrs().Effort = effort
			},
			func(task *models.Task, parts *[]string) {
				if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.Effort != "" {
					appendMetadataPart(parts, "task.effort", string(task.TaskAttrs.Effort))
				}
			},
		),
		typedMetadataField("task.worktype", models.TaskTypeTask,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				workType := models.WorkType(value)
				if !workType.IsValid() {
					addValidationWarning(warnings, "task.worktype", "invalid task.worktype %q, ignoring", value)
					return
				}
				task.EnsureTaskAttrs().WorkType = workType
			},
			func(task *models.Task, parts *[]string) {
				if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.WorkType != "" {
					appendMetadataPart(parts, "task.worktype", string(task.TaskAttrs.WorkType))
				}
			},
		),
		typedMetadataField("story.points", models.TaskTypeStory,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if points, ok := parseIntOrWarn(value, "story.points", warnings); ok {
					if points < 0 {
						addValidationWarning(warnings, "story.points", "invalid story.points %d (must be non-negative), ignoring", points)
						return
					}
					task.EnsureStoryAttrs().StoryPoints = points
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() && task.StoryAttrs.StoryPoints > 0 {
					appendMetadataPart(parts, "story.points", strconv.Itoa(task.StoryAttrs.StoryPoints))
				}
			},
		),
		typedMetadataField("story.persona", models.TaskTypeStory,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureStoryAttrs().Persona = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() && task.StoryAttrs.Persona != "" {
					appendMetadataPart(parts, "story.persona", escapeNewlines(task.StoryAttrs.Persona))
				}
			},
		),
		typedMetadataField("epic.scope", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				scope := models.EpicScope(value)
				if !scope.IsValid() {
					addValidationWarning(warnings, "epic.scope", "invalid epic.scope %q, ignoring", value)
					return
				}
				task.EnsureEpicAttrs().Scope = scope
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Scope != "" {
					appendMetadataPart(parts, "epic.scope", string(task.EpicAttrs.Scope))
				}
			},
		),
		typedMetadataField("epic.milestone", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureEpicAttrs().Milestone = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Milestone != "" {
					appendMetadataPart(parts, "epic.milestone", escapeNewlines(task.EpicAttrs.Milestone))
				}
			},
		),
		typedMetadataField("epic.metrics", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
				task.EnsureEpicAttrs().SuccessMetrics = unescapeNewlines(value)
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.SuccessMetrics != "" {
					appendMetadataPart(parts, "epic.metrics", escapeNewlines(task.EpicAttrs.SuccessMetrics))
				}
			},
		),
		typedMetadataField("epic.initial_children", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if v, ok := parseIntOrWarn(value, "epic.initial_children", warnings); ok && v > 0 {
					task.EnsureEpicAttrs().InitialChildCount = v
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.InitialChildCount > 0 {
					appendMetadataPart(parts, "epic.initial_children", strconv.Itoa(task.EpicAttrs.InitialChildCount))
				}
			},
		),
		typedMetadataField("epic.scope_baselined", models.TaskTypeEpic,
			func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
				if t, ok := parseDateOrWarn(value, "epic.scope_baselined", warnings); ok {
					task.EnsureEpicAttrs().ScopeBaselinedAt = &t
				}
			},
			func(task *models.Task, parts *[]string) {
				if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.ScopeBaselinedAt != nil {
					appendMetadataPart(parts, "epic.scope_baselined", task.EpicAttrs.ScopeBaselinedAt.Format(util.DateFormat))
				}
			},
		),
	}
}
