package scene

type ListParams struct{}

type ListResult struct {
	Scenes []string `json:"scenes" jsonschema:"nullable,description=Project scene paths with the res:// prefix; null when no scenes are found."`
}

type TreeParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type CurrentParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type CurrentResult struct {
	Scene   string `json:"scene" jsonschema:"description=Active scene resource path; empty for a scene never saved."`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Unsaved bool   `json:"unsaved"`
}

type OpenParams struct {
	Scene string `json:"scene" jsonschema:"minLength=1,description=Existing scene resource path such as res://main.tscn."`
}

type OpenResult = CurrentResult

type SaveParams struct {
	ExpectedScene string `json:"expected_scene,omitempty" jsonschema:"description=Fail if the active scene path differs; does not switch scenes."`
}

type SaveResult struct {
	Scene string `json:"scene"`
	Saved bool   `json:"saved"`
}

type TreeResult struct {
	Scene string   `json:"scene"`
	Root  TreeNode `json:"root"`
}

type TreeNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Path     string     `json:"path" jsonschema:"description=Node path relative to the scene root; the root is a dot."`
	Children []TreeNode `json:"children"`
}
