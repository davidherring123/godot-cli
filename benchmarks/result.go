package main

import "encoding/json"

type benchmarkResult struct {
	Task          string               `json:"task"`
	Approach      string               `json:"approach"`
	Model         string               `json:"model"`
	Agent         string               `json:"agent"`
	TimeoutMS     int64                `json:"timeoutMs"`
	MCPProfile    string               `json:"mcpProfile,omitempty"`
	SessionID     string               `json:"sessionId"`
	Success       bool                 `json:"success"`
	Failure       string               `json:"failure,omitempty"`
	SetupMS       int64                `json:"setupMs"`
	SetupFailure  string               `json:"setupFailure,omitempty"`
	AgentFailure  string               `json:"agentFailure,omitempty"`
	TaskFailure   string               `json:"taskFailure,omitempty"`
	AgentMS       int64                `json:"agentMs"`
	CostUSD       *float64             `json:"costUsd"`
	Tokens        *resultTokens        `json:"tokens"`
	Context       contextMetrics       `json:"context"`
	Tools         []toolCall           `json:"tools"`
	ExecutedTools []toolCall           `json:"executedTools,omitempty"`
	MCP           *mcpTraceStats       `json:"mcp,omitempty"`
	Artifacts     *runArtifacts        `json:"artifacts,omitempty"`
	Verification  *verificationSummary `json:"verification,omitempty"`
}

type verificationSummary struct {
	Loaded bool         `json:"loaded"`
	Passed int          `json:"passed"`
	Total  int          `json:"total"`
	Score  float64      `json:"score"`
	Checks []probeCheck `json:"checks,omitempty"`
}

func summarizeVerification(probe probeResult) *verificationSummary {
	summary := &verificationSummary{Loaded: probe.Loaded, Total: len(probe.Checks), Checks: probe.Checks}
	for _, check := range probe.Checks {
		if check.Passed {
			summary.Passed++
		}
	}
	if summary.Total > 0 {
		summary.Score = float64(summary.Passed) / float64(summary.Total)
	}
	return summary
}

type tokenUsage struct {
	Input     *float64        `json:"input"`
	Output    *float64        `json:"output"`
	Reasoning *float64        `json:"reasoning"`
	Cache     tokenCacheUsage `json:"cache"`
}

type tokenCacheUsage struct {
	Read  *float64 `json:"read"`
	Write *float64 `json:"write"`
}

type resultTokens struct {
	Input        *float64 `json:"input"`
	Output       *float64 `json:"output"`
	Reasoning    *float64 `json:"reasoning"`
	CacheRead    *float64 `json:"cacheRead"`
	CacheWrite   *float64 `json:"cacheWrite"`
	PromptTokens *float64 `json:"promptTokens"`
	CacheHitRate *float64 `json:"cacheHitRate"`
}

type contextMetrics struct {
	Messages        int `json:"messages"`
	Bytes           int `json:"bytes"`
	ToolOutputBytes int `json:"toolOutputBytes"`
}

type toolCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// summarizeMessages extracts the tool calls from an OpenCode session's message
// history for the result record.
func summarizeMessages(messages []sessionMessage) []toolCall {
	var tools []toolCall

	for _, message := range messages {
		if message.Type != "assistant" {
			continue
		}
		for _, content := range message.Content {
			var kind struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(content, &kind) != nil {
				continue
			}
			if kind.Type != "tool" {
				continue
			}
			var part toolPart
			if json.Unmarshal(content, &part) == nil {
				tools = append(tools, toolCall{
					Name:  part.Name,
					Input: part.State.Input,
				})
			}
		}
	}
	return tools
}

// summarizeTokens normalizes OpenCode token accounting, deriving prompt size
// and cache hit rate from the input and cache-read counts.
func summarizeTokens(tokens *tokenUsage) *resultTokens {
	if tokens == nil {
		return nil
	}
	summary := &resultTokens{
		Input:      tokens.Input,
		Output:     tokens.Output,
		Reasoning:  tokens.Reasoning,
		CacheRead:  tokens.Cache.Read,
		CacheWrite: tokens.Cache.Write,
	}
	if tokens.Input != nil && tokens.Cache.Read != nil {
		prompt := *tokens.Input + *tokens.Cache.Read
		summary.PromptTokens = &prompt

		if prompt > 0 {
			rate := *tokens.Cache.Read / prompt
			summary.CacheHitRate = &rate
		}
	}
	return summary
}

// toolOutputBytes measures the serialized tool output carried by one context
// message.
func toolOutputBytes(message json.RawMessage) int {
	var parsed sessionMessage

	if json.Unmarshal(message, &parsed) != nil {
		return 0
	}
	bytes := 0

	for _, content := range parsed.Content {
		var part toolPart

		if json.Unmarshal(content, &part) == nil && part.Type == "tool" {
			bytes += len(part.State.Content)
		}
	}
	return bytes
}
