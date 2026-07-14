package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/metrics"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/planner"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	roadmapCriticalPath bool
	roadmapZoom         string
	roadmapFormat       string
)

var roadmapCmd = &cobra.Command{
	Use:   "roadmap [project-name]",
	Short: "Display a roadmap (Gantt chart) for the project or all projects",
	Long: `Render a roadmap Gantt chart showing scheduled tasks, critical path, and
completion forecasts. When no project name is given, uses the current project.

Zoom levels:
  week     Day columns, ~2 months visible
  month    Week columns, ~6 months visible (default)
  quarter  Month columns, ~1 year visible

Output formats:
  ascii    Text Gantt chart (default)
  mermaid  Mermaid gantt block for embedding in Markdown
  json     Structured JSON with schedule, critical path, and forecasts`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRoadmap,
}

func init() {
	rootCmd.AddCommand(roadmapCmd)
	roadmapCmd.Flags().BoolVar(&roadmapCriticalPath, "critical-path", false, "Filter to critical path tasks only")
	roadmapCmd.Flags().StringVar(&roadmapZoom, "zoom", "month", "Timeline granularity: week|month|quarter")
	roadmapCmd.Flags().StringVar(&roadmapFormat, "format", "ascii", "Output format: ascii|mermaid|json")
}

func runRoadmap(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	var projects []*models.Project
	var allArchived []metrics.TaskSummary

	if len(args) > 0 {
		p := workspace.Runtime.GetProject(args[0])
		if p == nil {
			return fmt.Errorf("project not found: %s", args[0])
		}
		if err := storage.LoadProject(p); err != nil {
			return fmt.Errorf("failed to load project: %w", err)
		}
		projects = []*models.Project{p}
		summaries, err := roadmapArchiveSummariesForMetrics(p)
		if err != nil {
			if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping archive summaries for %s: %v\n", p.Name, err); writeErr != nil {
				return fmt.Errorf("failed to write warning: %w", writeErr)
			}
		} else {
			allArchived = append(allArchived, summaries...)
		}
	} else {
		active := workspace.Runtime.ActiveProjects()
		for _, p := range active {
			if err := storage.LoadProject(p); err != nil {
				if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping project %s: %v\n", p.Name, err); writeErr != nil {
					return fmt.Errorf("failed to write warning: %w", writeErr)
				}
				continue
			}
			projects = append(projects, p)
			summaries, err := roadmapArchiveSummariesForMetrics(p)
			if err != nil {
				if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping archive summaries for %s: %v\n", p.Name, err); writeErr != nil {
					return fmt.Errorf("failed to write warning: %w", writeErr)
				}
				continue
			}
			allArchived = append(allArchived, summaries...)
		}
	}

	if len(projects) == 0 {
		return fmt.Errorf("no projects found")
	}

	// Compute effort-weighted velocity. Use the user's configured week-start
	// so weekly throughput buckets align with their dashboard preferences.
	weekStart := metrics.WeekStartFromWeekEnd(workspace.Runtime.Settings().GetWeekEndDay())
	agg := metrics.ComputeAllAtWithWeekStartAndSummaries(projects, metrics.TimeRange30d, time.Now(), weekStart, allArchived)
	var allTasks []*models.Task
	for _, p := range projects {
		allTasks = append(allTasks, p.Tasks...)
	}
	var velocity float64
	if agg != nil {
		velocity = planner.WeightedVelocityWithSummaries(agg.Throughput, metrics.TaskSummariesFromTasks(allTasks), allArchived)
	}

	// Compute critical path and schedule.
	allIncomplete := planner.CollectAllIncompleteTasks(projects)
	criticalPath := planner.CriticalPath(allIncomplete)
	scheduled := planner.SerialSchedule(allIncomplete, time.Now(), velocity)

	if roadmapCriticalPath {
		var filtered []planner.ScheduledTask
		for _, s := range scheduled {
			if criticalPath.OnPath[s.Task.ID] {
				filtered = append(filtered, s)
			}
		}
		scheduled = filtered
	}

	switch roadmapFormat {
	case "mermaid":
		return renderMermaid(scheduled, criticalPath, projects)
	case "json":
		return renderJSON(scheduled, criticalPath, projects, velocity)
	default:
		return renderASCII(scheduled, criticalPath, projects, roadmapZoom, velocity)
	}
}

