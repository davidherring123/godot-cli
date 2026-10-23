package core_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/batch"
	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/resource"
)

func TestResourceApplyDryRunAndApplyOrdered(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_apply_succeeds/material.tres"
	absolute := resourceFilesystemPath(session, path)

	operations := []resource.ApplyOperation{
		{
			Operation: "create",
			Resource:  path,
			Type:      "StandardMaterial3D",
			Properties: map[string]json.RawMessage{
				"albedo_color": json.RawMessage(
					`{"r":0.2,"g":0.4,"b":0.6,"a":1}`,
				),
			},
		},
		{
			Operation: "set",
			Resource:  path,
			Properties: map[string]json.RawMessage{
				"roughness": json.RawMessage(`0.4`),
			},
		},
	}

	dryRun, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: operations,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("expected resource.apply dry run to succeed: %v", err)
	}

	if !dryRun.DryRun || dryRun.Applied {
		t.Fatalf("expected an unapplied dry run, got %+v", dryRun)
	}

	if dryRun.OperationCount != 2 || dryRun.Changed != 2 || dryRun.Unchanged != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", dryRun)
	}

	if dryRun.Operations[0].Status != "created" ||
		dryRun.Operations[1].Status != "updated" {
		t.Fatalf("unexpected dry-run statuses: %+v", dryRun.Operations)
	}

	if _, err := os.Stat(absolute); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run created %s: %v", path, err)
	}

	applied, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: operations,
	})
	if err != nil {
		t.Fatalf("expected resource.apply to succeed: %v", err)
	}

	if applied.DryRun || !applied.Applied {
		t.Fatalf("expected an applied batch, got %+v", applied)
	}

	if applied.OperationCount != 2 || applied.Changed != 2 {
		t.Fatalf("unexpected apply summary: %+v", applied)
	}

	text := readResourceFile(t, session, path)

	for _, expected := range []string{
		`type="StandardMaterial3D"`,
		"albedo_color = Color(0.2, 0.4, 0.6, 1)",
		"roughness = 0.4",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in the applied resource:\n%s", expected, text)
		}
	}
}

func TestResourceApplyDryRunDoesNotMutateLoadedResource(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_apply_rollback/material.tres"

	before := inspectResource(t, session, resource.InspectParams{
		Resource:   path,
		Properties: []string{"roughness"},
	})

	_, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: []resource.ApplyOperation{
			{
				Operation: "set",
				Resource:  path,
				Properties: map[string]json.RawMessage{
					"roughness": json.RawMessage(`0.9`),
				},
			},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("expected resource.apply dry run to succeed: %v", err)
	}

	after := inspectResource(t, session, resource.InspectParams{
		Resource:   path,
		Properties: []string{"roughness"},
	})

	if string(before.Properties[0].Value) != string(after.Properties[0].Value) {
		t.Fatalf(
			"dry run changed the loaded resource: before %s, after %s",
			before.Properties[0].Value,
			after.Properties[0].Value,
		)
	}
}

func TestResourceApplyFailureReportsOperationAndRollsBack(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_apply_rollback/material.tres"
	missing := "res://cases/resource_apply_rollback/missing.tres"
	before := readResourceFile(t, session, path)

	_, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: []resource.ApplyOperation{
			{
				Operation: "set",
				Resource:  path,
				Properties: map[string]json.RawMessage{
					"roughness": json.RawMessage(`0.9`),
				},
			},
			{
				Operation: "set",
				Resource:  missing,
				Properties: map[string]json.RawMessage{
					"roughness": json.RawMessage(`0.1`),
				},
			},
		},
	})

	assertAPIErrorCode(t, err, bridge.ErrorCodeNotFound)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, expected := range []string{"operation 1", "set", missing} {
		if !strings.Contains(apiError.Message, expected) {
			t.Fatalf("expected %q in %q", expected, apiError.Message)
		}
	}

	after := readResourceFile(t, session, path)

	if after != before {
		t.Fatalf(
			"expected the failed batch to leave the resource unchanged:\nbefore:\n%s\nafter:\n%s",
			before,
			after,
		)
	}
}

func TestResourceApplyRejectsOversizedBatch(t *testing.T) {
	operations := make([]resource.ApplyOperation, batch.MaxOperations+1)

	for index := range operations {
		operations[index] = resource.ApplyOperation{
			Operation: "delete",
			Resource:  "res://cases/resource_apply_limit/missing.tres",
		}
	}

	_, err := resource.Apply("", resource.ApplyParams{Operations: operations})

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) || !strings.Contains(apiError.Message, "At most 100") {
		t.Fatalf("expected an operation limit error, got %v", err)
	}
}

