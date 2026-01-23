package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui"
)

var (
	configPath string
	verbose    bool

	// Build information, populated via ldflags
	Version   = "dev"
	GitCommit = ""
	BuildTime = ""
)

var rootCmd = &cobra.Command{
	Use:   "tt",
	Short: "A terminal-based task and project manager",
	Long: `tt is a CLI-first task and project manager designed for developers.

It supports hierarchical tasks (epics, stories, tasks, bugs, ideas),
markdown-based storage, and integrates with tools like fzf and glow.

Examples:
  tt                           # Launch interactive TUI
  tt dash                      # Show dashboard summary
  tt project new "My Project"  # Create a new project
  tt task add "Fix login bug"  # Add a task to current project
  tt task list --type epic     # List all epics`,
	RunE: runRoot,
}

func Execute() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default: ~/.tt.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.Flags().Bool("version", false, "show version information")

	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if configPath != "" {
		viper.Set("config", configPath)
	}
}

func GetConfigPath() string {
	return viper.GetString("config")
}

// IsVerbose returns true if verbose mode is enabled via --verbose or -v flag.
func IsVerbose() bool {
	return viper.GetBool("verbose")
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		printVersion()
		return nil
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return tui.RunApp(cfg, configManager)
}

func printVersion() {
	if GitCommit == "" && BuildTime == "" {
		fmt.Printf("tt version %s\n", Version)
		return
	}
	fmt.Printf("tt version %s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
}
