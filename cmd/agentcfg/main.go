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
		exampleOut string
		output     string
		inPlace    bool
	)

	root := &cobra.Command{
		Use:           "agentcfg",
		Short:         "Compile agentcfg.yaml into native coding-agent configs",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", buildinfo.Version, buildinfo.Commit, buildinfo.Date),
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "agentcfg.yaml", "path to agentcfg.yaml")
	root.PersistentFlags().StringVarP(&to, "to", "t", "", `comma-separated target ids, or "all"`)
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "log at info level to stderr")
	root.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "log at debug level to stderr")

	newRequest := func() app.Request {
		level := logging.LevelWarn
		if verbose {
			level = logging.LevelInfo
		}
		if debug {
			level = logging.LevelDebug
		}
		return app.Request{
			ConfigPath: configPath,
			To:         to,
			Stdout:     os.Stdout,
			Stderr:     os.Stderr,
			Log:        logging.New(level),
			Output:     output,
			InPlace:    inPlace,
		}
	}

	root.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate the IR and selected targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.Validate(newRequest())
		},
	})
	gen := &cobra.Command{
		Use:   "gen",
		Short: "Generate native configs to stdout or merge into local files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.Generate(newRequest())
		},
	}
	gen.Flags().StringVarP(&output, "output", "o", "", "merge into a file (single artifact) or directory (multiple artifacts/targets)")
	gen.Flags().BoolVar(&inPlace, "in-place", false, "merge into native configuration paths")
	gen.MarkFlagsMutuallyExclusive("output", "in-place")
	root.AddCommand(gen)
	genExample := &cobra.Command{
		Use:   "gen-example",
		Short: "Print the example agentcfg.yaml to stdout (or write it with -o)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.GenExample(newRequest(), exampleOut)
		},
	}
	genExample.Flags().StringVarP(&exampleOut, "output", "o", "", "write the example to this file instead of stdout")
	root.AddCommand(genExample)

	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
