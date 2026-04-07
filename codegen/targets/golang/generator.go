package golang

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/boxer/codegen/core"
	"github.com/boxer/codegen/ir"
)

// GenerateResult holds the codegen output.
type GenerateResult struct {
	Code          string   `json:"code"`
	Filename      string   `json:"filename"`
	Prerequisites []string `json:"prerequisites"`
	Warnings      []string `json:"warnings"`
}

// Generate produces a Go HTTP handler from the IR.
func Generate(flow *ir.GatewayIR) (*GenerateResult, error) {
	infos, order, err := core.AnalyzeGraph(flow)
	if err != nil {
		return nil, fmt.Errorf("graph analysis failed: %w", err)
	}

	data := buildTemplateData(flow, infos, order)

	tmpl, err := template.New("handler").Funcs(funcMap()).Parse(handlerTemplate)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execute error: %w", err)
	}

	return &GenerateResult{
		Code:          buf.String(),
		Filename:      "handler_" + sanitize(flow.ID) + ".go",
		Prerequisites: core.Prerequisites(flow),
		Warnings:      data.Warnings,
	}, nil
}

// ── Template data ────────────────────────────────────

type templateData struct {
	PackageName string
	FuncName    string
	FlowID      string
	FlowName    string
	Nodes       []templateNode
	Warnings    []string
}

type templateNode struct {
	ID          string
	Type        string
	NeedsLabel  bool
	Upstream    ir.Upstream
	Path        string
	Method      string
	Timeout     int
	Expression  string
	OutputVar   string
	NextID      string
	TrueBranch  string
	FalseBranch string
	Cases       []templateCase
	DefaultID   string
	Branches    []string
	BranchNodes []templateNode // fork: resolved branch node data for inline codegen
	JoinID      string
	InputVars   []string
	StatusCode  int
	Body        string
	Strategy    string
	HasFallback bool
}

type templateCase struct {
	Value    string
	TargetID string
}

