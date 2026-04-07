# Boxer Codegen

Go 工具，將 Gateway IR JSON 生成可部署的 Go HTTP handler 或 Kong Plugin (Lua)。支援 CLI 和 HTTP service 兩種模式。

## 使用方式

### Codegen CLI

```bash
# IR → Go handler
go run ./cmd/main.go -input testdata/flow-user-profile.json -target golang -output handler.go

# IR → Kong Plugin Lua
go run ./cmd/main.go -input testdata/flow-user-profile.json -target kong -output handler.lua

# IR → Kong Plugin Lua + kong.yaml (decK declarative config)
go run ./cmd/main.go -input testdata/flow-user-profile.json -target kong -output handler.lua -deck
# → handler.lua + kong.yaml（可直接 deck sync -s kong.yaml）
```

### Codegen HTTP Service

```bash
go run ./cmd/main.go serve [-addr :8080]
```

API：

```
POST /api/codegen
Content-Type: application/json

{
  "ir":     { ...GatewayIR },
  "target": "golang" | "kong"
}

Response:
{
  "code":          "...生成的程式碼...",
  "filename":      "handler_xxx.go" | "handler_xxx.lua",
  "prerequisites": { "upstreams": ["user-service", ...] },
  "warnings":      [],
  "cached":        false
}
```

- 相同 IR + target 的請求會自動 cache（SHA256 hash）
- 內建 CORS middleware，可直接被前端呼叫
- 前端 vite dev server 預設 proxy `/api/codegen` 到 `localhost:8080`

### Demo Server

IR interpreter 模式，讀取 IR JSON 直接啟動 HTTP server（用於 E2E 驗證）：

```bash
# Condition 分支
go run ./demo -input testdata/flow-user-profile.json -mock testdata/mock-user-vip.json
curl "http://localhost:9090/api/user/?userId=42"

# Switch 多路分支
go run ./demo -input testdata/flow-order-switch.json -mock testdata/mock-order-digital.json
curl "http://localhost:9090/api/order/?orderId=ORD-001"

# Fork/Join 並行
go run ./demo -input testdata/flow-dashboard.json -mock testdata/mock-dashboard.json
curl "http://localhost:9090/api/dashboard/?userId=42"
```

## 套件結構

```
codegen/
├── cmd/main.go                        # CLI 入口 + serve 子命令
├── ir/types.go                        # Go IR 型別（對應前端 Zod schema）
├── core/graph.go                      # 拓樸排序 + 圖分析 + prerequisites
├── targets/
│   ├── golang/generator.go            # Go handler 生成器
│   └── kong/
│       ├── generator.go               # Kong Plugin Lua 生成器（function-per-node）
│       └── deck.go                    # kong.yaml (decK) 生成器
├── server/server.go                   # Codegen HTTP service（cache + CORS）
├── runtime/upstream.go                # Upstream 介面（Mock + HTTP 實作）
├── demo/main.go                       # E2E demo server（IR interpreter）
└── testdata/
    ├── flow-user-profile.json         # condition 分支
    ├── flow-order-switch.json         # switch 多路分支
    ├── flow-dashboard.json            # fork/join 並行
    ├── mock-user-vip.json
    ├── mock-order-digital.json
    └── mock-dashboard.json
```

## 生成的程式碼特性

### Go Handler

- `http.HandlerFunc` 閉包，注入 `upstream` interface
- condition/switch 用 `goto` + label 實現分支（只有被跳轉的節點才生成 label）
- fork 用 `errgroup` 真正並行，分支邏輯 inline 進 goroutine，`sync.Mutex` 保護 vars
- join 支援 merge / array / custom 三種策略
- JSONata 表達式全部 runtime evaluate（`jsonata-go`）
- `interpolate()` 處理 `${ctx.params.xxx}` 路徑插值

### Kong Plugin (Lua)

- function-per-node 模式：每個節點一個 `local function`，回傳下一個 node ID
- dispatcher 迴圈：`while next_node do fn = nodes[next_node]; next_node = fn(ctx) end`
- fork 用 `ngx.thread.spawn` + `ngx.thread.wait` 實現並行
- JSONata 表達式全部 runtime evaluate（`resty.jsonata`）
- `ctx:interpolate()` 處理路徑參數插值

### kong.yaml (decK)

- 自動生成 service + route + plugin 配置
- 自動列出所有 upstream 及預設 K8s service target
- 可直接 `deck sync -s kong.yaml` 部署到 Kong
