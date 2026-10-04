package scene

type ListParams struct{}

type ListResult struct {
	Scenes []string `json:"scenes" jsonschema:"nullable,description=Project scene paths with the res:// prefix; null when no scenes are found."`
}

type TreeParams struct {
	Scene string `json:"scene" jsonschema:"minLength=1,description=Scene resource path such as res://main.tscn."`
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