func TestResourceApplyDryRunDeleteDoesNotRemoveFile(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_apply_delete/orphan.tres"
	absolute := resourceFilesystemPath(session, path)

	dryRun, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: []resource.ApplyOperation{
			{Operation: "delete", Resource: path},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("expected resource.apply dry run to succeed: %v", err)
	}

	if dryRun.Operations[0].Status != "deleted" {
		t.Fatalf("unexpected dry-run status: %+v", dryRun.Operations[0])
	}

	if _, err := os.Stat(absolute); err != nil {
		t.Fatalf("dry run removed %s: %v", path, err)
	}

	applied, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: []resource.ApplyOperation{
			{Operation: "delete", Resource: path},
		},
	})
	if err != nil {
		t.Fatalf("expected resource.apply delete to succeed: %v", err)
	}

	if !applied.Applied || applied.Changed != 1 {
		t.Fatalf("unexpected delete summary: %+v", applied)
	}

	if _, err := os.Stat(absolute); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be deleted, stat returned: %v", path, err)
	}
}

func TestResourceApplyReportsInvalidOperationIndex(t *testing.T) {
	_, err := resource.Apply("", resource.ApplyParams{
		Operations: []resource.ApplyOperation{
			{
				Operation: "set",
				Resource:  "res://cases/resource_apply_invalid/material.tres",
			},
		},
	})

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	for _, expected := range []string{"operation 0", "set", "nonempty properties"} {
		if !strings.Contains(apiError.Message, expected) {
			t.Fatalf("expected %q in %q", expected, apiError.Message)
		}
	}
}

func TestResourceApplyReferencesEarlierCreatedResource(t *testing.T) {
	session := requireSuite(t)
	texture := "res://cases/resource_apply_created_reference/texture.tres"
	atlas := "res://cases/resource_apply_created_reference/atlas.tres"

	operations := []resource.ApplyOperation{
		{
			Operation: "create",
			Resource:  texture,
			Type:      "PlaceholderTexture2D",
		},
		{
			Operation: "create",
			Resource:  atlas,
			Type:      "AtlasTexture",
			Properties: map[string]json.RawMessage{
				"atlas": json.RawMessage(
					`{"resource":"` + texture + `"}`,
				),
			},
		},
	}

	dryRun, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: operations,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("expected a dry run referencing an earlier create: %v", err)
	}

	if dryRun.Changed != 2 {
		t.Fatalf("unexpected dry-run summary: %+v", dryRun)
	}

	if dryRun.Operations[1].Type != "AtlasTexture" {
		t.Fatalf("unexpected preview type: %+v", dryRun.Operations[1])
	}

	for _, path := range []string{texture, atlas} {
		if _, err := os.Stat(resourceFilesystemPath(session, path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dry run wrote %s: %v", path, err)
		}
	}

	applied, err := resource.Apply(session.Root(), resource.ApplyParams{
		Operations: operations,
	})
	if err != nil {
		t.Fatalf("expected apply to reference an earlier create: %v", err)
	}

	if !applied.Applied || applied.Changed != 2 {
		t.Fatalf("unexpected apply summary: %+v", applied)
	}

	text := readResourceFile(t, session, atlas)

	if !strings.Contains(text, texture) {
		t.Fatalf("expected the atlas to reference %s:\n%s", texture, text)
	}
}

func TestResourceApplyDeleteRejectsCreatedReference(t *testing.T) {
	session := requireSuite(t)
	texture := "res://cases/resource_apply_created_reference_delete/texture.tres"
	atlas := "res://cases/resource_apply_created_reference_delete/atlas.tres"

	operations := []resource.ApplyOperation{
		{
			Operation: "create",
			Resource:  texture,
			Type:      "PlaceholderTexture2D",
		},
		{
			Operation: "create",
			Resource:  atlas,
			Type:      "AtlasTexture",
			Properties: map[string]json.RawMessage{
				"atlas": json.RawMessage(
					`{"resource":"` + texture + `"}`,
				),
			},
		},
		{
			Operation: "delete",
			Resource:  texture,
		},
	}

	for _, dryRun := range []bool{true, false} {
		_, err := resource.Apply(session.Root(), resource.ApplyParams{
			Operations: operations,
			DryRun:     dryRun,
		})

		assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)

		var apiError *bridge.APIError

		if !errors.As(err, &apiError) {
			t.Fatalf("expected an API error, got %v", err)
		}

		for _, expected := range []string{"operation 2", "still referenced", atlas} {
			if !strings.Contains(apiError.Message, expected) {
				t.Fatalf(
					"dry_run=%t: expected %q in %q",
					dryRun,
					expected,
					apiError.Message,
				)
			}
		}

		for _, path := range []string{texture, atlas} {
			if _, err := os.Stat(resourceFilesystemPath(session, path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("dry_run=%t wrote %s: %v", dryRun, path, err)
			}
		}
	}
}

