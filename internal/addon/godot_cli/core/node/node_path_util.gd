class_name NodePathUtil
extends RefCounted

# NodePath does not expose rel_path_to, path_join, or simplify to scripting,
# so these helpers mirror the engine behavior using name arrays.

static func resolve(owner_path: NodePath, path: NodePath) -> NodePath:
    if path.is_absolute():
        return NodePath("/" + str(path.get_concatenated_names()))

    var names: Array[String] = []

    for index in owner_path.get_name_count():
        names.append(String(owner_path.get_name(index)))

    for index in path.get_name_count():
        var name := String(path.get_name(index))

        if name == "..":
            if not names.is_empty():
                names.pop_back()
        elif name != ".":
            names.append(name)

    return NodePath("/" + "/".join(names))

static func relative(from_path: NodePath, to_path: NodePath) -> NodePath:
    var from_names: Array[String] = []
    var to_names: Array[String] = []

    for index in from_path.get_name_count():
        from_names.append(String(from_path.get_name(index)))

    for index in to_path.get_name_count():
        to_names.append(String(to_path.get_name(index)))

    var common := 0

    while common < from_names.size() and common < to_names.size():
        if from_names[common] != to_names[common]:
            break

        common += 1

    if common == 0:
        return to_path

    var parts: Array[String] = []

    for index in range(common, from_names.size()):
        parts.append("..")

    for index in range(common, to_names.size()):
        parts.append(to_names[index])

    var text := "."

    if not parts.is_empty():
        text = "/".join(parts)

    if to_path.get_subname_count() > 0:
        text += ":" + str(to_path.get_concatenated_subnames())

    return NodePath(text)

static func with_last_name(path: NodePath, new_name: String) -> NodePath:
    var names: Array[String] = []

    for index in path.get_name_count():
        names.append(String(path.get_name(index)))

    if names.is_empty():
        return path

    names[names.size() - 1] = new_name

    var text := "/" + "/".join(names)

    if path.get_subname_count() > 0:
        text += ":" + str(path.get_concatenated_subnames())

    return NodePath(text)
