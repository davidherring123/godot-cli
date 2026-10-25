package invocation_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidherring123/godot-cli/internal/port/invocation"
	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

type testParams struct {
	Name string `json:"name"`
}

type testResult struct {
	Echo string `json:"echo"`
}

func TestCallRoundTripsTypedParamsAndResult(t *testing.T) {
	var (
		gotRoot   string
		gotAction string
		gotParams testParams
	)

	invocation.SetTransport(invocation.TransportFunc(func(
		projectRoot string,
		action string,
		params any,
	) (json.RawMessage, error) {
		gotRoot = projectRoot
		gotAction = action
		gotParams = params.(testParams)

		return json.RawMessage(`{"echo":"pong"}`), nil
	}))

	t.Cleanup(func() { invocation.SetTransport(nil) })

	result, err := invocation.Call[testParams, testResult](
		"/project",
		"test.ping",
		testParams{Name: "ping"},
	)
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotRoot != "/project" || gotAction != "test.ping" || gotParams.Name != "ping" {
		t.Fatalf(
			"transport got root=%q action=%q params=%+v",
			gotRoot,
			gotAction,
			gotParams,
		)
	}

	if result.Echo != "pong" {
		t.Fatalf("result = %+v; want echo pong", result)
	}
}

func TestCallPropagatesAPIError(t *testing.T) {
	want := &apierr.Error{
		Code:    apierr.CodeConflict,
		Message: "node already exists",
	}

	invocation.SetTransport(invocation.TransportFunc(func(
		string,
		string,
		any,
	) (json.RawMessage, error) {
		return nil, want
	}))

	t.Cleanup(func() { invocation.SetTransport(nil) })

	_, err := invocation.Call[testParams, testResult](
		"/project",
		"node.add",
		testParams{},
	)
	if err == nil {
		t.Fatal("Call returned nil error; want an API error")
	}

	var apiError *apierr.Error
	if !errors.As(err, &apiError) {
		t.Fatalf("error %v is not an API error", err)
	}

	if apiError.Code != apierr.CodeConflict || apiError.Message != want.Message {
		t.Fatalf("error = %+v; want %+v", apiError, want)
	}

	if err.Error() != "conflict: node already exists" {
		t.Fatalf("error string = %q", err.Error())
	}
}

func TestCallWithoutTransport(t *testing.T) {
	invocation.SetTransport(nil)

	_, err := invocation.Call[testParams, testResult](
		"/project",
		"scene.list",
		testParams{},
	)
	if !errors.Is(err, invocation.ErrNoTransport) {
		t.Fatalf("error = %v; want ErrNoTransport", err)
	}
}

func TestCallRejectsMalformedResult(t *testing.T) {
	invocation.SetTransport(invocation.TransportFunc(func(
		string,
		string,
		any,
	) (json.RawMessage, error) {
		return json.RawMessage(`not json`), nil
	}))

	t.Cleanup(func() { invocation.SetTransport(nil) })

	_, err := invocation.Call[testParams, testResult](
		"/project",
		"scene.list",
		testParams{},
	)
	if err == nil {
		t.Fatal("Call returned nil error; want a decoding error")
	}

	if got := err.Error(); !strings.Contains(got, "failed to read response:") {
		t.Fatalf("error = %q", got)
	}
}

func TestAPIErrorCodesAreStable(t *testing.T) {
	cases := map[apierr.Code]string{
		apierr.CodeInvalidRequest:     "invalid_request",
		apierr.CodeInvalidArgument:    "invalid_argument",
		apierr.CodeNotFound:           "not_found",
		apierr.CodeFailedPrecondition: "failed_precondition",
		apierr.CodeConflict:           "conflict",
		apierr.CodeBusy:               "busy",
		apierr.CodeUnsupported:        "unsupported",
		apierr.CodeActionFailed:       "action_failed",
		apierr.CodeInternalError:      "internal_error",
	}

	for code, want := range cases {
		if string(code) != want {
			t.Errorf("code = %q; want %q", code, want)
		}
	}
}
