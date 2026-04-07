package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/boxer/codegen/ir"
	"github.com/boxer/codegen/targets/golang"
	"github.com/boxer/codegen/targets/kong"
)

// Server is the codegen HTTP service.
type Server struct {
	cache sync.Map // hash → *CachedResult
}

type CodegenRequest struct {
	IR     ir.GatewayIR `json:"ir"`
	Target string       `json:"target"` // "golang" | "kong"
}

type CodegenResponse struct {
	Code          string   `json:"code"`
	Filename      string   `json:"filename"`
	Prerequisites struct {
		Upstreams []string `json:"upstreams"`
	} `json:"prerequisites"`
	Warnings []string `json:"warnings"`
	Cached   bool     `json:"cached"`
}

type CachedResult struct {
	Response CodegenResponse
}

func New() *Server {
	return &Server{}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/codegen", s.handleCodegen)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return corsMiddleware(mux)
}

func (s *Server) handleCodegen(w http.ResponseWriter, r *http.Request) {
	var req CodegenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, "invalid request: %v", err)
		return
	}

	if req.Target == "" {
		req.Target = "golang"
	}

	// Cache lookup
	cacheKey := computeHash(req)
	if cached, ok := s.cache.Load(cacheKey); ok {
		resp := cached.(*CachedResult).Response
		resp.Cached = true
		writeJSON(w, 200, resp)
		return
	}

	// Generate
	var resp CodegenResponse
	switch req.Target {
	case "golang":
		result, err := golang.Generate(&req.IR)
		if err != nil {
			httpError(w, 422, "codegen failed: %v", err)
			return
		}
		resp.Code = result.Code
		resp.Filename = result.Filename
		resp.Prerequisites.Upstreams = result.Prerequisites
		resp.Warnings = result.Warnings

	case "kong":
		result, err := kong.Generate(&req.IR)
		if err != nil {
			httpError(w, 422, "codegen failed: %v", err)
			return
		}
		resp.Code = result.Code
		resp.Filename = result.Filename
		resp.Prerequisites.Upstreams = result.Prerequisites
		resp.Warnings = result.Warnings

	default:
		httpError(w, 400, "unknown target: %s", req.Target)
		return
	}

	// Cache store
	s.cache.Store(cacheKey, &CachedResult{Response: resp})

	writeJSON(w, 200, resp)
}

// ── Helpers ──────────────────────────────────────────

func computeHash(req CodegenRequest) string {
	data, _ := json.Marshal(req)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListenAndServe starts the codegen HTTP service.
func ListenAndServe(addr string) error {
	s := New()
	log.Printf("codegen service listening on %s", addr)
	return http.ListenAndServe(addr, s.Handler())
}
