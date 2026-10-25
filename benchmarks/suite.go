package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type suiteRun struct {
	Approach string
	Index    int
}

type suiteOutcome struct {
	Approach string
	Index    int
	Path     string
	Err      error
	Result   *benchmarkResult
}

type suiteSummary struct {
	Task            string          `json:"task"`
	Model           string          `json:"model"`
	RunsPerApproach int             `json:"runsPerApproach"`
	Parallel        int             `json:"parallel"`
	StartedAt       time.Time       `json:"startedAt"`
	Approaches      []approachStats `json:"approaches"`
}

type approachStats struct {
	Approach           string  `json:"approach"`
	Runs               int     `json:"runs"`
	Completed          int     `json:"completed"`
	Successes          int     `json:"successes"`
	SetupFailures      int     `json:"setupFailures"`
	SuccessRate        float64 `json:"successRate"`
	MedianSetupMS      float64 `json:"medianSetupMs"`
	MedianAgentMS      float64 `json:"medianAgentMs"`
	MedianTools        float64 `json:"medianTools"`
	MedianMCPCalls     float64 `json:"medianMcpCalls"`
	MedianMCPErrors    float64 `json:"medianMcpErrors"`
	MedianInputTokens  float64 `json:"medianInputTokens"`
	MedianOutputTokens float64 `json:"medianOutputTokens"`
	MedianPromptTokens float64 `json:"medianPromptTokens"`
	MedianCacheRead    float64 `json:"medianCacheRead"`
	MedianCacheHitRate float64 `json:"medianCacheHitRate"`
	MedianCostUSD      float64 `json:"medianCostUsd"`
	MedianContextBytes float64 `json:"medianContextBytes"`
}

func runSuite(
	repositoryRoot string,
	options runOptions,
	approaches []string,
	runs int,
	parallel int,
) error {
	startedAt := time.Now()

	if parallel < 1 {
		parallel = 1
	}
	var sequence []suiteRun

	for index := 1; index <= runs; index++ {
		order := approaches
		if index%2 == 0 && len(approaches) > 1 {
			order = reversed(approaches)
		}
		for _, approach := range order {
			sequence = append(sequence, suiteRun{Approach: approach, Index: index})
		}
	}
	outcomes := make([]suiteOutcome, len(sequence))
	semaphore := make(chan struct{}, parallel)
	var wait sync.WaitGroup

	for position, item := range sequence {
		wait.Add(1)

		go func(position int, item suiteRun) {
			defer wait.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			runOptions := options
			runOptions.Approach = item.Approach
			runOptions.Label = fmt.Sprintf("%s#%d", item.Approach, item.Index)

			outcome := suiteOutcome{Approach: item.Approach, Index: item.Index}

			path, err := runBenchmark(repositoryRoot, runOptions)
			outcome.Path = path
			outcome.Err = err

			// A setup failure still writes a result. Read it so setup duration
			// and failure counts are represented in the summary.
			if path != "" {
				if result, readErr := readResult(path); readErr == nil {
					outcome.Result = &result
				} else if err == nil {
					outcome.Err = readErr
				}
			}
			outcomes[position] = outcome
		}(position, item)
	}
	wait.Wait()

	for _, outcome := range outcomes {
		if outcome.Err != nil {
			fmt.Fprintf(
				os.Stderr,
				"run %s#%d failed: %v\n",
				outcome.Approach,
				outcome.Index,
				outcome.Err,
			)
		}
	}
	task, err := loadTask(repositoryRoot, options.Task)
	if err != nil {
		return err
	}
	summary := suiteSummary{
		Task:            task.ID,
		Model:           options.Model,
		RunsPerApproach: runs,
		Parallel:        parallel,
		StartedAt:       startedAt,
	}
	for _, approach := range approaches {
		summary.Approaches = append(
			summary.Approaches,
			summarizeApproach(approach, outcomes),
		)
	}
	printSuiteSummary(summary)

	stamp := startedAt.UTC().Format("20060102T150405.000000000Z")
	summaryPath := filepath.Join(
		options.ResultsDir,
		strings.Join([]string{
			stamp,
			"summary",
			task.ID,
			sanitizePath(options.Model),
		}, "-")+".json",
	)

	if err := writeJSON(summaryPath, summary); err != nil {
		return err
	}
	fmt.Println("Benchmark summary:", summaryPath)

	return nil
}

