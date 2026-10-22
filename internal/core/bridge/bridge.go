// Package bridge connects to the TCP bridge exposed by a running Godot editor.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const HOST = "127.0.0.1"

const (
	connectTimeout = time.Second
	// requestTimeout bounds a single request/response exchange with the
	// editor. The addon handles one action at a time, so a stalled or crashed
	// editor must not block the caller forever.
	requestTimeout = 10 * time.Second
)

var (
	// ErrNoEditor reports that no running Godot editor is registered for the
	// project.
	ErrNoEditor = errors.New("no running Godot editor found")

	// ErrMultipleEditors reports that more than one editor is registered for
	// the project.
	ErrMultipleEditors = errors.New("multiple Godot editor instances found")
)

type Instance struct {
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	Project string `json:"project"`
	Version string `json:"version"`
}

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
	Result  R         `json:"result,omitempty"`
	Code    ErrorCode `json:"code,omitempty"`
	Message string    `json:"message,omitempty"`
}

func Success[R any](result R) Response[R] {
	return Response[R]{
		Result: result,
	}
}

func Failure[R any](code ErrorCode, message string) Response[R] {
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
		return emptyResult, &APIError{
			Code:    response.Code,
			Message: response.Message,
		}
	}

	return response.Result, nil
}

// FindInstance returns the single running Godot editor registered for the
// project. It returns ErrNoEditor when none is running and ErrMultipleEditors
// when more than one is.
func FindInstance(projectRoot string) (Instance, error) {
	instancesDir := filepath.Join(
		projectRoot,
		".godot",
		"godot-cli",
		"instances",
	)

	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Instance{}, ErrNoEditor
		}

		return Instance{}, err
	}

	var instances []Instance

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(instancesDir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var instance Instance

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
		return Instance{}, ErrNoEditor
	}

	if len(instances) > 1 {
		return Instance{}, ErrMultipleEditors
	}

	return instances[0], nil
}
