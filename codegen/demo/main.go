// demo/main.go — End-to-end demo: IR → codegen → live HTTP server
//
// Usage:
//   go run ./demo -input testdata/flow-user-profile.json -port 9090
//
// Test:
//   curl "http://localhost:9090/api/user/42?userId=42"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	jsonata "github.com/blues/jsonata-go"

	"github.com/boxer/codegen/ir"
	"github.com/boxer/codegen/runtime"
)

func main() {
	input := flag.String("input", "", "IR JSON file")
	port := flag.Int("port", 9090, "HTTP port")
	mockFile := flag.String("mock", "", "Mock upstreams JSON file (optional)")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./demo -input flow.json [-port 9090] [-mock mocks.json]")
		os.Exit(1)
	}

	// Load IR
	data, err := os.ReadFile(*input)
	if err != nil {
		log.Fatalf("read IR: %v", err)
	}
	var flow ir.GatewayIR
	if err := json.Unmarshal(data, &flow); err != nil {
		log.Fatalf("parse IR: %v", err)
	}

	// Load mock data
	up := buildUpstream(*mockFile, &flow)

	// Build handler directly from IR (interpreted mode, no codegen needed)
	handler := buildHandler(&flow, up)

	// Register route
	mux := http.NewServeMux()
	routePath := convertPath(flow.Trigger.Path)
	mux.HandleFunc(routePath, handler)

	log.Printf("Flow: %s (%s)", flow.Name, flow.ID)
	log.Printf("Route: %s %s", flow.Trigger.Method, routePath)
	log.Printf("Upstreams: %v", upstreamNames(&flow))
	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}

// buildUpstream creates a mock or HTTP upstream.
func buildUpstream(mockFile string, flow *ir.GatewayIR) runtime.Upstream {
	if mockFile != "" {
		data, err := os.ReadFile(mockFile)
		if err != nil {
			log.Fatalf("read mock: %v", err)
		}
		var mocks map[string]any
		if err := json.Unmarshal(data, &mocks); err != nil {
			log.Fatalf("parse mock: %v", err)
		}
		return &runtime.MockUpstream{Data: mocks}
	}

	// Default: mock with sample data
	mocks := map[string]any{}
	for _, n := range flow.Nodes {
		if n.Type == "http-call" {
			name := n.GetUpstream().Name
			if _, ok := mocks[name]; !ok {
				mocks[name] = map[string]any{
					"_mock":    true,
					"upstream": name,
					"message":  fmt.Sprintf("mock response from %s", name),
				}
			}
		}
	}
	log.Printf("Using auto-generated mock data (use -mock file.json for custom)")
	return &runtime.MockUpstream{Data: mocks}
}

