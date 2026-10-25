package input

import (
	"fmt"
	"math"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

func call[P any, R any](projectRoot, name string, params P) (R, error) {
	var empty R

	client, err := bridge.Connect(projectRoot)
	if err != nil {
		return empty, err
	}

	defer func() { _ = client.Close() }()

	return bridge.Call[P, R](client, name, params)
}

func validationError(message string) *bridge.APIError {
	return &bridge.APIError{
		Code:    bridge.ErrorCodeInvalidArgument,
		Message: message,
	}
}

func validateAction(action string) error {
	if action == "" {
		return validationError("An action name is required")
	}

	return nil
}

func validateDeadzone(deadzone float64) error {
	if math.IsNaN(deadzone) || math.IsInf(deadzone, 0) || deadzone < 0 || deadzone > 1 {
		return validationError("Deadzone must be between 0 and 1")
	}

	return nil
}

func validateEvent(event Event) error {
	switch event.Type {
	case EventTypeKey:
		keycodeMissing := event.Keycode == nil || *event.Keycode == 0
		physicalMissing := event.PhysicalKeycode == nil || *event.PhysicalKeycode == 0

		if keycodeMissing && physicalMissing {
			return validationError(
				"A key binding requires a keycode or physical_keycode",
			)
		}
	case EventTypeMouseButton, EventTypeJoypadButton:
		if event.ButtonIndex == nil {
			return validationError("A button binding requires a button_index")
		}
	case EventTypeJoypadMotion:
		if event.Axis == nil || event.AxisValue == nil {
			return validationError(
				"A joypad motion binding requires an axis and axis_value",
			)
		}

		if math.IsNaN(*event.AxisValue) || math.IsInf(*event.AxisValue, 0) {
			return validationError("axis_value must be finite")
		}
	default:
		return validationError(
			fmt.Sprintf("Unsupported event type: %s", event.Type),
		)
	}

	return nil
}
