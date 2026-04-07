package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Upstream is the interface that generated handlers depend on.
type Upstream interface {
	Call(name, path, method string, timeout time.Duration) (any, error)
}

// MockUpstream returns predefined responses for each upstream name.
type MockUpstream struct {
	Data map[string]any // upstream name → mock response
}

func (m *MockUpstream) Call(name, path, method string, timeout time.Duration) (any, error) {
	if data, ok := m.Data[name]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("mock: no data for upstream %q (path=%s)", name, path)
}

// HTTPUpstream calls real HTTP endpoints.
// Registry maps upstream name → base URL (e.g. "user-service" → "http://localhost:8081")
type HTTPUpstream struct {
	Registry map[string]string
	Client   *http.Client
}

func NewHTTPUpstream(registry map[string]string) *HTTPUpstream {
	return &HTTPUpstream{
		Registry: registry,
		Client:   &http.Client{},
	}
}

func (h *HTTPUpstream) Call(name, path, method string, timeout time.Duration) (any, error) {
	base, ok := h.Registry[name]
	if !ok {
		return nil, fmt.Errorf("upstream %q not registered", name)
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(method, base+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	return result, nil
}
