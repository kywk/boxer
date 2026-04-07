# Boxer — API Gateway Low-Code Editor

視覺化 API Gateway 流程編輯器，透過拖拉節點設計 API 組合邏輯，自動生成可部署的 Go HTTP handler 或 Kong Plugin (Lua)。

## 架構

```
┌──────────────────────────────────────┐
│   Visual Flow Editor (Vue Flow)      │  ← 拖拉式流程設計
│   + Node Palette + Config Panel      │
└───────────────┬──────────────────────┘
                │ vueFlowToIR()
                ▼
┌──────────────────────────────────────┐
│       Gateway IR (JSON + Zod)        │  ← 核心合約
└──────────┬───────────────────────────┘
           │ codegen
    ┌──────┴──────┐
    ▼             ▼
Go Handler    Kong Plugin (Phase 4)
```

## 快速開始

### 前端編輯器

```bash
cd frontend
npm install
npm run dev
# → http://localhost:5173
```

### Go Codegen CLI

```bash
cd codegen

# IR → Go handler source code
go run ./cmd/main.go -input testdata/flow-user-profile.json -output handler.go

# Demo server（IR 直接執行為 HTTP endpoint）
go run ./demo -input testdata/flow-user-profile.json -mock testdata/mock-user-vip.json
curl "http://localhost:9090/api/user/?userId=42"
```

## 節點類型

| 節點 | 用途 | Handle |
|------|------|--------|
| **HTTP Call** | 呼叫上游服務 | 1 in → 1 out |
| **Condition** | 布林分支（if/else） | 1 in → true/false out |
| **Switch** | 多路分支（switch/case） | 1 in → case:0, case:1, ..., default out |
| **Transform** | JSONata 資料轉換 | 1 in → 1 out |
| **Fork** | 並行分支起點 | 1 in → N out |
| **Join** | 並行分支合併 | N in → 1 out（merge/array/custom） |
| **Sub-Flow** | 引用其他流程（inline 展開） | 1 in → 1 out |
| **Response** | 回傳 HTTP response（終端節點） | 1 in → 無 |

## IR Schema

核心合約定義在 `frontend/src/ir/schema.ts`（Zod），Go 對應型別在 `codegen/ir/types.go`。

關鍵設計：
- **upstream** 為 object `{ name, provider, url? }`，支援 kong / k8s-service / url
- **http-call** 支援 retry（maxAttempts / backoff / delay）和 fallback（default-value / skip / error）
- **fork/join** 顯式表達並行語義，Go codegen 生成 `errgroup` 並行呼叫
- **sub-flow** codegen 時 inline 展開，不做 runtime resolve
- **switch** 支援多路分支，`sourceHandle` 為 `case:N` 或 `default`

## 專案結構

```
boxer/
├── docs/history/                    # 設計文件與決策記錄
├── frontend/                        # Vue 3 + Vue Flow + TypeScript
│   └── src/
│       ├── ir/schema.ts             # IR Zod Schema（核心合約）
│       ├── components/
│       │   ├── FlowEditor.vue       # 主編輯器 + 測試面板
│       │   ├── NodePalette.vue      # 左側拖拉庫
│       │   ├── ConfigPanel.vue      # 右側屬性面板
│       │   └── nodes/              # 8 種節點 Vue 組件
│       └── composables/
│           ├── useFlowValidator.ts  # 連線語義驗證
│           ├── useIRExport.ts       # vueFlowToIR()
│           ├── useIRImport.ts       # irToVueFlow() + 自動佈局
│           ├── useIRExecutor.ts     # 瀏覽器端測試執行器
│           └── useSubFlowExpander.ts# sub-flow inline 展開
└── codegen/                         # Go codegen
    ├── cmd/main.go                  # CLI 入口
    ├── ir/types.go                  # Go IR 型別
    ├── core/graph.go                # 拓樸排序 + 圖分析
    ├── targets/golang/generator.go  # Go handler 生成器
    ├── runtime/upstream.go          # Upstream 介面 + Mock/HTTP 實作
    ├── demo/main.go                 # E2E demo server
    └── testdata/                    # 測試 IR + mock 資料
```

## 開發進度

- [x] Phase 1 — IR Schema + Vue Flow 編輯器 + 瀏覽器端執行器
- [x] Phase 2 — Go Codegen CLI + E2E demo server
- [x] Phase 3 — Sub-flow 展開 + 並行 fork + retry/fallback
- [ ] Phase 4 — ConfigPanel 完整表單 / 執行結果視覺化 / Kong Plugin Lua codegen / 部署整合

## 設計決策

詳見 `docs/history/2016-04-05-init/spec-design.md` 第十節「設計決策」。

## License

Private
