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
}

type rankedTask struct {
	task  *models.Task
	score float64
}

func runScoreRank(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	profile := cfg.GetProjectScoringProfile(project.ID)
	if profile == nil {
		return fmt.Errorf("no scoring profile assigned to project %q", project.Name)
	}

	includeAll, _ := cmd.Flags().GetBool("all")
	limit, _ := cmd.Flags().GetInt("limit")

	var ranked []rankedTask
	for _, task := range project.Tasks {
		if !includeAll && task.Status.IsComplete() {
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

	if len(ranked) == 0 {
		fmt.Println("No tasks to rank.")
		return nil
	}

	fmt.Printf("Tasks ranked by %s score (project: %s):\n\n", profile.Name, project.Name)
	for i, r := range ranked {
		pct := scoring.ScorePercent(r.score, profile)
		fmt.Printf("  %2d. [S:%d] #%-4d %-6s %s\n",
			i+1,
			int(pct),
			r.task.ShortID,
			r.task.Type.Icon()+"/"+string(r.task.Status)[:3],
			r.task.Title,
		)
	}
	return nil
}