func TestResourceApplyDeleteRejectsSetReference(t *testing.T) {
	session := requireSuite(t)
	texture := "res://cases/resource_apply_set_reference/texture.tres"
	atlas := "res://cases/resource_apply_set_reference/atlas.tres"
	before := readResourceFile(t, session, atlas)

	operations := []resource.ApplyOperation{
		{
			Operation: "create",
			Resource:  texture,
			Type:      "PlaceholderTexture2D",
		},
		{
			Operation: "set",
			Resource:  atlas,
			Properties: map[string]json.RawMessage{
				"atlas": json.RawMessage(
					`{"resource":"` + texture + `"}`,
				),
			},
		},
		{
			Operation: "delete",
			Resource:  texture,
		},
	}

	for _, dryRun := range []bool{true, false} {
		_, err := resource.Apply(session.Root(), resource.ApplyParams{
			Operations: operations,
			DryRun:     dryRun,
		})

		assertAPIErrorCode(t, err, bridge.ErrorCodeConflict)

		var apiError *bridge.APIError

		if !errors.As(err, &apiError) {
			t.Fatalf("expected an API error, got %v", err)
		}

		for _, expected := range []string{"operation 2", "still referenced", atlas} {
			if !strings.Contains(apiError.Message, expected) {
				t.Fatalf(
					"dry_run=%t: expected %q in %q",
					dryRun,
					expected,
					apiError.Message,
				)
			}
		}

		if _, err := os.Stat(resourceFilesystemPath(session, texture)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dry_run=%t wrote %s: %v", dryRun, texture, err)
		}

		if after := readResourceFile(t, session, atlas); after != before {
			t.Fatalf(
				"dry_run=%t changed the atlas fixture:\nbefore:\n%s\nafter:\n%s",
				dryRun,
				before,
				after,
			)
		}
	}
}

func TestResourceApplyAddonRejectsExclusiveFields(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[resource.ApplyParams, resource.ApplyResult](
		session.Client,
		resource.ApplyName,
		resource.ApplyParams{
			Operations: []resource.ApplyOperation{
				{
					Operation: "set",
					Resource:  "res://cases/resource_apply_addon_fields/material.tres",
					Type:      "StandardMaterial3D",
					Properties: map[string]json.RawMessage{
						"roughness": json.RawMessage(`0.4`),
					},
				},
			},
		},
	)

	assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

	var apiError *bridge.APIError

	if !errors.As(err, &apiError) ||
		!strings.Contains(apiError.Message, "does not accept type") {
		t.Fatalf("expected an addon field exclusivity error, got %v", err)
	}
}

func TestResourceApplyRejectsFieldsFromOtherOperations(t *testing.T) {
	cases := []struct {
		name      string
		operation resource.ApplyOperation
		field     string
	}{
		{
			name: "set with type",
			operation: resource.ApplyOperation{
				Operation: "set",
				Resource:  "res://cases/resource_apply_fields/material.tres",
				Type:      "StandardMaterial3D",
				Properties: map[string]json.RawMessage{
					"roughness": json.RawMessage(`0.4`),
				},
			},
			field: "type",
		},
		{
			name: "delete with properties",
			operation: resource.ApplyOperation{
				Operation: "delete",
				Resource:  "res://cases/resource_apply_fields/material.tres",
				Properties: map[string]json.RawMessage{
					"roughness": json.RawMessage(`0.4`),
				},
			},
			field: "properties",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := resource.Apply("", resource.ApplyParams{
				Operations: []resource.ApplyOperation{testCase.operation},
			})

			assertAPIErrorCode(t, err, bridge.ErrorCodeInvalidArgument)

			var apiError *bridge.APIError

			if !errors.As(err, &apiError) {
				t.Fatalf("expected an API error, got %v", err)
			}

			if !strings.Contains(apiError.Message, "does not accept") ||
				!strings.Contains(apiError.Message, testCase.field) {
				t.Fatalf(
					"expected a field exclusivity error for %s, got %q",
					testCase.field,
					apiError.Message,
				)
			}
		})
	}
}
