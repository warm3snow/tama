package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/warm3snow/tama/internal/agent"
	"github.com/warm3snow/tama/internal/config"
)

// 版本信息，将在构建时通过 -ldflags 设置
var (
	// Version 是应用程序的版本号
	Version = "0.1.0"
	// BuildTime 是应用程序的构建时间
	BuildTime = "unknown"
	// Commit 是应用程序的 Git commit 哈希
	Commit = "unknown"
)

// rootCmd 表示基础命令
var rootCmd = &cobra.Command{
	Use:     "tama",
	Short:   "Tama is an autonomous AI coding assistant",
	Version: Version,
	Long: `Tama is an autonomous AI coding assistant that acts as a peer programmer.
It can perform multi-step coding tasks by analyzing your codebase, reading files,
proposing edits, and running commands.`,
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// 添加版本命令
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`
  _______                      
 |__   __|                     
    | | __ _ _ __ ___   __ _   
    | |/ _' | '_ ' _ \ / _' |  
    | | (_| | | | | | | (_| |  
    |_|\__,_|_| |_| |_|\__,_|  
                               
 Copilot Agent - Your AI Coding Assistant
 
 Version:    %s
 Build Time: %s
 Commit:     %s
 OS/Arch:    %s/%s
 Go Version: %s
`, Version, BuildTime, Commit, runtime.GOOS, runtime.GOARCH, runtime.Version())
		},
	}

	// 添加启动命令
	var startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the Tama agent",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
				os.Exit(1)
			}

			a := agent.New(cfg)
			if err := a.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting agent: %s\n", err)
				os.Exit(1)
			}
		},
	}

	// 添加执行命令
	var execCmd = &cobra.Command{
		Use:   "exec [task]",
		Short: "Execute a specific task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := args[0]

			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
				os.Exit(1)
			}

			a := agent.New(cfg)
			if err := a.ExecuteTask(task); err != nil {
				fmt.Fprintf(os.Stderr, "Error executing task: %s\n", err)
				os.Exit(1)
			}
		},
	}

	// 将命令添加到根命令
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(execCmd)
}
