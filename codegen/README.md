# Boxer Codegen

Go CLI 工具，將 Gateway IR JSON 生成可部署的 Go HTTP handler。

## 使用方式

### Codegen CLI

```bash
# IR → Go handler source code
go run ./cmd/main.go -input testdata/flow-user-profile.json -output handler.go

# 指定 target（目前只支援 golang）
go run ./cmd/main.go -input flow.json -target golang -output handler.go
```

生成的 handler 依賴：
- `github.com/blues/jsonata-go` — JSONata 表達式 runtime
- `golang.org/x/sync/errgroup` — fork/join 並行執行

### Demo Server

IR interpreter 模式，讀取 IR JSON 直接啟動 HTTP server：

```bash
# 使用 mock data
go run ./demo -input testdata/flow-user-profile.json -mock testdata/mock-user-vip.json
curl "http://localhost:9090/api/user/?userId=42"

# Switch 流程
go run ./demo -input testdata/flow-order-switch.json -mock testdata/mock-order-digital.json
curl "http://localhost:9090/api/order/?orderId=ORD-001"

# Fork/Join 並行
go run ./demo -input testdata/flow-dashboard.json -mock testdata/mock-dashboard.json
curl "http://localhost:9090/api/dashboard/?userId=42"
```

## 套件結構

```
codegen/
├── cmd/main.go                    # CLI 入口
├── ir/types.go                    # Go IR 型別（對應前端 Zod schema）
├── core/graph.go                  # 拓樸排序 + 圖分析 + prerequisites
├── targets/golang/generator.go    # Go handler 模板生成器
├── runtime/upstream.go            # Upstream 介面（Mock + HTTP 實作）
├── demo/main.go                   # E2E demo server（IR interpreter）
└── testdata/                      # 測試資料
    ├── flow-user-profile.json     # condition 分支
    ├── flow-order-switch.json     # switch 多路分支
    ├── flow-dashboard.json        # fork/join 並行
    ├── mock-user-vip.json
    ├── mock-order-digital.json
    └── mock-dashboard.json
```

## 生成的 Go Handler 特性

- `http.HandlerFunc` 閉包，注入 `upstream` interface
- condition/switch 用 `goto` + label 實現分支（只有被跳轉的節點才生成 label）
- fork 用 `errgroup` 真正並行，`sync.Mutex` 保護 vars
- join 支援 merge / array / custom 三種策略
- JSONata 表達式全部 runtime evaluate
- `interpolate()` 處理 `${ctx.params.xxx}` 路徑插值
