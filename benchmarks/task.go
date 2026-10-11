package main

type taskDefinition struct {
	ID           string       `json:"id"`
	Goal         string       `json:"goal"`
	InitialScene string       `json:"initialScene"`
	Verify       verification `json:"verify"`
}

type verification struct {
	Scene      string         `json:"scene"`
	Node       string         `json:"node,omitempty"`
	Absent     bool           `json:"absent,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Nodes      []nodeCheck    `json:"nodes,omitempty"`
}

type nodeCheck struct {
	Node       string         `json:"node"`
	Absent     bool           `json:"absent,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

func (v verification) checks() []nodeCheck {
	checks := make([]nodeCheck, 0, 1+len(v.Nodes))

	if v.Node != "" {
		checks = append(checks, nodeCheck{
			Node:       v.Node,
			Absent:     v.Absent,
			Properties: v.Properties,
		})
	}

	return append(checks, v.Nodes...)
}
