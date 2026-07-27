package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

var scoreRankCmd = &cobra.Command{
	Use:   "rank",
	Short: "List tasks ranked by composite score",
	Long: `List all incomplete tasks in the current project ranked by their composite score,
from highest to lowest. Only tasks with an active scoring profile are ranked.`,
	RunE: runScoreRank,
}

func init() {
	scoreCmd.AddCommand(scoreRankCmd)

	scoreRankCmd.Flags().IntP("limit", "n", 0, "limit number of results (0 = all)")
	scoreRankCmd.Flags().Bool("all", false, "include completed tasks")
	scoreRankCmd.Flags().Bool("snoozed", false, "include snoozed tasks (hidden by default)")
	scoreRankCmd.Flags().Bool("json", false, "output as JSON")
}

type rankedTask struct {
	task  *models.Task
	score float64
}

func runScoreRank(cmd *cobra.Command, args []string) error {
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	profile := workspace.Runtime.Automation().GetProjectScoringProfile(project.ID)
	if profile == nil {
		return fmt.Errorf("no scoring profile assigned to project %q", project.Name)
	}

	includeAll, _ := cmd.Flags().GetBool("all")
	includeSnoozed, _ := cmd.Flags().GetBool("snoozed")
	limit, _ := cmd.Flags().GetInt("limit")

	var ranked []rankedTask
	for _, task := range project.Tasks {
		if !includeAll && task.IsTerminalState() {
			continue
		}
		if !includeSnoozed && task.IsSnoozed() {
			continue
		}
		score := scoring.ComputeScore(task, profile)
		ranked = append(ranked, rankedTask{task: task, score: score})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if limit > 0 && limit < len(ranked) {
		ranked = ranked[:limit]
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")

	if len(ranked) == 0 {
		if jsonOutput {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No tasks to rank.")
		return nil
	}
	if jsonOutput {
		type rankedJSON struct {
			Rank         int     `json:"rank"`
			ID           string  `json:"id"`
			ShortID      int     `json:"short_id"`
			Type         string  `json:"type"`
			State        string  `json:"state"`
			Title        string  `json:"title"`
			Priority     int     `json:"priority"`
			Score        float64 `json:"score"`
			ScorePercent int     `json:"score_percent"`
		}

		items := make([]rankedJSON, len(ranked))
		for i, r := range ranked {
			pct := scoring.ScorePercent(r.score, profile)
			items[i] = rankedJSON{
				Rank:         i + 1,
				ID:           r.task.ID,
				ShortID:      r.task.ShortID,
				Type:         string(r.task.Type),
				State:        string(r.task.State()),
				Title:        r.task.Title,
				Priority:     r.task.Priority,
				Score:        r.score,
				ScorePercent: int(pct),
			}
		}

		return printJSON(items)
	}

	fmt.Printf("Tasks ranked by %s score (project: %s):\n\n", profile.Name, project.Name)
	for i, r := range ranked {
		pct := scoring.ScorePercent(r.score, profile)
		statusStr := string(r.task.State())
		if len(statusStr) > 3 {
			statusStr = statusStr[:3]
		}
		fmt.Printf("  %2d. [S:%d] #%-4d %-6s %s\n",
			i+1,
			int(pct),
			r.task.ShortID,
			r.task.Type.Icon()+"/"+statusStr,
			r.task.Title,
		)
	}
	return nil
}
