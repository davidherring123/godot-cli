package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	mcpBenchmarkServerName = "godot"
	mcpProbeRPCID          = "godot-benchmark-probe"
	mcpProbePluginPath     = ".opencode/plugins/godot-benchmark-probe/index.ts"
	mcpPollInterval        = 100 * time.Millisecond
	mcpPreflightTimeout    = 90 * time.Second
)

// mcpProbePluginSource installs a temporary OpenCode plugin that exposes the
// server's live tool registry over plugin RPC. OpenCode has no public HTTP
// endpoint for invoking a registered tool without a model turn, so this plugin
// is the model-free bridge into the same private server, location, and MCP
// client that a real benchmark session uses. It never calls a provider.
const mcpProbePluginSource = `import { Plugin } from "@opencode/plugin"
import { Rpc } from "@opencode/plugin/rpc"

const Probe = Rpc.define({
  id: "godot-benchmark-probe",
  events: {},
  methods: {
    catalog: {
      input: { type: "object", properties: {}, additionalProperties: false },
      output: {
        type: "object",
        properties: {
          tools: {
            type: "array",
            items: {
              type: "object",
              properties: {
                id: { type: "string" },
                description: { type: "string" },
              },
              required: ["id", "description"],
              additionalProperties: false,
            },
          },
        },
        required: ["tools"],
        additionalProperties: false,
      },
    },
    call: {
      input: {
        type: "object",
        properties: {
          sessionID: { type: "string" },
          agent: { type: "string" },
          name: { type: "string" },
          arguments: {},
        },
        required: ["sessionID", "agent", "name", "arguments"],
        additionalProperties: false,
      },
      output: {},
    },
    observe: {
      input: {
        type: "object",
        properties: { sessionID: { type: "string" }, clear: { type: "boolean" } },
        required: ["sessionID"],
        additionalProperties: false,
      },
      output: {
        type: "object",
        properties: { calls: { type: "array" } },
        required: ["calls"],
        additionalProperties: false,
      },
    },
  },
})

function contentText(content: any): string {
  if (!Array.isArray(content)) return ""
  return content
    .filter((part) => part && part.type === "text" && typeof part.text === "string")
    .map((part) => part.text)
    .join("\n")
}

export default Plugin.define({
  id: "godot-benchmark-probe",
  async setup(ctx) {
    const observed = new Map<string, Array<{ name: string, input: unknown }>>()
    await ctx.tool.hook("execute.before", (event: any) => {
      if (typeof event?.sessionID !== "string" || typeof event?.tool !== "string") return
      const calls = observed.get(event.sessionID) ?? []
      calls.push({ name: event.tool, input: event.input ?? {} })
      observed.set(event.sessionID, calls)
    })
    await ctx.rpc.register(Probe, {
      catalog: async () => {
        const tools = await ctx.tool.list()
        return {
          tools: tools.map((tool) => ({
            id: tool.id,
            description: typeof tool.description === "string" ? tool.description : "",
          })),
        }
      },
      call: async (input: any) => {
        const { sessionID, agent, name, arguments: args } = input
        const messageID = "msg_benchmark_probe"
        const toolID = "tool_benchmark_" + name
        try {
          const tool = (await ctx.tool.list()).find((item) => item.id === name)
          if (!tool) {
            return { ok: false, error: "OpenCode tool registry does not contain " + name }
          }
          const result = await tool.execute(args ?? {}, {
            sessionID,
            agent,
            messageID,
            id: toolID,
            progress: async () => {},
            signal: new AbortController().signal,
          })
          const structured = result?.output ?? result?.structured ?? null
          const content = result?.content ?? []
          if (result?.isError) {
            return {
              ok: false,
              error: contentText(content) || "MCP tool returned an error",
              content,
            }
          }
          return { ok: true, structured, content }
        } catch (error) {
          return {
            ok: false,
            error: error instanceof Error ? error.message : String(error),
          }
        }
      },
      observe: async (input: any) => {
        const calls = observed.get(input.sessionID) ?? []
        if (input.clear === true) observed.delete(input.sessionID)
        return { calls }
      },
    })
  },
})
`

