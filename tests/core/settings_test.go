package core_test

import (
	"encoding/json"
	"testing"

	"github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/core/apierr"
	"github.com/davidherring123/godot-cli/internal/core/project/settings"
	"github.com/davidherring123/godot-cli/tests/harness"
)

const (
	viewportWidthSetting = "display/window/size/viewport_width"
	projectNameSetting   = "application/config/name"
)

func TestProjectListSucceeds(t *testing.T) {
	session := requireSuite(t)

	result := listProjectSettings(t, session, settings.ListParams{
		Prefix: "display/window/size",
	})

	if len(result.Settings) == 0 {
		t.Fatal("expected at least one display/window/size setting")
	}

	setting := findSetting(t, result.Settings, viewportWidthSetting)

	if setting.Type != "int" {
		t.Fatalf("expected type int, got %s", setting.Type)
	}

	if !setting.Overridden {
		t.Fatal("expected the fixture to override viewport_width")
	}

	if value := decodeNumber(t, setting.Value); value != 800 {
		t.Fatalf("expected value 800, got %v", value)
	}

	if value := decodeNumber(t, setting.Default); value != 1152 {
		t.Fatalf("expected default 1152, got %v", value)
	}

	if err := session.LogJSON(settings.ListName, result); err != nil {
		t.Fatal(err)
	}
}

func TestProjectListUnknownPrefixIsEmpty(t *testing.T) {
	session := requireSuite(t)

	result := listProjectSettings(t, session, settings.ListParams{
		Prefix: "godot_cli/does_not_exist",
	})

	if len(result.Settings) != 0 {
		t.Fatalf("expected no settings, got %d", len(result.Settings))
	}
}

func TestProjectGetSucceeds(t *testing.T) {
	session := requireSuite(t)

	result := getProjectSetting(t, session, viewportWidthSetting)

	if result.Name != viewportWidthSetting {
		t.Fatalf("expected %s, got %s", viewportWidthSetting, result.Name)
	}

	if !result.Overridden {
		t.Fatal("expected the fixture to override viewport_width")
	}

	if value := decodeNumber(t, result.Value); value != 800 {
		t.Fatalf("expected value 800, got %v", value)
	}

	if err := session.LogJSON(settings.GetName, result); err != nil {
		t.Fatal(err)
	}
}

func TestProjectGetMissingReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[
		settings.GetParams,
		settings.GetResult,
	](
		session.Client,
		settings.GetName,
		settings.GetParams{Setting: "godot_cli/missing"},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestProjectSetAndUnsetRoundTrip(t *testing.T) {
	session := requireSuite(t)

	original := getProjectSetting(t, session, viewportWidthSetting)

	t.Cleanup(func() {
		if original.Overridden {
			_, _ = bridge.Call[
				settings.SetParams,
				settings.SetResult,
			](
				session.Client,
				settings.SetName,
				settings.SetParams{
					Setting: viewportWidthSetting,
					Value:   original.Value,
				},
			)
		} else {
			_, _ = bridge.Call[
				settings.UnsetParams,
				settings.UnsetResult,
			](
				session.Client,
				settings.UnsetName,
				settings.UnsetParams{Setting: viewportWidthSetting},
			)
		}
	})

	updated := setProjectSetting(t, session, viewportWidthSetting, json.RawMessage(`1024`))

	if !updated.Changed {
		t.Fatal("expected project.settings.set to report a change")
	}

	if !updated.Overridden {
		t.Fatal("expected the setting to be overridden after set")
	}

	if value := decodeNumber(t, updated.Value); value != 1024 {
		t.Fatalf("expected value 1024, got %v", value)
	}

	fetched := getProjectSetting(t, session, viewportWidthSetting)

	if value := decodeNumber(t, fetched.Value); value != 1024 {
		t.Fatalf("expected fetched value 1024, got %v", value)
	}

	removed := unsetProjectSetting(t, session, viewportWidthSetting)

	if !removed.Changed {
		t.Fatal("expected project.settings.unset to report a change")
	}

	if removed.Overridden {
		t.Fatal("expected the setting to fall back to the engine default")
	}

	if value := decodeNumber(t, removed.Value); value != 1152 {
		t.Fatalf("expected default 1152 after unset, got %v", value)
	}
}

func TestProjectSetNoOpReportsNoChange(t *testing.T) {
	session := requireSuite(t)

	result := setProjectSetting(t, session, viewportWidthSetting, json.RawMessage(`800`))

	if result.Changed {
		t.Fatal("expected setting the current value to report no change")
	}
}

func TestProjectSetInvalidTypeRejected(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[
		settings.SetParams,
		settings.SetResult,
	](
		session.Client,
		settings.SetName,
		settings.SetParams{
			Setting: viewportWidthSetting,
			Value:   json.RawMessage(`"not-a-number"`),
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeInvalidArgument)
}

func TestProjectSetUnknownReturnsNotFound(t *testing.T) {
	session := requireSuite(t)

	_, err := bridge.Call[
		settings.SetParams,
		settings.SetResult,
	](
		session.Client,
		settings.SetName,
		settings.SetParams{
			Setting: "godot_cli/missing",
			Value:   json.RawMessage(`1`),
		},
	)

	assertAPIErrorCode(t, err, apierr.CodeNotFound)
}

func TestProjectUnsetNotOverriddenReportsNoChange(t *testing.T) {
	session := requireSuite(t)

	result := unsetProjectSetting(t, session, "display/window/size/viewport_height")

	if result.Changed {
		t.Fatal("expected unsetting a non-overridden setting to report no change")
	}
}

func listProjectSettings(
	t *testing.T,
	session *harness.Session,
	params settings.ListParams,
) settings.ListResult {
	t.Helper()

	result, err := bridge.Call[
		settings.ListParams,
		settings.ListResult,
	](session.Client, settings.ListName, params)
	if err != nil {
		t.Fatalf("expected project.settings.settings.list to succeed: %v", err)
	}

	return result
}

func getProjectSetting(
	t *testing.T,
	session *harness.Session,
	name string,
) settings.GetResult {
	t.Helper()

	result, err := bridge.Call[
		settings.GetParams,
		settings.GetResult,
	](
		session.Client,
		settings.GetName,
		settings.GetParams{Setting: name},
	)
	if err != nil {
		t.Fatalf("expected project.settings.settings.get to succeed: %v", err)
	}

	return result
}

func setProjectSetting(
	t *testing.T,
	session *harness.Session,
	name string,
	value json.RawMessage,
) settings.SetResult {
	t.Helper()

	result, err := bridge.Call[
		settings.SetParams,
		settings.SetResult,
	](
		session.Client,
		settings.SetName,
		settings.SetParams{Setting: name, Value: value},
	)
	if err != nil {
		t.Fatalf("expected project.settings.set to succeed: %v", err)
	}

	return result
}

func unsetProjectSetting(
	t *testing.T,
	session *harness.Session,
	name string,
) settings.UnsetResult {
	t.Helper()

	result, err := bridge.Call[
		settings.UnsetParams,
		settings.UnsetResult,
	](
		session.Client,
		settings.UnsetName,
		settings.UnsetParams{Setting: name},
	)
	if err != nil {
		t.Fatalf("expected project.settings.unset to succeed: %v", err)
	}

	return result
}

func findSetting(
	t *testing.T,
	values []settings.Setting,
	name string,
) settings.Setting {
	t.Helper()

	for _, setting := range values {
		if setting.Name == name {
			return setting
		}
	}

	t.Fatalf("expected to find setting %s", name)

	return settings.Setting{}
}

func decodeNumber(t *testing.T, value json.RawMessage) float64 {
	t.Helper()

	var number float64

	if err := json.Unmarshal(value, &number); err != nil {
		t.Fatalf("decoding number %s: %v", value, err)
	}

	return number
}
