package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// mcpTraceCall is one completed tools/call record from the MCP server's JSONL
// trace. The field names are the stable contract shared with
// internal/agent/mcp; see --trace-file in `godot-cli mcp`.
type mcpTraceCall struct {
	Tool       string          `json:"tool"`
	Arguments  json.RawMessage `json:"arguments"`
	IsError    bool            `json:"is_error"`
	DurationMS float64         `json:"duration_ms"`
	SessionID  string          `json:"session_id,omitempty"`
	Timestamp  string          `json:"timestamp,omitempty"`
}

// UnmarshalJSON keeps the reader tolerant of timestamp encodings. The writer
// currently emits RFC3339Nano strings, but a numeric timestamp must not make an
// otherwise valid trace unreadable.
func (call *mcpTraceCall) UnmarshalJSON(data []byte) error {
	var wire struct {
		Tool       string          `json:"tool"`
		Arguments  json.RawMessage `json:"arguments"`
		IsError    bool            `json:"is_error"`
		DurationMS json.RawMessage `json:"duration_ms"`
		SessionID  string          `json:"session_id"`
		Timestamp  json.RawMessage `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Tool == "" {
		return errors.New("trace record has no tool name")
	}

	call.Tool = wire.Tool
	call.Arguments = wire.Arguments
	call.IsError = wire.IsError
	call.SessionID = wire.SessionID

	if len(wire.DurationMS) > 0 && string(wire.DurationMS) != "null" {
		var duration float64
		if err := json.Unmarshal(wire.DurationMS, &duration); err != nil {
			return fmt.Errorf("trace record duration_ms: %w", err)
		}
		call.DurationMS = duration
	}
	if len(wire.Timestamp) > 0 && string(wire.Timestamp) != "null" {
		if wire.Timestamp[0] == '"' {
			if err := json.Unmarshal(wire.Timestamp, &call.Timestamp); err != nil {
				return fmt.Errorf("trace record timestamp: %w", err)
			}
		} else {
			call.Timestamp = string(wire.Timestamp)
		}
	}
	return nil
}

// mcpTraceStats is the usage score derived from actual MCP tool calls. It is
// deliberately independent of the model-visible tool list: Code Mode wraps
// calls in its execute tool, so session messages alone undercount MCP usage.
type mcpTraceStats struct {
	Calls                    int            `json:"calls"`
	Errors                   int            `json:"errors"`
	DurationMS               int64          `json:"durationMs"`
	Tools                    map[string]int `json:"tools,omitempty"`
	InvalidLines             int            `json:"invalidLines,omitempty"`
	BatchCalls               int            `json:"batchCalls"`
	RuntimeCalls             int            `json:"runtimeCalls"`
	RuntimeFeedbackSucceeded bool           `json:"runtimeFeedbackSucceeded"`
}

// readMCPTrace reads trace records written at or after offset. The offset is
// captured after MCP preflight and before the model starts, so preflight calls
// never count as agent usage.
func readMCPTrace(path string, offset int64) ([]mcpTraceCall, int, error) {
	if path == "" {
		return nil, 0, errors.New("MCP trace path is empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("opening MCP trace %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	if offset < 0 {
		offset = 0
	}
	info, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("reading MCP trace %s: %w", path, err)
	}
	if offset > info.Size() {
		return nil, 0, fmt.Errorf(
			"MCP trace %s is smaller than the recorded offset (%d < %d)",
			path,
			info.Size(),
			offset,
		)
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("seeking MCP trace %s: %w", path, err)
	}

	return parseMCPTrace(file)
}

// traceFileSize returns the current trace size, or zero when the file does not
// exist yet (the MCP client may not have connected before preflight).
func traceFileSize(path string) (int64, error) {
	if path == "" {
		return 0, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading MCP trace size: %w", err)
	}
	return info.Size(), nil
}

func parseMCPTrace(reader io.Reader) ([]mcpTraceCall, int, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var calls []mcpTraceCall
	invalid := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var call mcpTraceCall
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			invalid++
			continue
		}
		calls = append(calls, call)
	}
	if err := scanner.Err(); err != nil {
		return nil, invalid, fmt.Errorf("reading MCP trace: %w", err)
	}
	return calls, invalid, nil
}

// summarizeMCPTrace counts calls for this benchmark run. Records tagged with a
// different OpenCode session are ignored; an untagged record is kept because
// older or synthetic transports may not send session metadata.
func summarizeMCPTrace(calls []mcpTraceCall, sessionID string) mcpTraceStats {
	stats := mcpTraceStats{Tools: map[string]int{}}

	for _, call := range calls {
		if sessionID != "" && call.SessionID != "" && call.SessionID != sessionID {
			continue
		}
		stats.Calls++
		if call.IsError {
			stats.Errors++
		}
		stats.DurationMS += int64(call.DurationMS)
		stats.Tools[call.Tool]++
		switch call.Tool {
		case "scene_apply", "resource_apply", "project_settings_apply", "input_map_apply":
			stats.BatchCalls++
		case "runtime_start", "runtime_status", "runtime_capture", "runtime_stop":
			stats.RuntimeCalls++
			if !call.IsError && (call.Tool == "runtime_status" || call.Tool == "runtime_capture") {
				stats.RuntimeFeedbackSucceeded = true
			}
		}
	}
	if len(stats.Tools) == 0 {
		stats.Tools = nil
	}
	return stats
}

// toolUsageCount is the comparable "tools used" metric. MCP runs are scored
// from the server trace because Code Mode hides MCP calls inside execute.
func toolUsageCount(result benchmarkResult) int {
	if result.Approach == approachMCP && result.MCP != nil {
		return result.MCP.Calls
	}
	return len(result.Tools)
}

var (
	patchFileMarker = regexp.MustCompile(`(?m)^\*\*\* (?:Update|Add|Delete) File:\s*(.+?)\s*$`)
	patchDiffMarker = regexp.MustCompile(`(?m)^(?:\+\+\+|---) (?:[ab]/)?(.+?)(?:\t.*)?$`)
	codePathField   = regexp.MustCompile(`(?s)["']?(?:path|filePath|file)["']?\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)
)

// directMutationViolations reports project mutations made with file tools.
// Reads (including reads of serialized .tscn/.tres files) and edits to source
// files are legitimate under the MCP-first workflow; only semantic Godot files
// count as a violation on a semantic task.
func directMutationViolations(
	sessionTools []toolCall,
	executedTools []toolCall,
	observationErr error,
	projectRoot string,
	semantic bool,
) []string {
	if !semantic {
		return nil
	}

	var violations []string
	seen := map[string]bool{}
	report := func(reason string) {
		if seen[reason] {
			return
		}
		seen[reason] = true
		violations = append(violations, reason)
	}

	hasExecute := containsToolCall(sessionTools, "execute")
	if reason := unclassifiableExecuteReason(hasExecute, executedTools, observationErr); reason != "" {
		report(reason)
		return violations
	}

	tools := executedTools
	if len(tools) == 0 && !hasExecute {
		tools = sessionTools
	}
	for _, tool := range tools {
		name := strings.ToLower(tool.Name)

		switch name {
		case "edit", "write", "multiedit", "patch", "apply_patch":
			paths := toolInputPaths(tool.Input)
			hasSerializedText := patchMentionsSerializedFile(tool.Input)

			for _, path := range paths {
				relative, inside := projectRelativePath(projectRoot, path)
				if !inside || !isSerializedGodotFile(relative) {
					continue
				}
				report(fmt.Sprintf(
					"%s edited serialized Godot file directly: %s",
					name,
					relative,
				))
			}
			if len(paths) == 0 && hasSerializedText {
				report(fmt.Sprintf(
					"%s appears to modify a serialized Godot file directly",
					name,
				))
			}
		}
	}
	return violations
}

func containsToolCall(tools []toolCall, name string) bool {
	for _, tool := range tools {
		if strings.EqualFold(tool.Name, name) {
			return true
		}
	}
	return false
}

func unclassifiableExecuteReason(
	hasExecute bool,
	executedTools []toolCall,
	observationErr error,
) string {
	if !hasExecute {
		return ""
	}
	if observationErr != nil {
		return "Code Mode execute could not be classified from runtime tool observations; treating it as an unsafe possible serialized-file mutation"
	}
	if !containsToolCall(executedTools, "execute") {
		return "Code Mode execute was present in session history but absent from runtime tool observations; treating it as an unsafe possible serialized-file mutation"
	}
	return ""
}

func toolInputPaths(input json.RawMessage) []string {
	if len(input) == 0 {
		return nil
	}

	var fields struct {
		Path     string          `json:"path"`
		FilePath string          `json:"filePath"`
		File     string          `json:"file"`
		Files    json.RawMessage `json:"files"`
		Patch    string          `json:"patch"`
		Diff     string          `json:"diff"`
		Code     string          `json:"code"`
	}
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil
	}

	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	add(fields.Path)
	add(fields.FilePath)
	add(fields.File)

	if len(fields.Files) > 0 {
		var names []string
		if json.Unmarshal(fields.Files, &names) == nil {
			for _, name := range names {
				add(name)
			}
		}
	}

	text := fields.Patch
	if text == "" {
		text = fields.Diff
	}
	for _, match := range patchFileMarker.FindAllStringSubmatch(text, -1) {
		add(match[1])
	}
	for _, match := range patchDiffMarker.FindAllStringSubmatch(text, -1) {
		add(match[1])
	}
	for _, match := range codePathField.FindAllStringSubmatch(fields.Code, -1) {
		add(match[1])
	}
	return paths
}

func patchMentionsSerializedFile(input json.RawMessage) bool {
	text := string(input)
	var fields struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(input, &fields) == nil && fields.Code != "" {
		text = fields.Code
	}
	lower := strings.ToLower(text)

	for _, suffix := range []string{
		".tscn", ".tres", ".res", ".scn", "project.godot",
	} {
		if strings.Contains(lower, suffix) {
			return true
		}
	}
	return false
}

func isSerializedGodotFile(relative string) bool {
	base := strings.ToLower(filepath.Base(relative))
	if base == "project.godot" {
		return true
	}

	switch strings.ToLower(filepath.Ext(base)) {
	case ".tscn", ".tres", ".res", ".scn", ".godot":
		return true
	default:
		return false
	}
}
