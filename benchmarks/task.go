package main

type taskDefinition struct {
	ID           string       `json:"id"`
	Goal         string       `json:"goal"`
	InitialScene string       `json:"initialScene"`
	Verify       verification `json:"verify"`
}

type verification struct {
	Scene string      `json:"scene"`
	Nodes []nodeCheck `json:"nodes"`
}

type nodeCheck struct {
	Node       string         `json:"node"`
	Absent     bool           `json:"absent,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}
