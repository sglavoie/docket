package format

import (
	"strconv"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func metadataTypedFieldCodecs() []metadataFieldCodec {
	return []metadataFieldCodec{
		metadataBugReproducibleFieldCodec(),
		metadataBugSeverityFieldCodec(),
		metadataBugWorkaroundFieldCodec(),
		metadataIdeaCategoryFieldCodec(),
		metadataIdeaEffortFieldCodec(),
		metadataIdeaValueFieldCodec(),
		metadataIdeaValidatedFieldCodec(),
		metadataTaskEffortFieldCodec(),
		metadataTaskWorkTypeFieldCodec(),
		metadataStoryPointsFieldCodec(),
		metadataStoryPersonaFieldCodec(),
		metadataEpicScopeFieldCodec(),
		metadataEpicMilestoneFieldCodec(),
		metadataEpicMetricsFieldCodec(),
		metadataEpicInitialChildrenFieldCodec(),
		metadataEpicScopeBaselinedFieldCodec(),
	}
}

func typedEnumField[T ~string](key string, taskType models.TaskType, parse func(string) T, isValid func(T) bool, set func(*models.Task, T), get func(*models.Task) (T, bool)) metadataFieldCodec {
	return typedMetadataField(key, taskType,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			parsed := parse(value)
			if !isValid(parsed) {
				addValidationWarning(warnings, key, "invalid %s %q, ignoring", key, value)
				return
			}
			set(task, parsed)
		},
		func(task *models.Task, parts *[]string) {
			if value, ok := get(task); ok && value != "" {
				appendMetadataPart(parts, key, string(value))
			}
		},
	)
}

func typedEscapedStringField(key string, taskType models.TaskType, set func(*models.Task, string), get func(*models.Task) (string, bool)) metadataFieldCodec {
	return typedMetadataField(key, taskType,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			set(task, unescapeNewlines(value))
		},
		func(task *models.Task, parts *[]string) {
			if value, ok := get(task); ok && value != "" {
				appendMetadataPart(parts, key, escapeNewlines(value))
			}
		},
	)
}

func typedBoolTrueField(key string, taskType models.TaskType, set func(*models.Task, bool), get func(*models.Task) (bool, bool)) metadataFieldCodec {
	return typedMetadataField(key, taskType,
		func(task *models.Task, value string, _ *metadataParseState, _ *[]ParseWarning) {
			set(task, value == "true")
		},
		func(task *models.Task, parts *[]string) {
			if value, ok := get(task); ok && value {
				appendMetadataPart(parts, key, "true")
			}
		},
	)
}

func typedIntField(key string, taskType models.TaskType, parse func(string, *[]ParseWarning) (int, bool), set func(*models.Task, int), get func(*models.Task) (int, bool), shouldWrite func(int) bool) metadataFieldCodec {
	return typedMetadataField(key, taskType,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, ok := parse(value, warnings); ok {
				set(task, parsed)
			}
		},
		func(task *models.Task, parts *[]string) {
			if value, ok := get(task); ok && shouldWrite(value) {
				appendMetadataPart(parts, key, strconv.Itoa(value))
			}
		},
	)
}

func typedDatePtrField(key string, taskType models.TaskType, set func(*models.Task, *time.Time), get func(*models.Task) (*time.Time, bool)) metadataFieldCodec {
	return typedMetadataField(key, taskType,
		func(task *models.Task, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			if parsed, ok := parseDateOrWarn(value, key, warnings); ok {
				set(task, &parsed)
			}
		},
		func(task *models.Task, parts *[]string) {
			if value, ok := get(task); ok && value != nil {
				appendMetadataPart(parts, key, value.Format(util.DateFormat))
			}
		},
	)
}

func metadataBugReproducibleFieldCodec() metadataFieldCodec {
	return typedEnumField("bug.reproducible", models.TaskTypeBug,
		func(value string) models.BugReproducible { return models.BugReproducible(value) },
		func(value models.BugReproducible) bool { return value.IsValid() },
		func(task *models.Task, value models.BugReproducible) { task.EnsureBugAttrs().Reproducible = value },
		func(task *models.Task) (models.BugReproducible, bool) {
			if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Reproducible != "" {
				return task.BugAttrs.Reproducible, true
			}
			return "", false
		},
	)
}