func buildTemplateData(flow *ir.GatewayIR, infos map[string]*core.NodeInfo, order []string) templateData {
	data := templateData{
		PackageName: "handler",
		FuncName:    toPascal(flow.ID),
		FlowID:      flow.ID,
		FlowName:    flow.Name,
	}

	// collect all goto targets so we know which nodes need labels
	gotoTargets := map[string]bool{}
	for _, info := range infos {
		n := info.Node
		switch n.Type {
		case "condition":
			gotoTargets[info.TrueBranch] = true
			gotoTargets[info.FalseBranch] = true
		case "switch":
			for _, t := range info.CaseBranch {
				if t != "" {
					gotoTargets[t] = true
				}
			}
			if info.DefaultID != "" {
				gotoTargets[info.DefaultID] = true
			}
		}
	}
	// nodes that are goto targets and have a NextID also need their NextID labeled
	// (because they emit "goto NextID" to avoid fall-through)
	for id := range gotoTargets {
		if info, ok := infos[id]; ok && info.NextID != "" {
			gotoTargets[info.NextID] = true
		}
	}

	// Track nodes that are inlined into fork goroutines (skip in main flow)
	inlinedNodes := map[string]bool{}
	for _, info := range infos {
		if info.Node.Type == "fork" {
			for _, bid := range info.Branches {
				if bInfo, ok := infos[bid]; ok && bInfo.Node.Type == "http-call" {
					inlinedNodes[bid] = true
				}
			}
		}
	}

	for _, id := range order {
		if inlinedNodes[id] {
			continue
		}
		info := infos[id]
		n := info.Node

		tn := templateNode{
			ID:          n.ID,
			Type:        n.Type,
			NeedsLabel:  gotoTargets[n.ID],
			OutputVar:   n.OutputVar,
			NextID:      info.NextID,
			TrueBranch:  info.TrueBranch,
			FalseBranch: info.FalseBranch,
			DefaultID:   info.DefaultID,
			Branches:    info.Branches,
			JoinID:      info.JoinID,
			InputVars:   info.InputVars,
		}

		switch n.Type {
		case "http-call":
			tn.Upstream = n.GetUpstream()
			tn.Path = n.GetString("path")
			tn.Method = n.GetString("method")
			if tn.Method == "" {
				tn.Method = "GET"
			}
			tn.Timeout = n.GetInt("timeout", 3000)
			if _, ok := n.Config["fallback"]; ok {
				tn.HasFallback = true
			}

		case "condition":
			tn.Expression = n.GetString("expression")

		case "switch":
			tn.Expression = n.GetString("expression")
			cases := n.GetStringSlice("cases")
			for i, c := range cases {
				target := ""
				if i < len(info.CaseBranch) {
					target = info.CaseBranch[i]
				}
				tn.Cases = append(tn.Cases, templateCase{Value: c, TargetID: target})
			}

		case "transform":
			tn.Expression = n.GetString("expression")

		case "fork":
			tn.Strategy = n.GetString("strategy")
			if tn.Strategy == "" {
				tn.Strategy = "all"
			}
			// Resolve branch nodes for inline codegen
			for _, bid := range info.Branches {
				if bInfo, ok := infos[bid]; ok {
					bn := bInfo.Node
					if bn.Type == "http-call" {
						tn.BranchNodes = append(tn.BranchNodes, templateNode{
							ID:        bn.ID,
							Type:      bn.Type,
							Upstream:  bn.GetUpstream(),
							Path:      bn.GetString("path"),
							Method:    orDefault(bn.GetString("method"), "GET"),
							Timeout:   bn.GetInt("timeout", 3000),
							OutputVar: bn.OutputVar,
						})
					}
				}
			}

		case "join":
			tn.Strategy = n.GetString("strategy")
			if tn.Strategy == "" {
				tn.Strategy = "merge"
			}
			tn.Expression = n.GetString("expression")

		case "response":
			tn.StatusCode = n.GetInt("statusCode", 200)
			tn.Body = n.GetString("body")
		}

		data.Nodes = append(data.Nodes, tn)
	}

	return data
}

// ── Template ─────────────────────────────────────────

