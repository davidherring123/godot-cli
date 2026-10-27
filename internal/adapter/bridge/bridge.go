// Package bridge connects to the TCP bridge exposed by a running Godot editor.
package bridge

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/davidherring123/godot-mcp/internal/port/editorinstance"
	"github.com/davidherring123/godot-mcp/internal/protocol/apierr"
)

const HOST = "127.0.0.1"

const (
	connectTimeout = time.Second
	// requestTimeout bounds a single request/response exchange with the
	// editor. The addon handles one action at a time, so a stalled or crashed
	// editor must not block the caller forever.
	requestTimeout = 10 * time.Second
)

type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	timeout time.Duration
}

type Request[P any] struct {
	Action string `json:"action"`
	Params P      `json:"params"`
}

type Response[R any] struct {
	Result  R           `json:"result,omitempty"`
	Code    apierr.Code `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

func Success[R any](result R) Response[R] {
	return Response[R]{
		Result: result,
	}
}

func Failure[R any](code apierr.Code, message string) Response[R] {
	return Response[R]{
		Code:    code,
		Message: message,
	}
}

func Connect(projectRoot string) (*Client, error) {
	instance, err := FindInstance(projectRoot)
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort(
		HOST,
		strconv.Itoa(instance.Port),
	)

	conn, err := net.DialTimeout("tcp", address, connectTimeout)
	if err != nil {
		return nil, fmt.Errorf(
			"could not connect to Godot editor: %w",
			err,
		)
	}

	return &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
		timeout: requestTimeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// SetTimeout changes the maximum time a single Call may take. A non-positive
// timeout restores the default. It is intended for tests and callers that know
// their action is short-lived.
func (c *Client) SetTimeout(timeout time.Duration) {
	if timeout <= 0 {
		c.timeout = requestTimeout
		return
	}

	c.timeout = timeout
}

func Call[P any, R any](
	client *Client,
	action string,
	params P,
) (R, error) {
	var emptyResult R

	if client.timeout > 0 {
		if err := client.conn.SetDeadline(time.Now().Add(client.timeout)); err != nil {
			return emptyResult, fmt.Errorf(
				"failed to set request deadline: %w",
				err,
			)
		}
	}

	request := Request[P]{
		Action: action,
		Params: params,
	}

	if err := client.encoder.Encode(request); err != nil {
		return emptyResult, fmt.Errorf(
			"failed to send request: %w",
			err,
		)
	}

	var response Response[R]

	if err := client.decoder.Decode(&response); err != nil {
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			return emptyResult, fmt.Errorf(
				"timed out waiting for Godot editor after %s: %w",
				client.timeout,
				err,
			)
		}

		return emptyResult, fmt.Errorf(
			"failed to read response: %w",
			err,
		)
	}

	if response.Code != "" {
		return emptyResult, &apierr.Error{
			Code:    response.Code,
			Message: response.Message,
		}
	}

	return response.Result, nil
}

// FindInstance returns the single running Godot editor registered for the
// project. It returns editorinstance.ErrNoEditor when none is running and
// editorinstance.ErrMultipleEditors when more than one is.
func FindInstance(projectRoot string) (editorinstance.Instance, error) {
	instancesDir := filepath.Join(
		projectRoot,
		".godot",
		"godot-mcp",
		"instances",
	)

	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return editorinstance.Instance{}, editorinstance.ErrNoEditor
		}

		return editorinstance.Instance{}, err
	}

	var instances []editorinstance.Instance

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(instancesDir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var instance editorinstance.Instance

		if err := json.Unmarshal(data, &instance); err != nil {
			continue
		}

		// A crashed editor leaves its registration behind. Ignore entries
		// whose process is gone (or that never contained a usable address)
		// instead of reporting them as a running editor. Do not remove the
		// file: the PID could have been reused, or another process may be in
		// the middle of replacing it.
		if instance.PID <= 0 || instance.Port <= 0 || !processAlive(instance.PID) {
			continue
		}

		instances = append(instances, instance)
	}

	if len(instances) == 0 {
		return editorinstance.Instance{}, editorinstance.ErrNoEditor
	}

	if len(instances) > 1 {
		return editorinstance.Instance{}, editorinstance.ErrMultipleEditors
	}

	return instances[0], nil
}
