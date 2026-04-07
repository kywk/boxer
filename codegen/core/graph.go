package core

import (
	"fmt"

	"github.com/boxer/codegen/ir"
)

// NodeInfo holds a node with its resolved graph relationships.
type NodeInfo struct {
	Node        *ir.Node
	NextID      string   // for linear nodes (non-branching)
	TrueBranch  string   // condition: true handle
	FalseBranch string   // condition: false handle
	CaseBranch  []string // switch: case:0, case:1, ...
	DefaultID   string   // switch: default handle
	Branches    []string // fork: all outgoing targets
	JoinID      string   // fork: downstream join node
	InputVars   []string // join: outputVars from incoming nodes
}

// AnalyzeGraph builds a map of NodeInfo from the IR.
func AnalyzeGraph(flow *ir.GatewayIR) (map[string]*NodeInfo, []string, error) {
	nodeMap := make(map[string]*ir.Node, len(flow.Nodes))
	for i := range flow.Nodes {
		nodeMap[flow.Nodes[i].ID] = &flow.Nodes[i]
	}

	infos := make(map[string]*NodeInfo, len(flow.Nodes))
	for i := range flow.Nodes {
		n := &flow.Nodes[i]
		info := &NodeInfo{Node: n}

		switch n.Type {
		case "condition":
			for _, e := range flow.Edges {
				if e.Source != n.ID {
					continue
				}
				switch e.SourceHandle {
				case "true":
					info.TrueBranch = e.Target
				case "false":
					info.FalseBranch = e.Target
				}
			}

		case "switch":
			cases := n.GetStringSlice("cases")
			info.CaseBranch = make([]string, len(cases))
			for _, e := range flow.Edges {
				if e.Source != n.ID {
					continue
				}
				if e.SourceHandle == "default" {
					info.DefaultID = e.Target
					continue
				}
				// parse "case:N"
				var idx int
				if _, err := fmt.Sscanf(e.SourceHandle, "case:%d", &idx); err == nil && idx < len(cases) {
					info.CaseBranch[idx] = e.Target
				}
			}

		case "fork":
			for _, e := range flow.Edges {
				if e.Source == n.ID {
					info.Branches = append(info.Branches, e.Target)
				}
			}
			// find downstream join
			for _, branchID := range info.Branches {
				for _, e := range flow.Edges {
					if e.Source == branchID {
						if target, ok := nodeMap[e.Target]; ok && target.Type == "join" {
							info.JoinID = e.Target
							break
						}
					}
				}
				if info.JoinID != "" {
					break
				}
			}

		case "join":
			for _, e := range flow.Edges {
				if e.Target == n.ID {
					if src, ok := nodeMap[e.Source]; ok && src.OutputVar != "" {
						info.InputVars = append(info.InputVars, src.OutputVar)
					}
				}
			}
			// next after join
			for _, e := range flow.Edges {
				if e.Source == n.ID {
					info.NextID = e.Target
					break
				}
			}

		default:
			// linear: http-call, transform, response, sub-flow
			for _, e := range flow.Edges {
				if e.Source == n.ID && e.SourceHandle == "" {
					info.NextID = e.Target
					break
				}
			}
		}

		infos[n.ID] = info
	}

	order, err := topoSort(flow)
	if err != nil {
		return nil, nil, err
	}

	return infos, order, nil
}

// Prerequisites returns the list of upstream names used in the flow.
func Prerequisites(flow *ir.GatewayIR) []string {
	seen := map[string]bool{}
	var result []string
	for _, n := range flow.Nodes {
		if n.Type == "http-call" {
			u := n.GetUpstream()
			if u.Name != "" && !seen[u.Name] {
				seen[u.Name] = true
				result = append(result, u.Name)
			}
		}
	}
	return result
}

// topoSort returns nodes in topological order.
func topoSort(flow *ir.GatewayIR) ([]string, error) {
	inDegree := make(map[string]int, len(flow.Nodes))
	for _, n := range flow.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range flow.Edges {
		inDegree[e.Target]++
	}

	var queue []string
	for _, n := range flow.Nodes {
		if inDegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, e := range flow.Edges {
			if e.Source == id {
				inDegree[e.Target]--
				if inDegree[e.Target] == 0 {
					queue = append(queue, e.Target)
				}
			}
		}
	}

	if len(order) != len(flow.Nodes) {
		return nil, fmt.Errorf("cycle detected in flow graph")
	}
	return order, nil
}
