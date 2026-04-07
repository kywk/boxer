package ir

// GatewayIR is the top-level IR structure, matching the frontend Zod schema.
type GatewayIR struct {
	Version        string          `json:"version"`
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Trigger        Trigger         `json:"trigger"`
	Nodes          []Node          `json:"nodes"`
	Edges          []Edge          `json:"edges"`
	ExecutionHints *ExecutionHints `json:"executionHints,omitempty"`
	Metadata       *Metadata       `json:"metadata,omitempty"`
}

type Trigger struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Upstream struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"` // "kong" | "k8s-service" | "url"
	URL      string `json:"url,omitempty"`
}

type RetryConfig struct {
	MaxAttempts int    `json:"maxAttempts,omitempty"`
	Backoff     string `json:"backoff,omitempty"` // "fixed" | "exponential"
	Delay       int    `json:"delay,omitempty"`
}

type FallbackConfig struct {
	Strategy string `json:"strategy,omitempty"` // "default-value" | "skip" | "error"
	Value    any    `json:"value,omitempty"`
}

type Node struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	OutputVar string         `json:"outputVar,omitempty"`
}

type Edge struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"`
}

type ExecutionHints struct {
	ParallelGroups [][]string `json:"parallelGroups,omitempty"`
}

type Metadata struct {
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Author    string `json:"author,omitempty"`
}

// ── Typed config helpers ─────────────────────────────

// GetString safely extracts a string from node config.
func (n *Node) GetString(key string) string {
	if v, ok := n.Config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt safely extracts an int from node config.
func (n *Node) GetInt(key string, defaultVal int) int {
	if v, ok := n.Config[key]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		}
	}
	return defaultVal
}

// GetUpstream extracts the upstream config object.
func (n *Node) GetUpstream() Upstream {
	raw, ok := n.Config["upstream"]
	if !ok {
		return Upstream{}
	}
	m, ok := raw.(map[string]any)
	if !ok {
		// legacy: upstream as plain string
		if s, ok := raw.(string); ok {
			return Upstream{Name: s, Provider: "kong"}
		}
		return Upstream{}
	}
	u := Upstream{}
	if v, ok := m["name"].(string); ok {
		u.Name = v
	}
	if v, ok := m["provider"].(string); ok {
		u.Provider = v
	}
	if v, ok := m["url"].(string); ok {
		u.URL = v
	}
	return u
}

// GetStringSlice extracts a string slice from node config.
func (n *Node) GetStringSlice(key string) []string {
	raw, ok := n.Config[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// GetBool safely extracts a bool from node config.
func (n *Node) GetBool(key string, defaultVal bool) bool {
	if v, ok := n.Config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}
