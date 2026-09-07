// Command agentcfg compiles agentcfg.yaml into native agent configs.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agentcfg/internal/app"
	"agentcfg/internal/buildinfo"
	"agentcfg/internal/logging"
	_ "agentcfg/internal/target/all"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var (
		configPath string
		to         string
		verbose    bool
		debug      bool
	)

	root := &cobra.Command{
		Use:           "agentcfg",
		Short:         "Compile agentcfg.yaml into native coding-agent configs",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", buildinfo.Version, buildinfo.Commit, buildinfo.Date),
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "agentcfg.yaml", "path to agentcfg.yaml")
	root.PersistentFlags().StringVar(&to, "to", "", `comma-separated target ids, or "all"`)
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "log at info level to stderr")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "log at debug level to stderr")

	level := logging.LevelWarn
	if verbose {
		level = logging.LevelInfo
	}
	if debug {
		level = logging.LevelDebug
	}

	newRequest := func() app.Request {
		return app.Request{
			ConfigPath: configPath,
			To:         to,
			Stdout:     os.Stdout,
			Stderr:     os.Stderr,
			Log:        logging.New(level),
		}
	}

	root.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate the IR and selected targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.Validate(newRequest())
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "gen",
		Short: "Generate native configs to stdout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.Generate(newRequest())
		},
	})

	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