func metadataBugSeverityFieldCodec() metadataFieldCodec {
	return typedEnumField("bug.severity", models.TaskTypeBug,
		func(value string) models.BugSeverity { return models.BugSeverity(value) },
		func(value models.BugSeverity) bool { return value.IsValid() },
		func(task *models.Task, value models.BugSeverity) { task.EnsureBugAttrs().Severity = value },
		func(task *models.Task) (models.BugSeverity, bool) {
			if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Severity != "" {
				return task.BugAttrs.Severity, true
			}
			return "", false
		},
	)
}

func metadataBugWorkaroundFieldCodec() metadataFieldCodec {
	return typedEscapedStringField("bug.workaround", models.TaskTypeBug,
		func(task *models.Task, value string) { task.EnsureBugAttrs().Workaround = value },
		func(task *models.Task) (string, bool) {
			if task.BugAttrs != nil && !task.BugAttrs.IsEmpty() && task.BugAttrs.Workaround != "" {
				return task.BugAttrs.Workaround, true
			}
			return "", false
		},
	)
}

func metadataIdeaCategoryFieldCodec() metadataFieldCodec {
	return typedEnumField("idea.category", models.TaskTypeIdea,
		func(value string) models.IdeaCategory { return models.IdeaCategory(value) },
		func(value models.IdeaCategory) bool { return value.IsValid() },
		func(task *models.Task, value models.IdeaCategory) { task.EnsureIdeaAttrs().Category = value },
		func(task *models.Task) (models.IdeaCategory, bool) {
			if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Category != "" {
				return task.IdeaAttrs.Category, true
			}
			return "", false
		},
	)
}

func metadataIdeaEffortFieldCodec() metadataFieldCodec {
	return typedEnumField("idea.effort", models.TaskTypeIdea,
		func(value string) models.EffortEstimate { return models.EffortEstimate(value) },
		func(value models.EffortEstimate) bool { return value.IsValid() },
		func(task *models.Task, value models.EffortEstimate) { task.EnsureIdeaAttrs().Effort = value },
		func(task *models.Task) (models.EffortEstimate, bool) {
			if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Effort != "" {
				return task.IdeaAttrs.Effort, true
			}
			return "", false
		},
	)
}

func metadataIdeaValueFieldCodec() metadataFieldCodec {
	return typedEnumField("idea.value", models.TaskTypeIdea,
		func(value string) models.IdeaValue { return models.IdeaValue(value) },
		func(value models.IdeaValue) bool { return value.IsValid() },
		func(task *models.Task, value models.IdeaValue) { task.EnsureIdeaAttrs().Value = value },
		func(task *models.Task) (models.IdeaValue, bool) {
			if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() && task.IdeaAttrs.Value != "" {
				return task.IdeaAttrs.Value, true
			}
			return "", false
		},
	)
}

func metadataIdeaValidatedFieldCodec() metadataFieldCodec {
	return typedBoolTrueField("idea.validated", models.TaskTypeIdea,
		func(task *models.Task, value bool) { task.EnsureIdeaAttrs().Validated = value },
		func(task *models.Task) (bool, bool) {
			if task.IdeaAttrs != nil && !task.IdeaAttrs.IsEmpty() {
				return task.IdeaAttrs.Validated, true
			}
			return false, false
		},
	)
}

func metadataTaskEffortFieldCodec() metadataFieldCodec {
	return typedEnumField("task.effort", models.TaskTypeTask,
		func(value string) models.EffortEstimate { return models.EffortEstimate(value) },
		func(value models.EffortEstimate) bool { return value.IsValid() },
		func(task *models.Task, value models.EffortEstimate) { task.EnsureTaskAttrs().Effort = value },
		func(task *models.Task) (models.EffortEstimate, bool) {
			if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.Effort != "" {
				return task.TaskAttrs.Effort, true
			}
			return "", false
		},
	)
}

func metadataTaskWorkTypeFieldCodec() metadataFieldCodec {
	return typedEnumField("task.worktype", models.TaskTypeTask,
		func(value string) models.WorkType { return models.WorkType(value) },
		func(value models.WorkType) bool { return value.IsValid() },
		func(task *models.Task, value models.WorkType) { task.EnsureTaskAttrs().WorkType = value },
		func(task *models.Task) (models.WorkType, bool) {
			if task.TaskAttrs != nil && !task.TaskAttrs.IsEmpty() && task.TaskAttrs.WorkType != "" {
				return task.TaskAttrs.WorkType, true
			}
			return "", false
		},
	)
}

