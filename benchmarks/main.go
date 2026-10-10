package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const approachBoth = "both"

func main() {
	var options runOptions
	var smokeAPI bool
	var runs int
	var parallel int

	flag.StringVar(&options.Model, "model", "", "OpenCode model in provider/model#variant format")
	flag.StringVar(&options.Agent, "agent", "build", "OpenCode agent")
	flag.StringVar(&options.Approach, "approach", approachCLI, "Benchmark approach: godot-cli, direct-files, or both")
	flag.StringVar(&options.Task, "task", "move_player", "Benchmark task")
	flag.StringVar(&options.GodotPath, "godot", "", "Godot executable (defaults to GODOT_BIN or PATH)")
	flag.StringVar(&options.OpenCodePath, "opencode", "opencode", "OpenCode executable")
	flag.StringVar(&options.ResultsDir, "results", "benchmarks/results", "Result directory")
	flag.DurationVar(&options.Timeout, "timeout", 5*time.Minute, "Maximum agent run duration")
	flag.BoolVar(&options.KeepProject, "keep-project", false, "Keep the temporary project after the run")
	flag.BoolVar(&smokeAPI, "smoke-api", false, "Validate the OpenCode session API without invoking a model")
	flag.IntVar(&runs, "runs", 1, "Number of benchmark repetitions per approach")
	flag.IntVar(&parallel, "parallel", 1, "Maximum concurrent runs; wall-clock timings are unreliable above 1")
	flag.Parse()

	if options.Model == "" && !smokeAPI {
		fmt.Fprintln(os.Stderr, "--model is required")
		os.Exit(2)
	}
	if runs < 1 {
		fmt.Fprintln(os.Stderr, "--runs must be at least 1")
		os.Exit(2)
	}
	if parallel < 1 {
		fmt.Fprintln(os.Stderr, "--parallel must be at least 1")
		os.Exit(2)
	}
	var approaches []string

	switch options.Approach {
	case approachCLI, approachDirect:
		approaches = []string{options.Approach}
	case approachBoth:
		approaches = []string{approachCLI, approachDirect}
	default:
		fmt.Fprintln(
			os.Stderr,
			"--approach must be godot-cli, direct-files, or both",
		)
		os.Exit(2)
	}
	if smokeAPI {
		message, err := smokeOpenCodeAPI(options.OpenCodePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println(message)
		fmt.Println("OpenCode API smoke test passed")
		return
	}
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if !filepath.IsAbs(options.ResultsDir) {
		options.ResultsDir = filepath.Join(repositoryRoot, options.ResultsDir)
	}
	if runs == 1 && len(approaches) == 1 {
		resultPath, err := runBenchmark(repositoryRoot, options)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println("Benchmark result:", resultPath)
		return
	}
	if err := runSuite(repositoryRoot, options, approaches, runs, parallel); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