// buildHandler interprets the IR at runtime (same logic as codegen output).
func buildHandler(flow *ir.GatewayIR, up runtime.Upstream) http.HandlerFunc {
	nodeMap := map[string]*ir.Node{}
	for i := range flow.Nodes {
		nodeMap[flow.Nodes[i].ID] = &flow.Nodes[i]
	}

	// Build edge lookup
	type edgeKey struct{ source, handle string }
	edgeTarget := map[edgeKey]string{}
	for _, e := range flow.Edges {
		edgeTarget[edgeKey{e.Source, e.SourceHandle}] = e.Target
	}
	nextNode := func(id, handle string) string {
		if t, ok := edgeTarget[edgeKey{id, handle}]; ok {
			return t
		}
		if handle != "" {
			if t, ok := edgeTarget[edgeKey{id, ""}]; ok {
				return t
			}
		}
		return ""
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := map[string]any{}
		mu := &sync.Mutex{}
		_ = mu

		params := map[string]any{}
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		vars["params"] = params

		// Find root node
		hasIncoming := map[string]bool{}
		for _, e := range flow.Edges {
			hasIncoming[e.Target] = true
		}
		currentID := ""
		for _, n := range flow.Nodes {
			if !hasIncoming[n.ID] {
				currentID = n.ID
				break
			}
		}

		visited := map[string]int{}
		for currentID != "" {
			visited[currentID]++
			if visited[currentID] > 100 {
				http.Error(w, "loop detected", 500)
				return
			}

			node := nodeMap[currentID]
			if node == nil {
				http.Error(w, "unknown node: "+currentID, 500)
				return
			}

			switch node.Type {
			case "http-call":
				path := interpolate(node.GetString("path"), params)
				method := node.GetString("method")
				if method == "" {
					method = "GET"
				}
				timeout := time.Duration(node.GetInt("timeout", 3000)) * time.Millisecond
				result, err := up.Call(node.GetUpstream().Name, path, method, timeout)
				if err != nil {
					http.Error(w, err.Error(), 502)
					return
				}
				vars[node.OutputVar] = result
				currentID = nextNode(currentID, "")

			case "condition":
				expr, err := jsonata.Compile(node.GetString("expression"))
				if err != nil {
					http.Error(w, "bad expression: "+err.Error(), 500)
					return
				}
				result, _ := expr.Eval(vars)
				if toBool(result) {
					currentID = nextNode(currentID, "true")
				} else {
					currentID = nextNode(currentID, "false")
				}

			case "switch":
				expr, err := jsonata.Compile(node.GetString("expression"))
				if err != nil {
					http.Error(w, "bad expression: "+err.Error(), 500)
					return
				}
				result, _ := expr.Eval(vars)
				val := toString(result)
				cases := node.GetStringSlice("cases")
				matched := false
				for i, c := range cases {
					if c == val {
						currentID = nextNode(currentID, fmt.Sprintf("case:%d", i))
						matched = true
						break
					}
				}
				if !matched {
					currentID = nextNode(currentID, "default")
				}

			case "transform":
				expr, err := jsonata.Compile(node.GetString("expression"))
				if err != nil {
					http.Error(w, "bad expression: "+err.Error(), 500)
					return
				}
				result, _ := expr.Eval(vars)
				vars[node.OutputVar] = result
				currentID = nextNode(currentID, "")

			case "fork":
				// Phase 2: sequential execution of branches
				branches := []string{}
				for _, e := range flow.Edges {
					if e.Source == currentID {
						branches = append(branches, e.Target)
					}
				}
				for _, bid := range branches {
					bn := nodeMap[bid]
					if bn != nil && bn.Type == "http-call" {
						path := interpolate(bn.GetString("path"), params)
						method := bn.GetString("method")
						if method == "" {
							method = "GET"
						}
						timeout := time.Duration(bn.GetInt("timeout", 3000)) * time.Millisecond
						result, err := up.Call(bn.GetUpstream().Name, path, method, timeout)
						if err != nil {
							http.Error(w, err.Error(), 502)
							return
						}
						vars[bn.OutputVar] = result
					}
				}
				// find join
				joinID := ""
				for _, bid := range branches {
					for _, e := range flow.Edges {
						if e.Source == bid {
							if t := nodeMap[e.Target]; t != nil && t.Type == "join" {
								joinID = t.ID
							}
						}
					}
				}
				currentID = joinID

			case "join":
				strategy := node.GetString("strategy")
				if strategy == "" {
					strategy = "merge"
				}
				inputVars := []string{}
				for _, e := range flow.Edges {
					if e.Target == currentID {
						if src := nodeMap[e.Source]; src != nil && src.OutputVar != "" {
							inputVars = append(inputVars, src.OutputVar)
						}
					}
				}
				switch strategy {
				case "merge":
					merged := map[string]any{}
					for _, v := range inputVars {
						if m, ok := vars[v].(map[string]any); ok {
							for k, val := range m {
								merged[k] = val
							}
						}
					}
					vars[node.OutputVar] = merged
				case "array":
					arr := []any{}
					for _, v := range inputVars {
						arr = append(arr, vars[v])
					}
					vars[node.OutputVar] = arr
				}
				currentID = nextNode(currentID, "")

			case "response":
				expr, err := jsonata.Compile(node.GetString("body"))
				if err != nil {
					http.Error(w, "bad body expression: "+err.Error(), 500)
					return
				}
				body, _ := expr.Eval(vars)
				statusCode := node.GetInt("statusCode", 200)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)
				json.NewEncoder(w).Encode(body)
				return

			default:
				http.Error(w, "unsupported node type: "+node.Type, 500)
				return
			}
		}

		http.Error(w, "flow did not reach response", 500)
	}
}

// ── Helpers ──────────────────────────────────────────

func interpolate(tmpl string, params map[string]any) string {
	result := tmpl
	for k, v := range params {
		result = strings.ReplaceAll(result, "${ctx.params."+k+"}", toString(v))
	}
	return result
}

func toBool(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return v != "" && v != 0
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// convertPath converts Express-style ":param" to Go's "{param}" or wildcard.
func convertPath(path string) string {
	// For ServeMux, just use the path prefix
	parts := strings.Split(path, "/")
	var clean []string
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			break // stop at first param, use as prefix match
		}
		clean = append(clean, p)
	}
	result := strings.Join(clean, "/")
	if !strings.HasSuffix(result, "/") {
		result += "/"
	}
	return result
}

func upstreamNames(flow *ir.GatewayIR) []string {
	seen := map[string]bool{}
	var names []string
	for _, n := range flow.Nodes {
		if n.Type == "http-call" {
			name := n.GetUpstream().Name
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}