func roadmapArchiveSummariesForMetrics(project *models.Project) ([]metrics.TaskSummary, error) {
	summaries, err := storage.LoadArchivedTaskSummaries(project)
	if err != nil {
		return nil, err
	}
	return roadmapStorageSummariesForMetrics(summaries), nil
}

func roadmapStorageSummariesForMetrics(summaries []storage.ArchiveTaskSummary) []metrics.TaskSummary {
	metricSummaries := make([]metrics.TaskSummary, 0, len(summaries))
	for _, summary := range summaries {
		metricSummaries = append(metricSummaries, metrics.TaskSummary{
			ID:             summary.ID,
			WorkflowState:  summary.WorkflowState,
			CompletedAt:    summary.CompletedAt,
			FirstStartedAt: summary.FirstStartedAt,
			EstimatedDays:  summary.EstimatedDays,
			Effort:         summary.Effort,
			StoryPoints:    summary.StoryPoints,
		})
	}
	return metricSummaries
}

// zoomUnitDays returns the number of days per column for the given zoom label.
func zoomUnitDays(zoom string) float64 {
	switch zoom {
	case "week":
		return 1
	case "quarter":
		return 30
	default: // month
		return 7
	}
}

// renderASCII prints a static text Gantt chart.
func renderASCII(scheduled []planner.ScheduledTask, cp *planner.CriticalPathResult, projects []*models.Project, zoom string, velocity float64) error {
	const barWidth = 40
	unitDays := zoomUnitDays(zoom)
	now := time.Now()
	axisStart := now.AddDate(0, 0, -7)

	// Header.
	fmt.Printf("Roadmap  [zoom: %s]\n", zoom)
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("%-30s  %-*s  %s\n", "Task", barWidth, "Timeline", "Forecast")
	fmt.Println(strings.Repeat("─", 80))

	// Build schedule map.
	schedMap := make(map[string]planner.ScheduledTask, len(scheduled))
	for _, s := range scheduled {
		schedMap[s.Task.ID] = s
	}

	for _, p := range projects {
		for _, t := range p.Tasks {
			if t.IsComplete() {
				continue
			}
			printASCIIRow(t, 0, schedMap, cp, axisStart, unitDays, barWidth, velocity, now)
			for _, child := range t.Children {
				if !child.IsComplete() {
					printASCIIRow(child, 1, schedMap, cp, axisStart, unitDays, barWidth, velocity, now)
				}
			}
		}
	}
	return nil
}

func printASCIIRow(t *models.Task, indent int, schedMap map[string]planner.ScheduledTask, cp *planner.CriticalPathResult, axisStart time.Time, unitDays float64, barWidth int, velocity float64, now time.Time) {
	prefix := strings.Repeat("  ", indent)
	title := prefix + t.Title
	if len(title) > 28 {
		title = title[:27] + "…"
	}

	// Build bar.
	bar := strings.Repeat(" ", barWidth)
	if s, ok := schedMap[t.ID]; ok {
		startCol := int(s.EarliestStart.Sub(axisStart).Hours() / 24 / unitDays)
		endCol := int(s.EarliestEnd.Sub(axisStart).Hours() / 24 / unitDays)
		startCol = max(0, min(startCol, barWidth-1))
		endCol = max(startCol+1, min(endCol, barWidth))

		barRunes := []rune(strings.Repeat(" ", barWidth))
		todayCol := int(now.Sub(axisStart).Hours() / 24 / unitDays)
		for i := startCol; i < endCol && i < barWidth; i++ {
			switch {
			case t.IsComplete():
				barRunes[i] = '▓'
			case i < todayCol:
				barRunes[i] = '▓'
			case i == todayCol:
				barRunes[i] = '█'
			case len(t.DependsOn) > 0:
				barRunes[i] = '─'
			default:
				barRunes[i] = '░'
			}
		}
		bar = string(barRunes)
	}

	// Critical path marker.
	critMarker := " "
	if cp != nil && cp.OnPath[t.ID] {
		critMarker = "*"
	}

	// Forecast.
	forecastStr := ""
	if velocity > 0 && len(t.Children) > 0 {
		fr := planner.ForecastCompletion(t, velocity, now)
		if !fr.ForecastDate.IsZero() {
			forecastStr = fr.ForecastDate.Format("2006-01-02") + " (" + fr.Confidence.String() + ")"
		}
	}

	fmt.Printf("%s%-30s  %s  %s\n", critMarker, title, bar, forecastStr)
}

