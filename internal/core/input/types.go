// Package input inspects and edits the project's Input Map.
package input

const (
	// EventTypeKey is an InputEventKey, identified by keycode or physical_keycode.
	EventTypeKey = "key"
	// EventTypeMouseButton is an InputEventMouseButton.
	EventTypeMouseButton = "mouse_button"
	// EventTypeJoypadButton is an InputEventJoypadButton.
	EventTypeJoypadButton = "joypad_button"
	// EventTypeJoypadMotion is an InputEventJoypadMotion.
	EventTypeJoypadMotion = "joypad_motion"
)

// Event is a single Input Map binding. Type selects which fields apply:
//
//   - key: keycode, physical_keycode, shift, ctrl, alt, meta, echo
//   - mouse_button: button_index
//   - joypad_button: button_index
//   - joypad_motion: axis, axis_value
//
// Device applies to every type; -1 (the default) matches all devices.
type Event struct {
	Type            string   `json:"type" jsonschema:"enum=key,enum=mouse_button,enum=joypad_button,enum=joypad_motion,description=Binding kind."`
	Device          *int     `json:"device,omitempty" jsonschema:"description=Device index, or -1 for all devices. Defaults to -1."`
	Keycode         *int     `json:"keycode,omitempty" jsonschema:"description=Keyboard key constant such as 32 for Space, matched by key label."`
	PhysicalKeycode *int     `json:"physical_keycode,omitempty" jsonschema:"description=Keyboard key constant matched by physical position."`
	Shift           bool     `json:"shift,omitempty" jsonschema:"description=Require Shift."`
	Ctrl            bool     `json:"ctrl,omitempty" jsonschema:"description=Require Ctrl."`
	Alt             bool     `json:"alt,omitempty" jsonschema:"description=Require Alt."`
	Meta            bool     `json:"meta,omitempty" jsonschema:"description=Require Meta."`
	Echo            bool     `json:"echo,omitempty" jsonschema:"description=Match key echo repeats."`
	ButtonIndex     *int     `json:"button_index,omitempty" jsonschema:"description=Mouse button or joypad button constant such as 1 for the left mouse button."`
	Axis            *int     `json:"axis,omitempty" jsonschema:"description=Joypad axis constant such as 0 for the left stick X axis."`
	AxisValue       *float64 `json:"axis_value,omitempty" jsonschema:"description=Joypad axis direction, typically -1 or 1."`
}

// Action is one Input Map action and its bindings.
type Action struct {
	Name     string  `json:"name" jsonschema:"description=Action name such as jump."`
	Deadzone float64 `json:"deadzone" jsonschema:"description=Analog deadzone between 0 and 1."`
	Events   []Event `json:"events" jsonschema:"description=Bindings in their stored order; the index identifies an event for removal."`
}

type ListParams struct{}

type ListResult struct {
	Actions []Action `json:"actions"`
}

type InspectParams struct {
	Action string `json:"action" jsonschema:"minLength=1,description=Existing action name such as jump."`
}

type InspectResult = Action

type AddParams struct {
	Action   string   `json:"action" jsonschema:"minLength=1,description=New action name such as jump. Fails if the action already exists."`
	Deadzone *float64 `json:"deadzone,omitempty" jsonschema:"description=Analog deadzone between 0 and 1; defaults to 0.5."`
}

type AddResult struct {
	Action   string  `json:"action"`
	Deadzone float64 `json:"deadzone"`
	Events   []Event `json:"events"`
	Changed  bool    `json:"changed"`
}

type SetParams struct {
	Action   string  `json:"action" jsonschema:"minLength=1,description=Existing action name such as jump."`
	Deadzone float64 `json:"deadzone" jsonschema:"description=New analog deadzone between 0 and 1."`
}

type SetResult struct {
	Action   string  `json:"action"`
	Deadzone float64 `json:"deadzone"`
	Events   []Event `json:"events"`
	Changed  bool    `json:"changed" jsonschema:"description=Whether the action changed; false when the deadzone already matched."`
}

type RenameParams struct {
	Action  string `json:"action" jsonschema:"minLength=1,description=Existing action name to rename."`
	NewName string `json:"new_name" jsonschema:"minLength=1,description=New action name. Fails if an action with that name already exists."`
}

type RenameResult struct {
	Action   string  `json:"action" jsonschema:"description=Action name before the rename."`
	NewName  string  `json:"new_name"`
	Deadzone float64 `json:"deadzone"`
	Events   []Event `json:"events"`
	Changed  bool    `json:"changed"`
}

type DeleteParams struct {
	Action string `json:"action" jsonschema:"minLength=1,description=Existing action name to remove."`
}

type DeleteResult struct {
	Action  string `json:"action"`
	Deleted bool   `json:"deleted"`
}

type EventAddParams struct {
	Action string `json:"action" jsonschema:"minLength=1,description=Existing action name to add a binding to."`
	Event  Event  `json:"event" jsonschema:"description=Binding to add."`
}

type EventAddResult struct {
	Action  string `json:"action"`
	Index   int    `json:"index" jsonschema:"description=Zero-based position of the new binding."`
	Event   Event  `json:"event"`
	Changed bool   `json:"changed"`
}

type EventDeleteParams struct {
	Action string `json:"action" jsonschema:"minLength=1,description=Existing action name to remove a binding from."`
	Index  int    `json:"index" jsonschema:"minimum=0,description=Zero-based position of the binding to remove."`
}

type EventDeleteResult struct {
	Action  string  `json:"action"`
	Index   int     `json:"index" jsonschema:"description=Index that was removed."`
	Events  []Event `json:"events" jsonschema:"description=Remaining bindings."`
	Changed bool    `json:"changed"`
}
