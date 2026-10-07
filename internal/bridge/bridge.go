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
	instance, err := findInstance(projectRoot)
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort(
		HOST,
		strconv.Itoa(instance.Port),
	)

	conn, err := net.DialTimeout("tcp", address, time.Second)
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
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func Call[P any, R any](
	client *Client,
	action string,
	params P,
) (R, error) {
	var emptyResult R

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

func findInstance(projectRoot string) (Instance, error) {
	instancesDir := filepath.Join(
		projectRoot,
		".godot",
		"godot-cli",
		"instances",
	)

	entries, err := os.ReadDir(instancesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Instance{}, errors.New(
				"no running Godot editor found",
			)
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

		instances = append(instances, instance)
	}

	if len(instances) == 0 {
		return Instance{}, errors.New(
			"no running Godot editor found",
		)
	}

	if len(instances) > 1 {
		return Instance{}, errors.New(
			"multiple Godot editor instances found",
		)
	}

	return instances[0], nil
}