var requiredMCPTools = []string{
	"godot_scene_list",
	"godot_scene_current",
}

type mcpServerStatus struct {
	Name   string `json:"name"`
	Status struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"status"`
}

type mcpToolInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type mcpProbeCatalog struct {
	Tools []mcpToolInfo `json:"tools"`
}

type mcpProbeResult struct {
	OK         bool              `json:"ok"`
	Structured json.RawMessage   `json:"structured"`
	Content    []json.RawMessage `json:"content"`
	Error      string            `json:"error"`
}

type mcpObservedCalls struct {
	Calls []toolCall `json:"calls"`
}

type mcpProbeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpSceneListResult struct {
	Scenes []string `json:"scenes"`
}

type mcpSceneCurrentResult struct {
	Scene   string `json:"scene"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Unsaved bool   `json:"unsaved"`
	Active  bool   `json:"active"`
}

type mcpPreflightReport struct {
	Tools             []mcpToolInfo
	Agent             string
	ConnectedServers  []string
	ExpectedScene     string
	SceneListCalls    int
	SceneCurrentCalls int
	// InstructionsEligible records the model-free part of OpenCode's
	// instruction-inclusion rule: at least one MCP tool and Code Mode execute
	// are allowed. OpenCode omits MCP instructions when either is fully denied.
	InstructionsEligible bool
}

func (server *openCodeServer) preflightMCP(
	ctx context.Context,
	projectRoot string,
	sessionID string,
	expectedScene string,
) (mcpPreflightReport, error) {
	return server.runMCPPreflight(ctx, projectRoot, sessionID, expectedScene)
}

// runMCPPreflight proves that the private OpenCode server can expose and execute
// godot-cli's MCP tools for one location and session without invoking a model.
func (server *openCodeServer) runMCPPreflight(
	ctx context.Context,
	projectRoot string,
	sessionID string,
	expectedScene string,
) (mcpPreflightReport, error) {
	report := mcpPreflightReport{ExpectedScene: expectedScene}

	if server == nil {
		return report, errors.New("mcp preflight: OpenCode server is nil")
	}
	if strings.TrimSpace(projectRoot) == "" {
		return report, errors.New("mcp preflight: project root is empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return report, errors.New("mcp preflight: session ID is empty")
	}
	if strings.TrimSpace(expectedScene) == "" {
		return report, errors.New("mcp preflight: expected scene is empty")
	}
	preflightContext, cancel := context.WithTimeout(ctx, mcpPreflightTimeout)
	defer cancel()

	session, err := server.getSession(preflightContext, sessionID)
	if err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"read OpenCode session",
			err,
		)
	}
	report.Agent = session.Agent
	if report.Agent == "" {
		report.Agent = "build"
	}
	if err := server.installMCPProbe(preflightContext, projectRoot); err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"install MCP probe plugin",
			err,
		)
	}

	statuses, err := server.waitForMCPConnected(preflightContext, projectRoot)
	if err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"wait for MCP server connection",
			err,
		)
	}
	for _, status := range statuses {
		if status.Status.Status == "connected" {
			report.ConnectedServers = append(report.ConnectedServers, status.Name)
		}
	}

	tools, err := server.waitForMCPCatalog(preflightContext, projectRoot)
	if err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"wait for MCP tool catalog",
			err,
		)
	}
	report.Tools = tools
	if err := server.verifyMCPPermissions(
		preflightContext,
		sessionID,
		report.Agent,
		tools,
		statuses,
	); err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"verify effective session permissions",
			err,
		)
	}
	report.InstructionsEligible = true

	listResult, err := server.callMCPProbeTool(
		preflightContext,
		projectRoot,
		sessionID,
		report.Agent,
		"godot_scene_list",
		map[string]any{},
	)
	if err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"call godot_scene_list",
			err,
		)
	}
	report.SceneListCalls++
	var scenes mcpSceneListResult
	if err := decodeMCPProbeStructured(listResult, &scenes); err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"decode godot_scene_list result",
			err,
		)
	}
	if !containsString(scenes.Scenes, expectedScene) {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"check godot_scene_list result",
			fmt.Errorf(
				"expected scene %q was not listed; available scenes: %s",
				expectedScene,
				formatStringList(scenes.Scenes),
			),
		)
	}

	currentResult, err := server.callMCPProbeTool(
		preflightContext,
		projectRoot,
		sessionID,
		report.Agent,
		"godot_scene_current",
		map[string]any{"expected_scene": expectedScene},
	)
	if err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"call godot_scene_current",
			err,
		)
	}
	report.SceneCurrentCalls++
	var current mcpSceneCurrentResult
	if err := decodeMCPProbeStructured(currentResult, &current); err != nil {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"decode godot_scene_current result",
			err,
		)
	}
	if !current.Active {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"check godot_scene_current result",
			fmt.Errorf(
				"editor reported no active scene; expected %q",
				expectedScene,
			),
		)
	}
	if current.Scene != expectedScene {
		return report, server.preflightError(
			preflightContext,
			projectRoot,
			"check godot_scene_current result",
			fmt.Errorf(
				"active scene is %q, expected %q",
				current.Scene,
				expectedScene,
			),
		)
	}
	// Preflight executes real tools through the same registry and therefore the
	// execution hook sees it. Start the benchmark with an empty observation set.
	return server.finishMCPPreflight(preflightContext, projectRoot, sessionID, report)
}

