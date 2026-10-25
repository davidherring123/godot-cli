package core_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/core/resource"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
	"github.com/davidherring123/godot-cli/tests/harness"
)

func TestResourceCreateSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_create_succeeds/material.tres"

	result := createResource(t, session, resource.CreateParams{
		Resource: path,
		Type:     "StandardMaterial3D",
		Properties: map[string]json.RawMessage{
			"albedo_color": json.RawMessage(`{"r":0.25,"g":0.5,"b":0.75,"a":1}`),
			"roughness":    json.RawMessage(`0.4`),
		},
	})

	if result.Resource != path {
		t.Fatalf("expected resource %s, got %s", path, result.Resource)
	}

	if result.Type != "StandardMaterial3D" {
		t.Fatalf("expected type StandardMaterial3D, got %s", result.Type)
	}

	var color struct {
		R float64 `json:"r"`
		G float64 `json:"g"`
		B float64 `json:"b"`
		A float64 `json:"a"`
	}

	if err := json.Unmarshal(result.Properties["albedo_color"], &color); err != nil {
		t.Fatalf("decoding albedo_color: %v", err)
	}

	if color.R != 0.25 || color.G != 0.5 || color.B != 0.75 || color.A != 1 {
		t.Fatalf("expected color (0.25, 0.5, 0.75, 1), got %+v", color)
	}

	text := readResourceFile(t, session, path)

	for _, expected := range []string{
		`[gd_resource type="StandardMaterial3D"`,
		"albedo_color = Color(0.25, 0.5, 0.75, 1)",
		"roughness = 0.4",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf(
				"expected the saved resource to contain %q:\n%s",
				expected,
				text,
			)
		}
	}

	if err := session.LogJSON(resource.CreateName, result); err != nil {
		t.Fatal(err)
	}
}

func TestResourceCreateCustomClassSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_create_custom_class/steel.tres"

	result := createResource(t, session, resource.CreateParams{
		Resource: path,
		Type:     "MaterialData",
		Properties: map[string]json.RawMessage{
			"color":     json.RawMessage(`{"r":0.2,"g":0.4,"b":0.6,"a":1}`),
			"roughness": json.RawMessage(`0.9`),
		},
	})

	if result.Type != "MaterialData" {
		t.Fatalf("expected type MaterialData, got %s", result.Type)
	}

	text := readResourceFile(t, session, path)

	for _, expected := range []string{
		"material_data.gd",
		"color = Color(0.2, 0.4, 0.6, 1)",
		"roughness = 0.9",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf(
				"expected the saved resource to contain %q:\n%s",
				expected,
				text,
			)
		}
	}

	inspected := inspectResource(t, session, resource.InspectParams{
		Resource:   path,
		Properties: []string{"roughness"},
	})

	if inspected.Type != "MaterialData" {
		t.Fatalf("expected inspected type MaterialData, got %s", inspected.Type)
	}

	if len(inspected.Properties) != 1 {
		t.Fatalf(
			"expected one inspected property, got %d",
			len(inspected.Properties),
		)
	}

	if inspected.Properties[0].Name != "roughness" {
		t.Fatalf(
			"expected inspected property roughness, got %s",
			inspected.Properties[0].Name,
		)
	}

	var roughness float64

	if err := json.Unmarshal(inspected.Properties[0].Value, &roughness); err != nil {
		t.Fatalf("decoding inspected roughness: %v", err)
	}

	if roughness != 0.9 {
		t.Fatalf("expected inspected roughness 0.9, got %v", roughness)
	}
}

func TestResourceCreateResSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_create_res/data.res"

	result := createResource(t, session, resource.CreateParams{
		Resource: path,
		Type:     "StandardMaterial3D",
	})

	if result.Resource != path {
		t.Fatalf("expected resource %s, got %s", path, result.Resource)
	}

	data, err := os.ReadFile(resourceFilesystemPath(session, path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	if !bytes.HasPrefix(data, []byte("RSRC")) {
		t.Fatalf("expected a binary resource file, got %q", data)
	}
}

func TestResourceCreateMissingDirectorySucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_create_missing_directory/nested/deep/value.tres"

	result := createResource(t, session, resource.CreateParams{
		Resource: path,
		Type:     "StandardMaterial3D",
	})

	if result.Resource != path {
		t.Fatalf("expected resource %s, got %s", path, result.Resource)
	}

	if text := readResourceFile(t, session, path); !strings.Contains(text, "StandardMaterial3D") {
		t.Fatalf("expected the created resource on disk:\n%s", text)
	}
}

func TestResourceCreateExistingFileConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_create_conflict/existing.tres"

	_, err := bridge.Call[resource.CreateParams, resource.CreateResult](
		session.Client,
		resource.CreateName,
		resource.CreateParams{Resource: path, Type: "StandardMaterial3D"},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)

	text := readResourceFile(t, session, path)

	if !strings.Contains(text, "Color(0.5, 0.5, 0.5, 1)") {
		t.Fatalf("expected the existing file to be untouched:\n%s", text)
	}
}

func TestResourceCreateInvalidPathRejected(t *testing.T) {
	session := requireSuite(t)

	for _, path := range []string{
		"cases/resource_create_invalid/data.tres",
		"res://cases/resource_create_invalid/data.txt",
	} {
		_, err := bridge.Call[resource.CreateParams, resource.CreateResult](
			session.Client,
			resource.CreateName,
			resource.CreateParams{Resource: path, Type: "StandardMaterial3D"},
		)

		assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
	}
}

func TestResourceCreateUnknownTypeRejected(t *testing.T) {
	session := requireSuite(t)

	for _, resourceType := range []string{"NotAResourceType", "Node2D"} {
		_, err := bridge.Call[resource.CreateParams, resource.CreateResult](
			session.Client,
			resource.CreateName,
			resource.CreateParams{
				Resource: "res://cases/resource_create_unknown/one.tres",
				Type:     resourceType,
			},
		)

		assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
	}
}

func TestResourceInspectSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_inspect_succeeds/material.tres"

	result := inspectResource(t, session, resource.InspectParams{
		Resource:   path,
		Properties: []string{"albedo_color"},
	})

	if result.Resource != path {
		t.Fatalf("expected resource %s, got %s", path, result.Resource)
	}

	if result.Type != "StandardMaterial3D" {
		t.Fatalf("expected type StandardMaterial3D, got %s", result.Type)
	}

	if len(result.Properties) != 1 {
		t.Fatalf(
			"expected one inspected property, got %d",
			len(result.Properties),
		)
	}

	if result.Properties[0].Name != "albedo_color" {
		t.Fatalf(
			"expected inspected property albedo_color, got %s",
			result.Properties[0].Name,
		)
	}

	var color struct {
		R float64 `json:"r"`
		G float64 `json:"g"`
		B float64 `json:"b"`
		A float64 `json:"a"`
	}

	if err := json.Unmarshal(result.Properties[0].Value, &color); err != nil {
		t.Fatalf("decoding albedo_color: %v", err)
	}

	if color.R != 1 || color.G != 0 || color.B != 0 || color.A != 1 {
		t.Fatalf("expected color (1, 0, 0, 1), got %+v", color)
	}

	if err := session.LogJSON(resource.InspectName, result); err != nil {
		t.Fatal(err)
	}
}

func TestResourceInspectMissingPropertyReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[resource.InspectParams, resource.InspectResult](
		session.Client,
		resource.InspectName,
		resource.InspectParams{
			Resource:   "res://cases/resource_inspect_succeeds/material.tres",
			Properties: []string{"not_a_property"},
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestResourceInspectMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[resource.InspectParams, resource.InspectResult](
		session.Client,
		resource.InspectName,
		resource.InspectParams{
			Resource: "res://cases/resource_inspect_missing/missing.tres",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestResourceSetSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_set_succeeds/material.tres"

	result := setResourceProperties(t, session, path, map[string]json.RawMessage{
		"albedo_color": json.RawMessage(`{"r":0,"g":0,"b":1,"a":1}`),
		"roughness":    json.RawMessage(`0.75`),
	})

	if !result.Changed {
		t.Fatal("expected resource.set to report a change")
	}

	if result.Type != "StandardMaterial3D" {
		t.Fatalf("expected type StandardMaterial3D, got %s", result.Type)
	}

	text := readResourceFile(t, session, path)

	for _, expected := range []string{
		"albedo_color = Color(0, 0, 1, 1)",
		"roughness = 0.75",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf(
				"expected the saved resource to contain %q:\n%s",
				expected,
				text,
			)
		}
	}

	if err := session.LogJSON(resource.SetName, result); err != nil {
		t.Fatal(err)
	}
}

func TestResourceSetReferenceSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_set_reference/atlas.tres"

	result := setResourceProperties(t, session, path, map[string]json.RawMessage{
		"atlas": json.RawMessage(`{"resource":"res://assets/placeholder_texture.tres"}`),
		"region": json.RawMessage(
			`{"position":{"x":0,"y":0},"size":{"x":8,"y":8}}`,
		),
	})

	if !result.Changed {
		t.Fatal("expected resource.set to report a change")
	}

	var atlas map[string]any

	if err := json.Unmarshal(result.Properties["atlas"], &atlas); err != nil {
		t.Fatalf("decoding atlas: %v", err)
	}

	if atlas["resource"] != "res://assets/placeholder_texture.tres" {
		t.Fatalf("expected the placeholder texture, got %v", atlas)
	}

	if atlas["type"] != "PlaceholderTexture2D" {
		t.Fatalf("expected type PlaceholderTexture2D, got %v", atlas)
	}

	text := readResourceFile(t, session, path)

	for _, expected := range []string{
		"res://assets/placeholder_texture.tres",
		"region = Rect2(0, 0, 8, 8)",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf(
				"expected the saved resource to contain %q:\n%s",
				expected,
				text,
			)
		}
	}
}

func TestResourceSetNoOpReportsNoChange(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_set_noop/material.tres"

	result := setResourceProperties(t, session, path, map[string]json.RawMessage{
		"roughness": json.RawMessage(`0.25`),
	})

	if result.Changed {
		t.Fatal("expected setting the current value to report no change")
	}
}