func metadataStoryPointsFieldCodec() metadataFieldCodec {
	return typedIntField("story.points", models.TaskTypeStory,
		func(value string, warnings *[]ParseWarning) (int, bool) {
			points, ok := parseIntOrWarn(value, "story.points", warnings)
			if !ok {
				return 0, false
			}
			if points < 0 {
				addValidationWarning(warnings, "story.points", "invalid story.points %d (must be non-negative), ignoring", points)
				return 0, false
			}
			return points, true
		},
		func(task *models.Task, value int) { task.EnsureStoryAttrs().StoryPoints = value },
		func(task *models.Task) (int, bool) {
			if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() {
				return task.StoryAttrs.StoryPoints, true
			}
			return 0, false
		},
		func(value int) bool { return value > 0 },
	)
}

func metadataStoryPersonaFieldCodec() metadataFieldCodec {
	return typedEscapedStringField("story.persona", models.TaskTypeStory,
		func(task *models.Task, value string) { task.EnsureStoryAttrs().Persona = value },
		func(task *models.Task) (string, bool) {
			if task.StoryAttrs != nil && !task.StoryAttrs.IsEmpty() && task.StoryAttrs.Persona != "" {
				return task.StoryAttrs.Persona, true
			}
			return "", false
		},
	)
}

func metadataEpicScopeFieldCodec() metadataFieldCodec {
	return typedEnumField("epic.scope", models.TaskTypeEpic,
		func(value string) models.EpicScope { return models.EpicScope(value) },
		func(value models.EpicScope) bool { return value.IsValid() },
		func(task *models.Task, value models.EpicScope) { task.EnsureEpicAttrs().Scope = value },
		func(task *models.Task) (models.EpicScope, bool) {
			if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Scope != "" {
				return task.EpicAttrs.Scope, true
			}
			return "", false
		},
	)
}

func metadataEpicMilestoneFieldCodec() metadataFieldCodec {
	return typedEscapedStringField("epic.milestone", models.TaskTypeEpic,
		func(task *models.Task, value string) { task.EnsureEpicAttrs().Milestone = value },
		func(task *models.Task) (string, bool) {
			if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.Milestone != "" {
				return task.EpicAttrs.Milestone, true
			}
			return "", false
		},
	)
}

func metadataEpicMetricsFieldCodec() metadataFieldCodec {
	return typedEscapedStringField("epic.metrics", models.TaskTypeEpic,
		func(task *models.Task, value string) { task.EnsureEpicAttrs().SuccessMetrics = value },
		func(task *models.Task) (string, bool) {
			if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.SuccessMetrics != "" {
				return task.EpicAttrs.SuccessMetrics, true
			}
			return "", false
		},
	)
}

func metadataEpicInitialChildrenFieldCodec() metadataFieldCodec {
	return typedIntField("epic.initial_children", models.TaskTypeEpic,
		func(value string, warnings *[]ParseWarning) (int, bool) {
			children, ok := parseIntOrWarn(value, "epic.initial_children", warnings)
			if !ok || children <= 0 {
				return 0, false
			}
			return children, true
		},
		func(task *models.Task, value int) { task.EnsureEpicAttrs().InitialChildCount = value },
		func(task *models.Task) (int, bool) {
			if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() {
				return task.EpicAttrs.InitialChildCount, true
			}
			return 0, false
		},
		func(value int) bool { return value > 0 },
	)
}

func metadataEpicScopeBaselinedFieldCodec() metadataFieldCodec {
	return typedDatePtrField("epic.scope_baselined", models.TaskTypeEpic,
		func(task *models.Task, value *time.Time) { task.EnsureEpicAttrs().ScopeBaselinedAt = value },
		func(task *models.Task) (*time.Time, bool) {
			if task.EpicAttrs != nil && !task.EpicAttrs.IsEmpty() && task.EpicAttrs.ScopeBaselinedAt != nil {
				return task.EpicAttrs.ScopeBaselinedAt, true
			}
			return nil, false
		},
	)
}