const handlerTemplate = `// Auto-generated by gateway-codegen — DO NOT EDIT
// Flow: {{ .FlowName }} ({{ .FlowID }})
package {{ .PackageName }}

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/blues/jsonata-go"
	"golang.org/x/sync/errgroup"
)

// upstream is a minimal interface for calling upstream services.
type upstream interface {
	Call(name, path, method string, timeout time.Duration) (any, error)
}

// Handle{{ .FuncName }} is the generated handler for flow "{{ .FlowID }}".
func Handle{{ .FuncName }}(up upstream) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := make(map[string]any)
		mu := &sync.Mutex{}
		_ = mu // used when fork/join is present
		params := extractParams(r)
		vars["params"] = params
{{ range .Nodes }}
{{ if eq .Type "http-call" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: http-call → {{ .Upstream.Name }}
		{
			path := interpolate(` + "`{{ .Path }}`" + `, params)
			result, err := up.Call("{{ .Upstream.Name }}", path, "{{ .Method }}", {{ .Timeout }}*time.Millisecond)
			if err != nil {
{{- if .HasFallback }}
				vars["{{ .OutputVar }}"] = nil
{{- else }}
				http.Error(w, err.Error(), 502)
				return
{{- end }}
			} else {
				vars["{{ .OutputVar }}"] = result
			}
		}
{{- if and .NeedsLabel .NextID }}
		goto {{ .NextID }}
{{ end }}
{{ else if eq .Type "condition" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: condition
		{
			expr, _ := jsonata.Compile(` + "`{{ .Expression }}`" + `)
			result, _ := expr.Eval(vars)
			if toBool(result) {
				goto {{ .TrueBranch }}
			}
			goto {{ .FalseBranch }}
		}
{{ else if eq .Type "switch" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: switch
		{
			expr, _ := jsonata.Compile(` + "`{{ .Expression }}`" + `)
			result, _ := expr.Eval(vars)
			val := toString(result)
			switch val {
{{- range .Cases }}
			case "{{ .Value }}":
				goto {{ .TargetID }}
{{- end }}
			default:
{{- if .DefaultID }}
				goto {{ .DefaultID }}
{{- else }}
				http.Error(w, "no matching case: "+val, 400)
				return
{{- end }}
			}
		}
{{ else if eq .Type "transform" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: transform
		{
			expr, _ := jsonata.Compile(` + "`{{ .Expression }}`" + `)
			result, _ := expr.Eval(vars)
			vars["{{ .OutputVar }}"] = result
		}
{{- if and .NeedsLabel .NextID }}
		goto {{ .NextID }}
{{ end }}
{{ else if eq .Type "fork" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: fork ({{ .Strategy }})
		{
			g, _ := errgroup.WithContext(r.Context())
{{- range .BranchNodes }}
			g.Go(func() error {
				// branch {{ .ID }}: http-call → {{ .Upstream.Name }}
				path := interpolate(` + "`{{ .Path }}`" + `, params)
				result, err := up.Call("{{ .Upstream.Name }}", path, "{{ .Method }}", {{ .Timeout }}*time.Millisecond)
				if err != nil { return err }
				mu.Lock()
				vars["{{ .OutputVar }}"] = result
				mu.Unlock()
				return nil
			})
{{- end }}
			if err := g.Wait(); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
		}
{{ else if eq .Type "join" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: join ({{ .Strategy }})
		{
{{- if eq .Strategy "merge" }}
			merged := make(map[string]any)
{{- range .InputVars }}
			if m, ok := vars["{{ . }}"].(map[string]any); ok {
				for k, v := range m { merged[k] = v }
			}
{{- end }}
			vars["{{ .OutputVar }}"] = merged
{{- else if eq .Strategy "array" }}
			vars["{{ .OutputVar }}"] = []any{ {{- range .InputVars }}vars["{{ . }}"], {{ end -}} }
{{- else }}
			expr, _ := jsonata.Compile(` + "`{{ .Expression }}`" + `)
			result, _ := expr.Eval(vars)
			vars["{{ .OutputVar }}"] = result
{{- end }}
		}
{{ else if eq .Type "response" }}{{ if .NeedsLabel }}	{{ .ID }}:
{{ end }}		// {{ .ID }}: response
		{
			expr, _ := jsonata.Compile(` + "`{{ .Body }}`" + `)
			body, _ := expr.Eval(vars)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader({{ .StatusCode }})
			json.NewEncoder(w).Encode(body)
			return
		}
{{ end }}{{ end }}	}
}

func extractParams(r *http.Request) map[string]any {
	params := make(map[string]any)
	for k, v := range r.URL.Query() {
		if len(v) > 0 { params[k] = v[0] }
	}
	return params
}

func interpolate(tmpl string, params map[string]any) string {
	result := tmpl
	for k, v := range params {
		result = strings.ReplaceAll(result, "${ctx.params."+k+"}", toString(v))
	}
	return result
}

func toBool(v any) bool {
	if v == nil { return false }
	if b, ok := v.(bool); ok { return b }
	return v != "" && v != 0 && v != false
}

func toString(v any) string {
	if v == nil { return "" }
	if s, ok := v.(string); ok { return s }
	return fmt.Sprintf("%v", v)
}
`

// ── Helpers ──────────────────────────────────────────

func funcMap() template.FuncMap {
	return template.FuncMap{}
}

var nonAlpha = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitize(s string) string {
	return strings.ToLower(nonAlpha.ReplaceAllString(s, "_"))
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func toPascal(s string) string {
	parts := nonAlpha.Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}