func TestResourceSetInvalidPropertyIsAtomic(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_set_atomic/material.tres"
	before := readResourceFile(t, session, path)

	_, err := bridge.Call[resource.SetParams, resource.SetResult](
		session.Client,
		resource.SetName,
		resource.SetParams{
			Resource: path,
			Properties: map[string]json.RawMessage{
				"roughness":        json.RawMessage(`0.9`),
				"missing_property": json.RawMessage(`1`),
			},
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)

	after := readResourceFile(t, session, path)

	if after != before {
		t.Fatalf(
			"expected the file to be untouched:\nbefore:\n%s\nafter:\n%s",
			before,
			after,
		)
	}
}

func TestResourceSetMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[resource.SetParams, resource.SetResult](
		session.Client,
		resource.SetName,
		resource.SetParams{
			Resource: "res://cases/resource_set_missing/missing.tres",
			Properties: map[string]json.RawMessage{
				"roughness": json.RawMessage(`0.1`),
			},
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestResourceDeleteSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_delete_succeeds/orphan.tres"

	result, err := bridge.Call[resource.DeleteParams, resource.DeleteResult](
		session.Client,
		resource.DeleteName,
		resource.DeleteParams{Resource: path},
	)
	if err != nil {
		t.Fatalf("expected resource.delete to succeed: %v", err)
	}

	if !result.Deleted {
		t.Fatal("expected resource.delete to report a deletion")
	}

	if result.Resource != path {
		t.Fatalf("expected resource %s, got %s", path, result.Resource)
	}

	if _, err := os.Stat(resourceFilesystemPath(session, path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be deleted, stat returned: %v", path, err)
	}

	if err := session.LogJSON(resource.DeleteName, result); err != nil {
		t.Fatal(err)
	}
}

func TestResourceDeleteLoadedSucceeds(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_delete_cached/cached.tres"

	inspectResource(t, session, resource.InspectParams{Resource: path})

	result, err := bridge.Call[resource.DeleteParams, resource.DeleteResult](
		session.Client,
		resource.DeleteName,
		resource.DeleteParams{Resource: path},
	)
	if err != nil {
		t.Fatalf("expected resource.delete to succeed: %v", err)
	}

	if !result.Deleted {
		t.Fatal("expected resource.delete to report a deletion")
	}

	if _, err := os.Stat(resourceFilesystemPath(session, path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be deleted, stat returned: %v", path, err)
	}
}

func TestResourceDeleteReferencedConflict(t *testing.T) {
	session := requireSuite(t)
	path := "res://cases/resource_delete_referenced/metal.tres"

	_, err := bridge.Call[resource.DeleteParams, resource.DeleteResult](
		session.Client,
		resource.DeleteName,
		resource.DeleteParams{Resource: path},
	)

	assertAPIErrorCode(t, err, apierr.CodeConflict)

	var apiError *apierr.Error

	if !errors.As(err, &apiError) {
		t.Fatalf("expected an API error, got %v", err)
	}

	if !strings.Contains(apiError.Message, "main.tscn") {
		t.Fatalf(
			"expected the conflict to name the referencing scene, got: %s",
			apiError.Message,
		)
	}

	if text := readResourceFile(t, session, path); !strings.Contains(text, "StandardMaterial3D") {
		t.Fatalf("expected the referenced resource to survive:\n%s", text)
	}
}

func TestResourceDeleteMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[resource.DeleteParams, resource.DeleteResult](
		session.Client,
		resource.DeleteName,
		resource.DeleteParams{
			Resource: "res://cases/resource_delete_missing/missing.tres",
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func createResource(
	t *testing.T,
	session *harness.Session,
	params resource.CreateParams,
) resource.CreateResult {
	t.Helper()

	result, err := bridge.Call[resource.CreateParams, resource.CreateResult](
		session.Client,
		resource.CreateName,
		params,
	)
	if err != nil {
		t.Fatalf("expected resource.create to succeed: %v", err)
	}

	return result
}

func inspectResource(
	t *testing.T,
	session *harness.Session,
	params resource.InspectParams,
) resource.InspectResult {
	t.Helper()

	result, err := bridge.Call[resource.InspectParams, resource.InspectResult](
		session.Client,
		resource.InspectName,
		params,
	)
	if err != nil {
		t.Fatalf("expected resource.inspect to succeed: %v", err)
	}

	return result
}

func setResourceProperties(
	t *testing.T,
	session *harness.Session,
	path string,
	properties map[string]json.RawMessage,
) resource.SetResult {
	t.Helper()

	result, err := bridge.Call[resource.SetParams, resource.SetResult](
		session.Client,
		resource.SetName,
		resource.SetParams{
			Resource:   path,
			Properties: properties,
		},
	)
	if err != nil {
		t.Fatalf("expected resource.set to succeed: %v", err)
	}

	return result
}

func readResourceFile(
	t *testing.T,
	session *harness.Session,
	path string,
) string {
	t.Helper()

	data, err := os.ReadFile(resourceFilesystemPath(session, path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(data)
}

func resourceFilesystemPath(session *harness.Session, path string) string {
	return filepath.Join(
		session.Root(),
		filepath.FromSlash(strings.TrimPrefix(path, "res://")),
	)
}
