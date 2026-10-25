// Package invocation is the transport-neutral boundary for action calls.
//
// Domain packages call Call with typed parameters and results. A process-wide
// Transport is installed once by the composition root (internal/compose), so
// domains never import a concrete adapter such as the TCP editor bridge.
package invocation

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoTransport reports that no editor transport has been installed.
var ErrNoTransport = errors.New("no editor transport configured")

// Transport performs one action request/response exchange for a project.
// Implementations are transport-specific and must return result JSON that can
// be decoded into the caller's result type.
type Transport interface {
	Invoke(projectRoot string, action string, params any) (json.RawMessage, error)
}

// TransportFunc adapts a function to Transport.
type TransportFunc func(
	projectRoot string,
	action string,
	params any,
) (json.RawMessage, error)

func (fn TransportFunc) Invoke(
	projectRoot string,
	action string,
	params any,
) (json.RawMessage, error) {
	return fn(projectRoot, action, params)
}

var transport Transport

// SetTransport installs the process-wide transport. Passing nil restores the
// unconfigured state and is intended for tests.
func SetTransport(next Transport) {
	transport = next
}

// Call invokes one action and decodes its result.
func Call[P any, R any](
	projectRoot string,
	action string,
	params P,
) (R, error) {
	var result R

	if transport == nil {
		return result, ErrNoTransport
	}

	encoded, err := transport.Invoke(projectRoot, action, params)
	if err != nil {
		return result, err
	}

	if len(encoded) == 0 {
		return result, nil
	}

	if err := json.Unmarshal(encoded, &result); err != nil {
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	return result, nil
}
