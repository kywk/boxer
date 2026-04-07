# Boxer — API Gateway Low-Code Editor

視覺化 API Gateway 流程編輯器，透過拖拉節點設計 API 組合邏輯，自動生成可部署的 Go HTTP handler 或 Kong Plugin (Lua)。

## 架構

```
┌──────────────────────────────────────┐
│   Visual Flow Editor (Vue Flow)      │  ← 拖拉式流程設計
│   + Node Palette + Config Panel      │
│   + Test Executor + Codegen 整合     │
└───────────────┬──────────────────────┘
                │ vueFlowToIR()
                ▼
┌──────────────────────────────────────┐
│       Gateway IR (JSON + Zod)        │  ← 核心合約
└──────────┬───────────────────────────┘
           │ codegen service / CLI
    ┌──────┴──────┐
    ▼             ▼
Go Handler    Kong Plugin (Lua)
                  + kong.yaml (decK)
```

## 快速開始

### 前端編輯器

```bash
cd frontend
npm install
npm run dev
# → http://localhost:5173
```

### Codegen Service（搭配前端使用）

```bash
cd codegen
go run ./cmd/main.go serve
# → http://localhost:8080
# 前端 vite dev server 自動 proxy /api/codegen 到此服務
```

啟動後在前端編輯器中點擊 **⚙ Go** 或 **⚙ Lua** 即可生成程式碼並下載。

### Codegen CLI（獨立使用）

```bash
cd codegen

# IR → Go handler
go run ./cmd/main.go -input testdata/flow-user-profile.json -target golang -output handler.go

# IR → Kong Plugin Lua + kong.yaml
go run ./cmd/main.go -input testdata/flow-user-profile.json -target kong -output handler.lua -deck
# → handler.lua + kong.yaml（可直接 deck sync -s kong.yaml）
```

### Demo Server（E2E 驗證）

```bash
cd codegen
go run ./demo -input testdata/flow-user-profile.json -mock testdata/mock-user-vip.json
curl "http://localhost:9090/api/user/?userId=42"
```

## 節點類型

| 節點 | 用途 | Handle |
|------|------|--------|
| **HTTP Call** | 呼叫上游服務（支援 retry / fallback） | 1 in → 1 out |
| **Condition** | 布林分支（if/else） | 1 in → true/false out |
| **Switch** | 多路分支（switch/case） | 1 in → case:0, case:1, ..., default out |
| **Transform** | JSONata 資料轉換 | 1 in → 1 out |
| **Fork** | 並行分支起點（all/race/allSettled） | 1 in → N out |
| **Join** | 並行分支合併（merge/array/custom） | N in → 1 out |
| **Sub-Flow** | 引用其他流程（codegen 時 inline 展開） | 1 in → 1 out |
| **Response** | 回傳 HTTP response（終端節點） | 1 in → 無 |

## IR Schema

核心合約定義在 `frontend/src/ir/schema.ts`（Zod），Go 對應型別在 `codegen/ir/types.go`。

關鍵設計：
- **upstream** 為 object `{ name, provider, url? }`，支援 kong / k8s-service / url
- **http-call** 支援 retry（maxAttempts / backoff / delay）和 fallback（default-value / skip / error）
- **fork/join** 顯式表達並行語義，Go 用 `errgroup`，Lua 用 `ngx.thread`
- **sub-flow** codegen 時 inline 展開，不做 runtime resolve
- **switch** 支援多路分支，`sourceHandle` 為 `case:N` 或 `default`

## 專案結構

```
boxer/
├── docs/history/                        # 設計文件與決策記錄
├── frontend/                            # Vue 3 + Vue Flow + TypeScript
│   └── src/
│       ├── ir/schema.ts                 # IR Zod Schema（核心合約）
│       ├── components/
│       │   ├── FlowEditor.vue           # 主編輯器 + 測試面板 + codegen 整合
│       │   ├── NodePalette.vue          # 左側拖拉庫
│       │   ├── ConfigPanel.vue          # 右側屬性面板 + 執行結果顯示
│       │   └── nodes/                   # 8 種節點 Vue 組件
│       └── composables/
│           ├── useFlowValidator.ts      # 連線語義驗證
│           ├── useIRExport.ts           # vueFlowToIR()
│           ├── useIRImport.ts           # irToVueFlow() + 自動佈局
│           ├── useIRExecutor.ts         # 瀏覽器端測試執行器
│           └── useSubFlowExpander.ts    # sub-flow inline 展開
└── codegen/                             # Go codegen
    ├── cmd/main.go                      # CLI 入口 + serve 子命令
    ├── ir/types.go                      # Go IR 型別
    ├── core/graph.go                    # 拓樸排序 + 圖分析
    ├── targets/
    │   ├── golang/generator.go          # Go handler 生成器
    │   └── kong/
    │       ├── generator.go             # Kong Plugin Lua 生成器
    │       └── deck.go                  # kong.yaml (decK) 生成器
    ├── server/server.go                 # Codegen HTTP service
    ├── runtime/upstream.go              # Upstream 介面 + Mock/HTTP 實作
    ├── demo/main.go                     # E2E demo server
    └── testdata/                        # 測試 IR + mock 資料
```

## 設計決策

詳見 `docs/history/2016-04-05-init/spec-design.md` 第十節「設計決策」。

## License

Private