// renderMermaid outputs a Mermaid gantt block.
func renderMermaid(scheduled []planner.ScheduledTask, cp *planner.CriticalPathResult, projects []*models.Project) error {
	projectName := "Roadmap"
	if len(projects) == 1 {
		projectName = projects[0].Name
	}

	fmt.Println("```mermaid")
	fmt.Println("gantt")
	fmt.Printf("    title %s\n", projectName)
	fmt.Println("    dateFormat YYYY-MM-DD")

	schedMap := make(map[string]planner.ScheduledTask, len(scheduled))
	for _, s := range scheduled {
		schedMap[s.Task.ID] = s
	}

	for _, p := range projects {
		for _, t := range p.Tasks {
			if t.IsComplete() {
				continue
			}
			fmt.Printf("    section %s\n", mermaidSafe(t.Title))
			printMermaidTask(t, schedMap, cp)
			for _, child := range t.Children {
				if !child.IsComplete() {
					printMermaidTask(child, schedMap, cp)
				}
			}
		}
	}

	fmt.Println("```")
	return nil
}

func printMermaidTask(t *models.Task, schedMap map[string]planner.ScheduledTask, cp *planner.CriticalPathResult) {
	s, ok := schedMap[t.ID]
	if !ok {
		return
	}

	tags := ""
	if cp != nil && cp.OnPath[t.ID] {
		tags = "crit, "
	} else if t.IsInState(models.StateInProgress) {
		tags = "active, "
	}

	durationDays := max(int(math.Ceil(s.EarliestEnd.Sub(s.EarliestStart).Hours()/24)), 1)

	fmt.Printf("    %s    :%s%s, %sd\n",
		mermaidSafe(t.Title),
		tags,
		s.EarliestStart.Format("2006-01-02"),
		fmt.Sprint(durationDays),
	)
}

func mermaidSafe(s string) string {
	return strings.NewReplacer(":", "", ",", "", ";", "").Replace(s)
}

// jsonOutput is the structure for --format json.
type jsonOutput struct {
	GeneratedAt  string           `json:"generated_at"`
	Velocity     float64          `json:"velocity_days_per_day"`
	CriticalPath jsonCriticalPath `json:"critical_path"`
	Schedule     []jsonTask       `json:"schedule"`
}

type jsonCriticalPath struct {
	LengthDays float64  `json:"length_days"`
	TaskIDs    []string `json:"task_ids"`
}

type jsonTask struct {
	ID            string  `json:"id"`
	ShortID       int     `json:"short_id"`
	Title         string  `json:"title"`
	ProjectName   string  `json:"project"`
	IsCritical    bool    `json:"is_critical"`
	Slack         float64 `json:"slack_days"`
	EarliestStart string  `json:"earliest_start,omitempty"`
	EarliestEnd   string  `json:"earliest_end,omitempty"`
}

func renderJSON(scheduled []planner.ScheduledTask, cp *planner.CriticalPathResult, projects []*models.Project, velocity float64) error {
	// Build project lookup by task ID.
	taskProject := make(map[string]string)
	for _, p := range projects {
		for _, t := range p.Tasks {
			taskProject[t.ID] = p.Name
		}
	}

	out := jsonOutput{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Velocity:    velocity,
	}

	if cp != nil {
		out.CriticalPath.LengthDays = cp.Length
		for _, t := range cp.Path {
			out.CriticalPath.TaskIDs = append(out.CriticalPath.TaskIDs, t.ID)
		}
	}

	for _, s := range scheduled {
		jt := jsonTask{
			ID:          s.Task.ID,
			ShortID:     s.Task.ShortID,
			Title:       s.Task.Title,
			ProjectName: taskProject[s.Task.ID],
			IsCritical:  cp != nil && cp.OnPath[s.Task.ID],
			Slack:       s.Slack,
		}
		if !s.EarliestStart.IsZero() {
			jt.EarliestStart = s.EarliestStart.Format("2006-01-02")
		}
		if !s.EarliestEnd.IsZero() {
			jt.EarliestEnd = s.EarliestEnd.Format("2006-01-02")
		}
		out.Schedule = append(out.Schedule, jt)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