func summarizeApproach(
	approach string,
	outcomes []suiteOutcome,
) approachStats {
	stats := approachStats{Approach: approach}

	var setupMS []float64
	var agentMS []float64
	var tools []float64
	var mcpCalls []float64
	var mcpErrors []float64
	var inputTokens []float64
	var outputTokens []float64
	var promptTokens []float64
	var cacheRead []float64
	var cacheHitRate []float64
	var costs []float64
	var contextBytes []float64

	for _, outcome := range outcomes {
		if outcome.Approach != approach {
			continue
		}
		stats.Runs++

		if outcome.Result == nil {
			continue
		}
		stats.Completed++

		if outcome.Result.SetupFailure != "" {
			stats.SetupFailures++
		}
		setupMS = append(setupMS, float64(outcome.Result.SetupMS))

		if !outcome.Result.Success {
			continue
		}
		stats.Successes++

		agentMS = append(agentMS, float64(outcome.Result.AgentMS))
		tools = append(tools, float64(toolUsageCount(*outcome.Result)))
		contextBytes = append(contextBytes, float64(outcome.Result.Context.Bytes))

		if mcp := outcome.Result.MCP; mcp != nil {
			mcpCalls = append(mcpCalls, float64(mcp.Calls))
			mcpErrors = append(mcpErrors, float64(mcp.Errors))
		}

		if tokens := outcome.Result.Tokens; tokens != nil {
			inputTokens = appendFloat(inputTokens, tokens.Input)
			outputTokens = appendFloat(outputTokens, tokens.Output)
			promptTokens = appendFloat(promptTokens, tokens.PromptTokens)
			cacheRead = appendFloat(cacheRead, tokens.CacheRead)
			cacheHitRate = appendFloat(cacheHitRate, tokens.CacheHitRate)
		}
		costs = appendFloat(costs, outcome.Result.CostUSD)
	}
	if stats.Runs > 0 {
		stats.SuccessRate = float64(stats.Successes) / float64(stats.Runs)
	}
	stats.MedianSetupMS = median(setupMS)
	stats.MedianAgentMS = median(agentMS)
	stats.MedianTools = median(tools)
	stats.MedianMCPCalls = median(mcpCalls)
	stats.MedianMCPErrors = median(mcpErrors)
	stats.MedianInputTokens = median(inputTokens)
	stats.MedianOutputTokens = median(outputTokens)
	stats.MedianPromptTokens = median(promptTokens)
	stats.MedianCacheRead = median(cacheRead)
	stats.MedianCacheHitRate = median(cacheHitRate)
	stats.MedianCostUSD = median(costs)
	stats.MedianContextBytes = median(contextBytes)

	return stats
}

func printSuiteSummary(summary suiteSummary) {
	fmt.Printf(
		"\n%s (%d run(s) per approach, parallel %d)\n",
		summary.Task,
		summary.RunsPerApproach,
		summary.Parallel,
	)

	fmt.Printf(
		"%-14s %5s %9s %7s %9s %8s %8s %7s %8s %8s %10s %7s %9s %9s\n",
		"approach",
		"runs",
		"success",
		"setup",
		"setupMs",
		"agentMs",
		"tools",
		"mcp",
		"inTok",
		"outTok",
		"promptTok",
		"cache%",
		"costUsd",
		"contextKB",
	)

	for _, stats := range summary.Approaches {
		fmt.Printf(
			"%-14s %5d %9s %7d %9.0f %8.0f %8.0f %7.0f %8.0f %8.0f %10.0f %6.0f%% %9.6f %9.1f\n",
			stats.Approach,
			stats.Runs,
			fmt.Sprintf("%d/%d", stats.Successes, stats.Runs),
			stats.SetupFailures,
			stats.MedianSetupMS,
			stats.MedianAgentMS,
			stats.MedianTools,
			stats.MedianMCPCalls,
			stats.MedianInputTokens,
			stats.MedianOutputTokens,
			stats.MedianPromptTokens,
			stats.MedianCacheHitRate*100,
			stats.MedianCostUSD,
			stats.MedianContextBytes/1024,
		)
	}
}

func readResult(path string) (benchmarkResult, error) {
	var result benchmarkResult

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}

func appendFloat(values []float64, value *float64) []float64 {
	if value == nil {
		return values
	}
	return append(values, *value)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func reversed(values []string) []string {
	result := make([]string, 0, len(values))

	for index := len(values) - 1; index >= 0; index-- {
		result = append(result, values[index])
	}
	return result
}