func (server *openCodeServer) installMCPProbe(ctx context.Context, projectRoot string) error {
	path := filepath.Join(projectRoot, filepath.FromSlash(mcpProbePluginPath))
	existing, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("reading probe plugin %s: %w", path, readErr)
	}
	if readErr != nil || string(existing) != mcpProbePluginSource {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating probe plugin directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(mcpProbePluginSource), 0o644); err != nil {
			return fmt.Errorf("writing probe plugin %s: %w", path, err)
		}
	}
	// Reload after writing so OpenCode rediscovers plugins even when the
	// location was loaded before the probe existed. A reload preserves the
	// session and its permissions but rebuilds the location's MCP clients.
	if _, err := server.request(
		ctx,
		"POST",
		"/api/location/reload",
		map[string]any{},
		nil,
	); err != nil {
		return fmt.Errorf("reloading OpenCode location: %w", err)
	}
	return nil
}

func (server *openCodeServer) waitForMCPConnected(
	ctx context.Context,
	projectRoot string,
) ([]mcpServerStatus, error) {
	ticker := time.NewTicker(mcpPollInterval)
	defer ticker.Stop()
	var lastStatuses []mcpServerStatus
	var lastErr error

	for {
		statuses, err := server.mcpServers(ctx, projectRoot)
		if err == nil {
			lastStatuses = statuses
			lastErr = nil
			status, found := findMCPServer(statuses, mcpBenchmarkServerName)
			if found {
				switch status.Status.Status {
				case "connected":
					return statuses, nil
				case "failed":
					return statuses, fmt.Errorf(
						"MCP server %q failed to connect: %s",
						mcpBenchmarkServerName,
						status.Status.Error,
					)
				case "needs_auth":
					return statuses, fmt.Errorf(
						"MCP server %q needs authentication: %s",
						mcpBenchmarkServerName,
						status.Status.Error,
					)
				case "disabled":
					return statuses, fmt.Errorf(
						"MCP server %q is disabled in the effective configuration",
						mcpBenchmarkServerName,
					)
				}
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			cause := ctx.Err()
			if lastErr != nil {
				cause = fmt.Errorf("%w; last API error: %v", cause, lastErr)
			}
			return lastStatuses, fmt.Errorf(
				"timed out waiting for MCP server %q; last status: %s: %w",
				mcpBenchmarkServerName,
				formatMCPStatuses(lastStatuses),
				cause,
			)
		case <-ticker.C:
		}
	}
}

func (server *openCodeServer) waitForMCPCatalog(
	ctx context.Context,
	projectRoot string,
) ([]mcpToolInfo, error) {
	ticker := time.NewTicker(mcpPollInterval)
	defer ticker.Stop()
	var lastErr error
	reloaded := false

	for {
		tools, err := server.mcpProbeCatalog(ctx, projectRoot)
		if err == nil {
			missing := missingMCPTools(tools, requiredMCPTools)
			if len(missing) == 0 {
				return tools, nil
			}
			lastErr = fmt.Errorf(
				"catalog is missing required tools: %s",
				strings.Join(missing, ", "),
			)
		} else {
			lastErr = err
			if !reloaded && isRPCMethodUnavailable(err) {
				reloaded = true
				if _, reloadErr := server.request(
					ctx,
					"POST",
					"/api/location/reload",
					map[string]any{},
					nil,
				); reloadErr == nil {
					continue
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"timed out waiting for MCP tool catalog: %v: %w",
				lastErr,
				ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func (server *openCodeServer) verifyMCPPermissions(
	ctx context.Context,
	sessionID string,
	agent string,
	tools []mcpToolInfo,
	statuses []mcpServerStatus,
) error {
	allowed := []struct {
		action   string
		resource string
		label    string
	}{
		{"godot_scene_list", "*", "godot_scene_list"},
		{"godot_scene_current", "*", "godot_scene_current"},
		{"execute", "*", "Code Mode execute"},
		{"skill", "godot-cli", "godot-cli skill"},
	}
	for _, check := range allowed {
		effect, err := server.evaluatePermission(
			ctx,
			sessionID,
			agent,
			check.action,
			check.resource,
		)
		if err != nil {
			return fmt.Errorf(
				"evaluating permission %s for %s: %w",
				check.action,
				check.label,
				err,
			)
		}
		if effect != "allow" {
			return fmt.Errorf(
				"effective permission for %s (action %q) is %q, want %q; OpenCode also omits MCP server instructions when every tool or Code Mode execute is denied",
				check.label,
				check.action,
				effect,
				"allow",
			)
		}
	}

	denied := []struct {
		action   string
		resource string
		label    string
	}{
		{"other_scene_list", "*", "unrelated MCP tool"},
		{"skill", "other", "unrelated skill"},
	}
	for _, status := range statuses {
		if status.Name == "" || status.Name == mcpBenchmarkServerName {
			continue
		}
		denied = append(denied, struct {
			action   string
			resource string
			label    string
		}{
			normalizeMCPToolName(status.Name) + "_probe",
			"*",
			"MCP server " + status.Name,
		})
	}
	for _, check := range denied {
		effect, err := server.evaluatePermission(
			ctx,
			sessionID,
			agent,
			check.action,
			check.resource,
		)
		if err != nil {
			return fmt.Errorf(
				"evaluating permission %s for %s: %w",
				check.action,
				check.label,
				err,
			)
		}
		if effect != "deny" {
			return fmt.Errorf(
				"effective permission for %s (action %q) is %q, want %q; only the godot MCP server may be reachable",
				check.label,
				check.action,
				effect,
				"deny",
			)
		}
	}

	for _, tool := range tools {
		if !strings.HasPrefix(tool.ID, mcpBenchmarkServerName+"_") {
			continue
		}
		if strings.TrimSpace(tool.Description) == "" {
			return fmt.Errorf(
				"godot tool %q has an empty description; the MCP catalog is not usable",
				tool.ID,
			)
		}
	}
	if len(tools) == 0 {
		return errors.New("MCP tool catalog is empty")
	}
	return nil
}

func (server *openCodeServer) mcpServers(
	ctx context.Context,
	projectRoot string,
) ([]mcpServerStatus, error) {
	var response struct {
		Data []mcpServerStatus `json:"data"`
	}
	if _, err := server.request(
		ctx,
		"GET",
		locationAPIPath("/api/mcp", projectRoot),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (server *openCodeServer) mcpProbeCatalog(
	ctx context.Context,
	projectRoot string,
) ([]mcpToolInfo, error) {
	var response struct {
		Output mcpProbeCatalog `json:"output"`
	}
	if _, err := server.request(
		ctx,
		"POST",
		locationAPIPath(
			"/api/rpc/"+url.PathEscape(mcpProbeRPCID)+"/catalog",
			projectRoot,
		),
		map[string]any{"input": map[string]any{}},
		&response,
	); err != nil {
		return nil, err
	}
	return response.Output.Tools, nil
}

func (server *openCodeServer) callMCPProbeTool(
	ctx context.Context,
	projectRoot string,
	sessionID string,
	agent string,
	name string,
	arguments map[string]any,
) (mcpProbeResult, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	var response struct {
		Output json.RawMessage `json:"output"`
	}
	if _, err := server.request(
		ctx,
		"POST",
		locationAPIPath(
			"/api/rpc/"+url.PathEscape(mcpProbeRPCID)+"/call",
			projectRoot,
		),
		map[string]any{
			"input": map[string]any{
				"sessionID": sessionID,
				"agent":     agent,
				"name":      name,
				"arguments": arguments,
			},
		},
		&response,
	); err != nil {
		return mcpProbeResult{}, err
	}
	if len(bytes.TrimSpace(response.Output)) == 0 {
		return mcpProbeResult{}, fmt.Errorf("probe RPC returned no output for %s", name)
	}
	var result mcpProbeResult
	if err := json.Unmarshal(response.Output, &result); err != nil {
		return mcpProbeResult{}, fmt.Errorf(
			"decoding probe RPC output for %s: %w (%s)",
			name,
			err,
			tailText(string(response.Output), 512),
		)
	}
	if !result.OK {
		if result.Error == "" {
			result.Error = "probe tool execution failed without an error message"
		}
		return result, fmt.Errorf("%s failed: %s", name, result.Error)
	}
	return result, nil
}

func (server *openCodeServer) mcpObservedToolCalls(
	ctx context.Context,
	projectRoot string,
	sessionID string,
	consume bool,
) ([]toolCall, error) {
	var response struct {
		Output mcpObservedCalls `json:"output"`
	}
	if _, err := server.request(
		ctx,
		"POST",
		locationAPIPath("/api/rpc/"+url.PathEscape(mcpProbeRPCID)+"/observe", projectRoot),
		map[string]any{"input": map[string]any{"sessionID": sessionID, "clear": consume}},
		&response,
	); err != nil {
		return nil, err
	}
	return response.Output.Calls, nil
}

func (server *openCodeServer) clearMCPToolObservations(
	ctx context.Context,
	projectRoot string,
	sessionID string,
) error {
	_, err := server.mcpObservedToolCalls(ctx, projectRoot, sessionID, true)
	return err
}

func (server *openCodeServer) finishMCPPreflight(
	ctx context.Context,
	projectRoot string,
	sessionID string,
	report mcpPreflightReport,
) (mcpPreflightReport, error) {
	if err := server.clearMCPToolObservations(ctx, projectRoot, sessionID); err != nil {
		return report, server.preflightError(ctx, projectRoot, "clear tool observations", err)
	}
	return report, nil
}

func (server *openCodeServer) evaluatePermission(
	ctx context.Context,
	sessionID string,
	agent string,
	action string,
	resource string,
) (string, error) {
	payload := map[string]any{
		"action":    action,
		"resources": []string{resource},
	}
	if agent != "" {
		payload["agent"] = agent
	}
	var response struct {
		Data struct {
			Effect string `json:"effect"`
		} `json:"data"`
	}
	if _, err := server.request(
		ctx,
		"POST",
		"/api/session/"+url.PathEscape(sessionID)+"/permission",
		payload,
		&response,
	); err != nil {
		return "", err
	}
	if response.Data.Effect == "" {
		return "", errors.New("permission endpoint returned no effect")
	}
	return response.Data.Effect, nil
}

func decodeMCPProbeStructured(result mcpProbeResult, target any) error {
	data := bytes.TrimSpace(result.Structured)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		for _, content := range result.Content {
			var part mcpProbeContent
			if json.Unmarshal(content, &part) == nil && part.Type == "text" {
				data = []byte(part.Text)
				break
			}
		}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("tool returned no structured content")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf(
			"decoding tool structured content: %w (%s)",
			err,
			tailText(string(data), 512),
		)
	}
	return nil
}

func (server *openCodeServer) preflightError(
	ctx context.Context,
	projectRoot string,
	stage string,
	err error,
) error {
	return fmt.Errorf(
		"mcp preflight: %s: %w%s",
		stage,
		err,
		server.mcpDiagnostics(ctx, projectRoot),
	)
}

func (server *openCodeServer) mcpDiagnostics(
	ctx context.Context,
	projectRoot string,
) string {
	diagnosticContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	statuses, statusErr := server.mcpServers(diagnosticContext, projectRoot)
	var builder strings.Builder
	builder.WriteString("\nMCP servers: ")
	if statusErr != nil {
		builder.WriteString("unavailable: " + statusErr.Error())
	} else {
		builder.WriteString(formatMCPStatuses(statuses))
	}
	if ctx != nil && ctx.Err() != nil {
		builder.WriteString("\npreflight context: " + ctx.Err().Error())
	}
	if logs := strings.TrimSpace(server.logs()); logs != "" {
		builder.WriteString("\nOpenCode server logs (tail):\n")
		builder.WriteString(tailText(logs, 4096))
	}
	return builder.String()
}

func findMCPServer(statuses []mcpServerStatus, name string) (mcpServerStatus, bool) {
	for _, status := range statuses {
		if status.Name == name {
			return status, true
		}
	}
	return mcpServerStatus{}, false
}

func formatMCPStatuses(statuses []mcpServerStatus) string {
	if len(statuses) == 0 {
		return "(no MCP servers configured)"
	}
	parts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		value := status.Name + "=" + status.Status.Status
		if status.Status.Error != "" {
			value += " (" + status.Status.Error + ")"
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ", ")
}

func missingMCPTools(tools []mcpToolInfo, required []string) []string {
	present := make(map[string]bool, len(tools))
	for _, tool := range tools {
		present[tool.ID] = true
	}
	var missing []string
	for _, name := range required {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func countMCPTools(tools []mcpToolInfo) int {
	count := 0
	for _, tool := range tools {
		if strings.HasPrefix(tool.ID, mcpBenchmarkServerName+"_") {
			count++
		}
	}
	return count
}

func verifyProfileCatalog(profile string, tools []mcpToolInfo) error {
	controlled := []string{
		"godot_scene_apply", "godot_resource_apply", "godot_project_settings_apply",
		"godot_input_map_apply", "godot_runtime_start", "godot_runtime_status",
		"godot_runtime_capture", "godot_runtime_stop",
	}
	present := make(map[string]bool, len(tools))
	for _, tool := range tools {
		present[tool.ID] = true
	}
	for _, name := range controlled {
		switch profile {
		case "legacy-mcp":
			if present[name] {
				return fmt.Errorf("legacy-mcp unexpectedly exposes %s", name)
			}
		case "improved-mcp", "normal-mcp", "hybrid-mcp":
			if !present[name] {
				return fmt.Errorf("%s does not expose %s", profile, name)
			}
		default:
			return fmt.Errorf("unsupported MCP profile %q", profile)
		}
	}
	return nil
}

func isRPCMethodUnavailable(err error) bool {
	message := err.Error()

	return strings.Contains(message, "rpc.unavailable") ||
		strings.Contains(message, "rpc.method_not_found")
}

func normalizeMCPToolName(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '_',
			character == '-':
			builder.WriteRune(character)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

func tailText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return "…" + value[len(value)-limit:]
}
