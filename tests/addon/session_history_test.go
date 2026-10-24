package addon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSceneEditSessionDefersHistoryAndUndoesInOrder exercises the session
// adapter without an editor. It verifies that no history action is created
// until every operation succeeds and that the one committed action undoes and
// redoes the whole batch in the right order.
func TestSceneEditSessionDefersHistoryAndUndoesInOrder(t *testing.T) {
	godot := findGodot(t)
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("config_version=5\n\n[application]\n\nconfig/name=\"SessionTest\"\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}

	source := filepath.Join(
		filepath.Dir(file),
		"..",
		"..",
		"internal",
		"addon",
		"godot_cli",
		"core",
		"scene",
		"scene_edit_session.gd",
	)

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("reading scene edit session: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "addon_session.gd"),
		data,
		0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "session_history_test.gd"),
		[]byte(sessionHistoryScript),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(
		godot,
		"--headless",
		"--path",
		root,
		"--script",
		"res://session_history_test.gd",
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("session history script failed: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "SESSION_HISTORY_OK") {
		t.Fatalf("session history script did not report success:\n%s", output)
	}
}

func findGodot(t *testing.T) string {
	t.Helper()

	for _, candidate := range []string{
		os.Getenv("GODOT_BIN"),
		"godot",
		"godot4",
	} {
		if candidate == "" {
			continue
		}

		path, err := exec.LookPath(candidate)
		if err == nil {
			return path
		}
	}

	t.Skip("godot executable not found; set GODOT_BIN to run addon tests")
	return ""
}

const sessionHistoryScript = `extends SceneTree

const Session = preload("res://addon_session.gd")

class FakeHistory:
    extends RefCounted

    const NO_ARG := &"__godot_cli_no_arg__"

    var undo_redo := UndoRedo.new()
    var create_count := 0

    func create_action(
        name: String,
        merge_mode: int,
        _custom_context = null,
        backward_undo_ops := true
    ) -> void:
        create_count += 1
        undo_redo.create_action(name, merge_mode, backward_undo_ops)

    func commit_action(execute := true) -> void:
        undo_redo.commit_action(execute)

    func add_do_method(
        object: Object,
        method: StringName,
        arg1 = NO_ARG,
        arg2 = NO_ARG,
        arg3 = NO_ARG,
        arg4 = NO_ARG
    ) -> void:
        undo_redo.add_do_method(
            Callable(object, method).bindv(_arguments(arg1, arg2, arg3, arg4))
        )

    func add_undo_method(
        object: Object,
        method: StringName,
        arg1 = NO_ARG,
        arg2 = NO_ARG,
        arg3 = NO_ARG,
        arg4 = NO_ARG
    ) -> void:
        undo_redo.add_undo_method(
            Callable(object, method).bindv(_arguments(arg1, arg2, arg3, arg4))
        )

    func add_do_property(object: Object, property: StringName, value: Variant) -> void:
        undo_redo.add_do_property(object, property, value)

    func add_undo_property(object: Object, property: StringName, value: Variant) -> void:
        undo_redo.add_undo_property(object, property, value)

    func add_do_reference(object: Object) -> void:
        undo_redo.add_do_reference(object)

    func add_undo_reference(object: Object) -> void:
        undo_redo.add_undo_reference(object)

    func _arguments(arg1, arg2, arg3, arg4) -> Array:
        var result: Array = []

        for argument in [arg1, arg2, arg3, arg4]:
            if typeof(argument) == TYPE_STRING_NAME and argument == NO_ARG:
                break

            result.append(argument)

        return result

var failures: Array[String] = []

func _initialize() -> void:
    var root := Node2D.new()
    root.name = "Root"

    var a := Node2D.new()
    a.name = "A"
    root.add_child(a)

    var b := Node2D.new()
    b.name = "B"
    root.add_child(b)

    var history := FakeHistory.new()
    var session := Session.new(history, "test batch", root)

    var c := Node2D.new()
    c.name = "C"

    session.create_action("add", UndoRedo.MERGE_DISABLE, root, true)
    session.add_do_method(a, "add_child", c, true)
    session.add_do_reference(c)
    session.add_undo_method(a, "remove_child", c)
    session.commit_action()

    session.create_action("move", UndoRedo.MERGE_DISABLE, root, false)
    session.add_do_method(b, "reparent", c, true)
    session.add_do_method(c, "move_child", b, 0)
    session.add_undo_method(b, "reparent", root, true)
    session.add_undo_method(root, "move_child", b, 1)
    session.commit_action()

    _check(history.create_count == 0, "history was registered before finish")
    _check(root.get_node_or_null("A/C/B") != null, "do methods did not run in order")

    session.finish()

    _check(history.create_count == 1, "expected exactly one editor action")
    _check(history.undo_redo.get_history_count() == 1, "expected exactly one undo step")

    history.undo_redo.undo()

    _check(root.get_node_or_null("A/C") == null, "undo left the added node behind")
    _check(b.get_parent() == root, "undo did not restore the moved node's parent")
    _check(b.get_index() == 1, "undo did not restore the moved node's index")

    history.undo_redo.redo()

    _check(root.get_node_or_null("A/C/B") != null, "redo did not replay the batch in order")

    var rollback_node := Node2D.new()
    rollback_node.name = "RollbackNode"

    var rollback_session := Session.new(history, "rollback batch", root)
    rollback_session.create_action("add", UndoRedo.MERGE_DISABLE, root, true)
    rollback_session.add_do_method(root, "add_child", rollback_node, true)
    rollback_session.add_do_reference(rollback_node)
    rollback_session.add_undo_method(root, "remove_child", rollback_node)
    rollback_session.commit_action()

    var complete_rollback: Dictionary = rollback_session.rollback()

    _check(complete_rollback["ok"], "complete rollback was reported as incomplete")
    _check(complete_rollback["restored"] == 1, "complete rollback restored count was wrong")
    _check(complete_rollback["total"] == 1, "references inflated the undo total")
    _check(complete_rollback["failures"].is_empty(), "complete rollback reported failures")
    _check(not is_instance_valid(rollback_node), "detached created node was not freed")

    var apply_note_session := Session.new(history, "apply note batch", root)
    var freed_target := Node2D.new()

    apply_note_session.create_action("partial", UndoRedo.MERGE_DISABLE, root, true)
    apply_note_session.add_do_method(root, "set_name", "ChangedRoot")
    apply_note_session.add_do_method(freed_target, "set_name", "NeverRuns")
    apply_note_session.add_undo_method(root, "set_name", "Root")
    freed_target.free()
    apply_note_session.commit_action()

    var noted_rollback: Dictionary = apply_note_session.rollback()

    _check(noted_rollback["ok"], "apply note was treated as an undo failure")
    _check(noted_rollback["restored"] == 1, "noted rollback restored count was wrong")
    _check(noted_rollback["total"] == 1, "noted rollback total was wrong")
    _check(noted_rollback["notes"].size() == 1, "apply note detail was not exposed")
    _check(noted_rollback["failures"].is_empty(), "noted rollback reported undo failures")
    _check(root.name == "Root", "noted rollback did not restore the applied change")

    var victim := Node2D.new()
    victim.name = "Victim"
    root.add_child(victim)

    var incomplete_session := Session.new(history, "incomplete batch", root)
    incomplete_session.create_action("rename", UndoRedo.MERGE_DISABLE, root, false)
    incomplete_session.add_do_method(victim, "set_name", "Changed")
    incomplete_session.add_undo_method(victim, "set_name", "Victim")
    incomplete_session.commit_action()

    victim.free()

    var rollback: Dictionary = incomplete_session.rollback()

    _check(not rollback["ok"], "incomplete rollback was reported as complete")
    _check(rollback["restored"] == 0, "incomplete rollback reported restored steps")
    _check(rollback["total"] == 1, "incomplete rollback reported the wrong total")
    _check(rollback["notes"].is_empty(), "incomplete rollback reported apply notes")
    _check(rollback["failures"].size() == 1, "undo failure detail was not exposed")
    _check(
        "no longer valid" in rollback["failures"][0],
        "undo failure did not explain the invalid object"
    )

    if failures.is_empty():
        print("SESSION_HISTORY_OK")
        quit(0)
        return

    for failure in failures:
        print("SESSION_HISTORY_FAILURE: %s" % failure)

    quit(1)

func _check(condition: bool, message: String) -> void:
    if not condition:
        failures.append(message)
`
