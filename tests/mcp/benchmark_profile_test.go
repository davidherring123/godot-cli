package mcp_test

import (
	"strings"
	"testing"

	agentmcp "github.com/davidherring123/godot-cli/internal/agent/mcp"
)

func TestBenchmarkProfiles(t *testing.T) {
	excluded := map[string]bool{
		"scene.apply": true, "resource.apply": true, "project.settings.apply": true,
		"input.map_apply": true, "runtime.start": true, "runtime.status": true,
		"runtime.capture": true, "runtime.stop": true,
	}

	legacy, err := agentmcp.ResolveBenchmarkProfile(agentmcp.BenchmarkProfileLegacy, "/project")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range legacy.Actions {
		if excluded[action.Name] {
			t.Fatalf("legacy profile exposed excluded action %q", action.Name)
		}
	}
	for _, unavailable := range []string{"scene_apply", "resource_apply", "runtime_start"} {
		if strings.Contains(legacy.Instructions, unavailable) {
			t.Fatalf("legacy instructions mention unavailable tool %q", unavailable)
		}
	}

	improved, err := agentmcp.ResolveBenchmarkProfile(agentmcp.BenchmarkProfileImproved, "/project")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, action := range improved.Actions {
		found[action.Name] = true
	}
	for name := range excluded {
		if !found[name] {
			t.Fatalf("improved profile omitted action %q", name)
		}
	}
	if !strings.Contains(improved.Instructions, "runtime_start") {
		t.Fatal("improved instructions omit runtime feedback guidance")
	}
}
