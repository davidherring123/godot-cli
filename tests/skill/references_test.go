package skill_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/agent/skill"
	"github.com/davidherring123/godot-cli/internal/core"
)

func TestGenerateReferencesEveryCoreAction(t *testing.T) {
	files, err := skill.Generate("v1.2.3")
	if err != nil {
		t.Fatalf("generating references: %v", err)
	}

	var reference skill.Reference

	if err := json.Unmarshal(files["references/actions.json"], &reference); err != nil {
		t.Fatalf("parsing actions.json: %v", err)
	}

	if reference.Version != "v1.2.3" {
		t.Fatalf("reference version = %q, want v1.2.3", reference.Version)
	}

	if len(reference.Actions) != len(core.All()) {
		t.Fatalf("reference has %d actions, want %d", len(reference.Actions), len(core.All()))
	}

	for _, action := range reference.Actions {
		for _, path := range []string{action.InputSchema, action.OutputSchema} {
			data, ok := files["references/"+path]
			if !ok {
				t.Fatalf("%s is missing generated schema %s", action.Name, path)
			}

			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatalf("%s is not JSON: %v", path, err)
			}

			if len(schema) == 0 {
				t.Fatalf("%s is an empty schema", path)
			}
		}
	}

	markdown := string(files["references/actions.md"])

	for _, action := range reference.Actions {
		if !strings.Contains(markdown, "`"+action.Name+"`") {
			t.Errorf("actions.md does not document %s", action.Name)
		}
	}
}
