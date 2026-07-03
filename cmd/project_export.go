package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var projectExportFile string

var projectExportCmd = &cobra.Command{
	Use:   "export [slug-or-id]",
	Short: "Export a project to clean Markdown",
	Long: `Export a project to a human-readable Markdown file in the exports/ folder of your tt data directory.

If no project is specified, exports the current project.
Use --file to write to an arbitrary path instead.

EXAMPLES

  tt project export
  tt project export my-project
  tt project export my-project --file /tmp/my-project.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectExport,
}

func init() {
	projectCmd.AddCommand(projectExportCmd)
	projectExportCmd.Flags().StringVar(&projectExportFile, "file", "", "write to this path instead of the default exports/<slug>.md")
}

func runProjectExport(cmd *cobra.Command, args []string) error {
	project, _, manager, err := LoadConfigAndProjectOrCurrent(args)
	if err != nil {
		return err
	}

	dest := projectExportFile
	if dest == "" {
		dest = manager.ExportPath(project.Slug)
		if err := os.MkdirAll(manager.ExportsDir(), 0700); err != nil {
			return fmt.Errorf("create exports dir: %w", err)
		}
	}

	destPath, err := ExportProjectMarkdown(project, dest)
	if err != nil {
		return err
	}

	fmt.Printf("Exported %q to %s\n", project.Name, destPath)
	return nil
}

// ExportProjectMarkdown writes the project to destPath (which must be non-empty;
// callers resolve default export locations through config.Manager.ExportPath)
// and returns the final absolute path.
// LoadProject is called on p to ensure tasks are populated before exporting.
func ExportProjectMarkdown(p *models.Project, destPath string) (string, error) {
	return storage.ExportProjectMarkdown(p, destPath)
}
