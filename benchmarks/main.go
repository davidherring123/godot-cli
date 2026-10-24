package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	approachBoth          = "both"
	defaultBenchmarkModel = "deepseek/deepseek-flash"
)

func main() {
	var options runOptions
	var smoke bool
	var runs int
	var parallel int

	flag.StringVar(&options.Model, "model", defaultBenchmarkModel, "OpenCode model in provider/model#variant format")
	flag.StringVar(&options.Agent, "agent", "build", "OpenCode agent")
	flag.StringVar(
		&options.Approach,
		"approach",
		approachMCP,
		"Benchmark approach: mcp, direct-files, or both",
	)
	flag.StringVar(&options.Task, "task", "playable_platformer", "Benchmark task")
	flag.StringVar(&options.MCPProfile, "mcp-profile", "", "Controlled MCP profile: legacy-mcp, improved-mcp, normal-mcp, or hybrid-mcp")
	flag.StringVar(&options.GodotPath, "godot", "", "Godot executable (defaults to GODOT_BIN or PATH)")
	flag.StringVar(&options.OpenCodePath, "opencode", "opencode", "OpenCode executable")
	flag.StringVar(&options.ResultsDir, "results", "benchmarks/results", "Result directory")
	flag.DurationVar(&options.Timeout, "timeout", 5*time.Minute, "Maximum agent run duration")
	flag.BoolVar(&options.KeepProject, "keep-project", false, "Keep the temporary project after the run")
	flag.BoolVar(&options.PreserveProject, "preserve-project", false, "Copy the generated project into the results directory")
	flag.BoolVar(&smoke, "smoke", false, "Validate the full godot MCP benchmark path without invoking a model")
	flag.IntVar(&runs, "runs", 1, "Number of benchmark repetitions per approach")
	flag.IntVar(&parallel, "parallel", 1, "Maximum concurrent runs; wall-clock timings are unreliable above 1")
	flag.Parse()

	if runs < 1 {
		fmt.Fprintln(os.Stderr, "--runs must be at least 1")
		os.Exit(2)
	}
	if parallel < 1 {
		fmt.Fprintln(os.Stderr, "--parallel must be at least 1")
		os.Exit(2)
	}
	approaches, err := resolveApproaches(options.Approach)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if !filepath.IsAbs(options.ResultsDir) {
		options.ResultsDir = filepath.Join(repositoryRoot, options.ResultsDir)
	}
	if smoke {
		message, err := smokeGodotMCP(repositoryRoot, options)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println(message)
		fmt.Println("MCP smoke test passed")
		return
	}
	if runs == 1 && len(approaches) == 1 {
		resultPath, err := runBenchmark(repositoryRoot, options)
		if resultPath != "" {
			fmt.Println("Benchmark result:", resultPath)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}
	if err := runSuite(repositoryRoot, options, approaches, runs, parallel); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
