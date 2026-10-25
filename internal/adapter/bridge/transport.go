package bridge

import "encoding/json"

// Transport adapts the TCP bridge to invocation.Transport.
type Transport struct{}

// Invoke sends one action request and returns the encoded result.
func (Transport) Invoke(
	projectRoot string,
	action string,
	params any,
) (json.RawMessage, error) {
	client, err := Connect(projectRoot)
	if err != nil {
		return nil, err
	}

	defer func() { _ = client.Close() }()

	return Call[any, json.RawMessage](client, action, params)
}
