package main

import "encoding/json"

type benchmarkResult struct {
	Task      string         `json:"task"`
	Approach  string         `json:"approach"`
	Model     string         `json:"model"`
	SessionID string         `json:"sessionId"`
	Success   bool           `json:"success"`
	Failure   string         `json:"failure,omitempty"`
	AgentMS   int64          `json:"agentMs"`
	CostUSD   *float64       `json:"costUsd"`
	Tokens    *resultTokens  `json:"tokens"`
	Context   contextMetrics `json:"context"`
	Tools     []toolCall     `json:"tools"`
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
