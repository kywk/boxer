# API Gateway Low-Code Editor — 技術設計文件

> 本文件彙整設計討論，用於在 Claude Code 繼續開發。  
> 技術棧：Vue 3 + Vue Flow + Go codegen + Kong Plugin (Lua) / Go program

---

## 一、背景與目標

### 問題陳述
需要一個 **API Gateway 視覺化低代碼編輯器**，支援：
- 基礎 data composition（多上游結果合併）
- conditional call flow（條件分支調用）
- upstream aggregation（並行/串行調用多個上游）
- request/response 轉換

### 方案決策
排除 n8n 的原因：
- n8n 的抽象層是「自動化任務流」，而 API Gateway 的抽象層是「請求/響應 Pipeline」，語義錯配
- n8n workflow JSON 是其 runtime 專屬格式，從它 codegen 到 Kong Plugin (Lua) / Go 的 transformer 會非常脆弱

**選定方案：自定義 IR 為核心**

```
Vue Flow Editor  →  toObject()  →  vueFlowToIR()  →  Gateway IR (JSON)  →  Codegen
                                                            ↑
                                                       Zod 驗證
                                                            ↓
                                                    fromObject() 還原
```

---

## 二、整體架構

```
┌──────────────────────────────────────┐
│   Visual Flow Editor (Vue Flow)      │  ← Vue 3 組件，控制節點語義
│   + Node Palette (左側拖拉庫)        │
│   + Config Panel (右側屬性面板)      │
└───────────────┬──────────────────────┘
                │ toObject() + vueFlowToIR()
                ▼
┌──────────────────────────────────────┐
│       Gateway IR (JSON Schema)       │  ← 核心合約，Zod 雙向驗證
│  { version, id, trigger,             │
│    nodes[], edges[], metadata }      │
└──────────┬───────────────────────────┘
           │
    ┌──────┴──────┐
    ▼             ▼
Kong Plugin    Go Program
  (Lua)         (HTTP handler)
```

IR 是前後端的穩定合約：
- 前端 UI 變更不影響 codegen
- 新增 codegen target（如 Envoy WASM）只需新增一個 target，不動 IR

---

## 三、專案目錄結構

```
gateway-editor/
├── frontend/                        # Vue 3 + Vite
│   ├── src/
│   │   ├── components/
│   │   │   ├── FlowEditor.vue       # VueFlow 主容器
│   │   │   ├── NodePalette.vue      # 左側節點拖拉庫
│   │   │   ├── ConfigPanel.vue      # 右側節點配置面板（點擊節點開啟）
│   │   │   └── nodes/               # 各節點 Vue 組件
│   │   │       ├── HttpCallNode.vue
│   │   │       ├── ConditionNode.vue
│   │   │       ├── TransformNode.vue
│   │   │       ├── ForkNode.vue
│   │   │       ├── JoinNode.vue
│   │   │       ├── SubFlowNode.vue
│   │   │       └── ResponseNode.vue
│   │   ├── composables/
│   │   │   ├── useIRExport.ts       # vueFlowToIR() mapper
│   │   │   ├── useIRImport.ts       # irToVueFlow() + 自動佈局
│   │   │   ├── useFlowPersistence.ts# 存檔/載入/草稿
│   │   │   ├── useFlowValidator.ts  # 連線語義驗證
│   │   │   └── useIRExecutor.ts     # 瀏覽器端測試執行器
│   │   └── ir/
│   │       └── schema.ts            # Zod IR Schema + 型別定義
│   └── package.json
│
├── codegen/                         # Go 程式
│   ├── main.go
│   ├── ir/
│   │   └── types.go                 # IR 型別（與前端 schema 對應）
│   ├── targets/
│   │   ├── kong/                    # Kong Plugin (Lua) codegen
│   │   │   ├── generator.go
│   │   │   └── templates/
│   │   │       ├── plugin.lua.tmpl
│   │   │       └── handler.lua.tmpl
│   │   └── golang/                  # Go HTTP handler codegen
│   │       ├── generator.go
│   │       └── templates/
│   │           └── handler.go.tmpl
│   └── validator/
│       └── ir_validator.go          # IR 結構合法性驗證
│
└── api/                             # 後端 API (Java Spring Boot 或 Go)
    └── flows/                       # Flow CRUD + 觸發 codegen
```

---

## 四、Gateway IR Schema（完整 Zod 定義）

```typescript
// frontend/src/ir/schema.ts
import { z } from 'zod'

// ── 節點類型 ──────────────────────────────────────────

const UpstreamSchema = z.object({
  name:     z.string(),                 // 邏輯名稱（如 'user-service'）
  provider: z.enum(['kong', 'k8s-service', 'url']).default('kong'),
  url:      z.string().optional(),      // provider='url' 時直接指定
})

const HttpCallNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('http-call'),
  config: z.object({
    upstream: UpstreamSchema,            // 上游服務定義
    path:     z.string(),               // 支援 ${ctx.params.xxx} 插值
    method:   z.enum(['GET','POST','PUT','DELETE','PATCH']).default('GET'),
    timeout:  z.number().int().positive().default(3000),
    headers:  z.record(z.string()).optional(),
    body:     z.string().optional(),    // JSONata 表達式
    retry:    z.object({
      maxAttempts: z.number().int().default(1),
      backoff:     z.enum(['fixed', 'exponential']).default('fixed'),
      delay:       z.number().default(1000),
    }).optional(),
    fallback: z.object({
      strategy: z.enum(['default-value', 'skip', 'error']).default('error'),
      value:    z.any().optional(),     // strategy='default-value' 時的預設值
    }).optional(),
  }),
  outputVar: z.string(),                // 結果存入 ctx.vars.{outputVar}
})

const ConditionNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('condition'),
  config: z.object({
    expression: z.string(),             // JSONata boolean 表達式
  }),
  // trueBranch / falseBranch 由 edges.sourceHandle 推導，不存在 node 上
})

const TransformNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('transform'),
  config: z.object({
    engine:     z.enum(['jsonata', 'jmespath']).default('jsonata'),
    expression: z.string(),
  }),
  outputVar: z.string(),
})

const ForkNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('fork'),
  config: z.object({
    strategy: z.enum(['all', 'race', 'allSettled']).default('all'),
    // all = 等全部完成, race = 第一個完成就繼續, allSettled = 全部結束（含失敗）
    timeout:  z.number().optional(),    // 整體超時（ms）
  }),
})

const JoinNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('join'),
  config: z.object({
    strategy:   z.enum(['merge', 'array', 'custom']).default('merge'),
    expression: z.string().optional(),  // strategy='custom' 時的 JSONata
  }),
  outputVar: z.string(),
})

const SubFlowNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('sub-flow'),
  config: z.object({
    flowId:   z.string(),               // 引用另一個 GatewayIR 的 id
    inputMap: z.record(z.string()),      // 參數映射：sub-flow param → 當前 ctx 的值
  }),
  outputVar: z.string(),
})

const ResponseNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('response'),
  config: z.object({
    statusCode: z.number().int().default(200),
    body:       z.string(),             // JSONata，引用 ctx.vars
    headers:    z.record(z.string()).optional(),
  }),
})

// 聯合所有節點類型（discriminatedUnion 提供精確的型別縮窄）
export const IRNodeSchema = z.discriminatedUnion('type', [
  HttpCallNodeSchema,
  ConditionNodeSchema,
  TransformNodeSchema,
  ForkNodeSchema,
  JoinNodeSchema,
  SubFlowNodeSchema,
  ResponseNodeSchema,
])

export const IREdgeSchema = z.object({
  source:       z.string(),
  target:       z.string(),
  sourceHandle: z.string().nullable().optional(), // 'true' | 'false' | null
})

// ── 頂層 IR Schema ────────────────────────────────────

export const GatewayIRSchema = z.object({
  version: z.literal('1.0'),
  id:      z.string(),
  name:    z.string(),
  trigger: z.object({
    method: z.enum(['GET','POST','PUT','DELETE','PATCH','ANY']),
    path:   z.string(),                 // e.g. '/api/user/:userId'
  }),
  nodes:    z.array(IRNodeSchema).min(1),
  edges:    z.array(IREdgeSchema),
  executionHints: z.object({
    parallelGroups: z.array(z.array(z.string())).optional(), // [['n1','n2'], ['n3','n4']]
  }).optional(),
  metadata: z.object({
    createdAt: z.string().datetime(),
    updatedAt: z.string().datetime(),
    author:    z.string().optional(),
  }).optional(),
})

export type GatewayIR = z.infer<typeof GatewayIRSchema>
export type IRNode    = z.infer<typeof IRNodeSchema>
```

### IR 範例（User Info Aggregation）

```json
{
  "version": "1.0",
  "id": "flow-user-info-aggregation",
  "name": "取得用戶完整資訊",
  "trigger": { "method": "GET", "path": "/api/user/:userId" },
  "nodes": [
    {
      "id": "n1", "type": "http-call",
      "config": {
        "upstream": { "name": "user-service", "provider": "kong" },
        "path": "/users/${ctx.params.userId}",
        "timeout": 3000
      },
      "outputVar": "userInfo"
    },
    {
      "id": "n2", "type": "condition",
      "config": { "expression": "userInfo.role = 'vip'" }
    },
    {
      "id": "n3", "type": "http-call",
      "config": {
        "upstream": { "name": "vip-service", "provider": "kong" },
        "path": "/vip/${userInfo.id}"
      },
      "outputVar": "vipData"
    },
    {
      "id": "n4", "type": "response",
      "config": {
        "statusCode": 200,
        "body": "{ 'user': userInfo, 'vip': vipData }"
      }
    }
  ],
  "edges": [
    { "source": "n1", "target": "n2" },
    { "source": "n2", "target": "n3", "sourceHandle": "true" },
    { "source": "n2", "target": "n4", "sourceHandle": "false" },
    { "source": "n3", "target": "n4" }
  ]
}
```

---

## 五、Vue Flow 編輯器

### 安裝

```bash
npm install @vue-flow/core @vue-flow/minimap @vue-flow/controls @vue-flow/background zod nanoid
```

### 節點類型映射

```typescript
// 每個 key 對應一個 Vue 組件，組件內用 Handle 定義連接口
const nodeTypes = {
  'http-call':  HttpCallNode,
  'condition':  ConditionNode,
  'transform':  TransformNode,
  'fork':       ForkNode,
  'join':       JoinNode,
  'sub-flow':   SubFlowNode,
  'response':   ResponseNode,
}
```

### Handle 設計（condition 節點範例）

```vue
<!-- ConditionNode.vue -->
<template>
  <div class="node-condition">
    <Handle type="target" :position="Position.Left" />
    
    <div class="node-body">
      <label>條件表達式（JSONata）</label>
      <textarea v-model="data.expression" @change="emit('update', data)" />
    </div>

    <!-- 兩個 source Handle，id 對應 edge.sourceHandle -->
    <Handle id="true"  type="source" :position="Position.Right" style="top: 35%" />
    <Handle id="false" type="source" :position="Position.Right" style="top: 65%" />
    <span style="position:absolute; right:-30px; top:30%">T</span>
    <span style="position:absolute; right:-30px; top:60%">F</span>
  </div>
</template>
```

### 連線語義驗證（useFlowValidator.ts）

```typescript
import { useVueFlow } from '@vue-flow/core'

export function useFlowValidator() {
  const { addEdges } = useVueFlow()

  // 合法連線規則表
  const ALLOWED_CONNECTIONS: Record<string, string[]> = {
    'http-call':  ['condition', 'transform', 'join', 'response', 'fork'],
    'condition':  ['http-call', 'transform', 'fork', 'join', 'response', 'sub-flow'],
    'transform':  ['condition', 'join', 'response', 'fork'],
    'fork':       ['http-call', 'transform', 'condition', 'sub-flow'],  // fork 出去接多個並行節點
    'join':       ['transform', 'condition', 'response', 'fork'],
    'sub-flow':   ['condition', 'transform', 'join', 'response', 'fork'],
    'response':   [],  // response 不能有出邊
  }

  function onConnect(params: Connection) {
    const sourceType = getNode(params.source)?.type
    const targetType = getNode(params.target)?.type

    if (!sourceType || !targetType) return
    if (!ALLOWED_CONNECTIONS[sourceType]?.includes(targetType)) {
      console.warn(`不允許的連線：${sourceType} → ${targetType}`)
      return
    }
    addEdges([{
      ...params,
      // condition → true 分支顯示綠色 animated edge
      ...(params.sourceHandle === 'true'  && { style: { stroke: '#22c55e' }, animated: true }),
      ...(params.sourceHandle === 'false' && { style: { stroke: '#ef4444' }, animated: true }),
    }])
  }

  return { onConnect }
}
```

### FlowEditor.vue 骨架

```vue
<template>
  <VueFlow
    :node-types="nodeTypes"
    @connect="onConnect"
    @node-click="openConfigPanel"
    @nodes-change="autoSaveDraft"
  >
    <!-- 左側：節點拖拉庫 -->
    <Panel position="top-left">
      <NodePalette @drag-start="onDragStart" />
    </Panel>

    <!-- 右上：操作按鈕 -->
    <Panel position="top-right">
      <button @click="runTest">執行測試</button>
      <button @click="handleSave" :disabled="isSaving">儲存</button>
      <button @click="handleExport">導出 IR → Codegen</button>
    </Panel>

    <MiniMap :node-color="nodeColor" />
    <Controls />
    <Background variant="dots" />
  </VueFlow>

  <!-- 右側：節點配置面板（v-if 控制顯示） -->
  <ConfigPanel v-if="selectedNode" :node="selectedNode" @update="updateNodeData" />
</template>
```

---

## 六、序列化 / 反序列化

### vueFlowToIR()（序列化）

```typescript
// composables/useIRExport.ts
import { useVueFlow } from '@vue-flow/core'
import { GatewayIRSchema, type GatewayIR } from '@/ir/schema'

export function useIRExport() {
  const { toObject } = useVueFlow()

  function vueFlowToIR(flowId: string, name: string, trigger: GatewayIR['trigger']): GatewayIR {
    const snapshot = toObject()

    const irNodes = snapshot.nodes.map(node => {
      const base = { id: node.id, type: node.type }
      switch (node.type) {
        case 'http-call':
          return { ...base, config: { upstream: { name: node.data.upstream, provider: node.data.provider ?? 'kong',
            ...(node.data.url && { url: node.data.url }) }, path: node.data.path,
            method: node.data.method ?? 'GET', timeout: node.data.timeout ?? 3000,
            ...(node.data.headers && { headers: node.data.headers }),
            ...(node.data.body && { body: node.data.body }),
            ...(node.data.retry && { retry: node.data.retry }),
            ...(node.data.fallback && { fallback: node.data.fallback }),
          }, outputVar: node.data.outputVar }
        case 'condition':
          return { ...base, config: { expression: node.data.expression } }
        case 'transform':
          return { ...base, config: { engine: node.data.engine ?? 'jsonata',
            expression: node.data.expression }, outputVar: node.data.outputVar }
        case 'fork':
          return { ...base, config: { strategy: node.data.strategy ?? 'all',
            ...(node.data.timeout && { timeout: node.data.timeout }) } }
        case 'join':
          return { ...base, config: { strategy: node.data.strategy ?? 'merge',
            ...(node.data.expression && { expression: node.data.expression }),
          }, outputVar: node.data.outputVar }
        case 'sub-flow':
          return { ...base, config: { flowId: node.data.flowId,
            inputMap: node.data.inputMap }, outputVar: node.data.outputVar }
        case 'response':
          return { ...base, config: { statusCode: node.data.statusCode ?? 200,
            body: node.data.body, ...(node.data.headers && { headers: node.data.headers }) } }
        default:
          throw new Error(`未知節點類型：${node.type}`)
      }
    })

    const irEdges = snapshot.edges.map(edge => ({
      source: edge.source,
      target: edge.target,
      ...(edge.sourceHandle && { sourceHandle: edge.sourceHandle }),
    }))

    const now = new Date().toISOString()
    return GatewayIRSchema.parse({
      version: '1.0', id: flowId, name, trigger,
      nodes: irNodes, edges: irEdges,
      metadata: { createdAt: now, updatedAt: now },
    })
  }

  return { vueFlowToIR }
}
```

### irToVueFlow()（反序列化 + 自動佈局）

```typescript
// composables/useIRImport.ts
export function useIRImport() {
  const { fromObject, fitView } = useVueFlow()

  function irToVueFlow(ir: GatewayIR) {
    const depths = computeNodeDepths(ir)    // BFS 拓樸排序

    const nodes = ir.nodes.map(node => ({
      id:       node.id,
      type:     node.type,
      position: { x: 200, y: (depths.get(node.id) ?? 0) * 140 + 40 },
      data:     { ...('config' in node ? node.config : {}), outputVar: (node as any).outputVar },
    }))

    const edges = ir.edges.map((edge, i) => ({
      id: `e${i}`, source: edge.source, target: edge.target,
      ...(edge.sourceHandle && { sourceHandle: edge.sourceHandle }),
      ...(edge.sourceHandle === 'true'  && { style: { stroke: '#22c55e' }, animated: true }),
      ...(edge.sourceHandle === 'false' && { style: { stroke: '#ef4444' }, animated: true }),
    }))

    fromObject({ nodes, edges, viewport: { x: 0, y: 0, zoom: 1 } })
    setTimeout(() => fitView({ padding: 0.2 }), 50)
  }

  return { irToVueFlow }
}

// BFS 計算拓樸深度（用於自動佈局 y 座標）
function computeNodeDepths(ir: GatewayIR): Map<string, number> {
  const depths = new Map<string, number>()
  const inEdges = new Map<string, string[]>()
  ir.nodes.forEach(n => inEdges.set(n.id, []))
  ir.edges.forEach(e => inEdges.get(e.target)?.push(e.source))
  const roots = ir.nodes.filter(n => (inEdges.get(n.id) ?? []).length === 0).map(n => n.id)
  const queue = roots.map(id => ({ id, depth: 0 }))
  while (queue.length) {
    const { id, depth } = queue.shift()!
    if ((depths.get(id) ?? -1) >= depth) continue
    depths.set(id, depth)
    ir.edges.filter(e => e.source === id).forEach(e => queue.push({ id: e.target, depth: depth + 1 }))
  }
  return depths
}
```

### 持久化（存檔 / 草稿）

```typescript
// composables/useFlowPersistence.ts
export function useFlowPersistence(flowId: string) {
  const { vueFlowToIR } = useIRExport()
  const { irToVueFlow  } = useIRImport()
  const isSaving  = ref(false)
  const error     = ref<string | null>(null)

  async function save(name: string, trigger: GatewayIR['trigger']) {
    isSaving.value = true
    try {
      const ir = vueFlowToIR(flowId, name, trigger)
      await fetch(`/api/flows/${flowId}`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(ir),
      })
    } catch (e: any) {
      error.value = e.message   // ZodError 含詳細欄位路徑
    } finally { isSaving.value = false }
  }

  async function load() {
    const res  = await fetch(`/api/flows/${flowId}`)
    const json = await res.json()
    irToVueFlow(GatewayIRSchema.parse(json))  // 載入時同樣 Zod 驗證
  }

  // 本地草稿（每 30 秒自動存）
  function saveDraft(name: string, trigger: GatewayIR['trigger']) {
    try {
      const ir = vueFlowToIR(flowId, name, trigger)
      localStorage.setItem(`flow-draft:${flowId}`, JSON.stringify(ir))
    } catch { }
  }

  function loadDraft(): GatewayIR | null {
    const raw = localStorage.getItem(`flow-draft:${flowId}`)
    if (!raw) return null
    try { return GatewayIRSchema.parse(JSON.parse(raw)) }
    catch { localStorage.removeItem(`flow-draft:${flowId}`); return null }
  }

  return { save, load, saveDraft, loadDraft, isSaving, error }
}
```

---

## 七、瀏覽器端測試執行器

在 UI 直接執行 IR，不依賴後端，支援 mock 或真實 upstream：

```typescript
// composables/useIRExecutor.ts
import jsonata from 'jsonata'

interface ExecutionContext {
  params:  Record<string, any>   // trigger path params
  vars:    Record<string, any>   // 節點 outputVar 存放處
  request: Record<string, any>  // 原始 request 資訊
}

export function useIRExecutor() {
  async function execute(
    ir: GatewayIR,
    mockData: Record<string, any>,
    mockUpstreams?: Record<string, any>  // upstream name → mock response
  ) {
    const ctx: ExecutionContext = { params: mockData, vars: {}, request: mockData }

    // 建立節點查找表
    const nodeMap = new Map(ir.nodes.map(n => [n.id, n]))

    // 找根節點（無入邊的節點）
    const hasIncoming = new Set(ir.edges.map(e => e.target))
    let currentId = ir.nodes.find(n => !hasIncoming.has(n.id))?.id
    if (!currentId) throw new Error('找不到根節點')

    const visited = new Set<string>()

    while (currentId) {
      if (visited.has(currentId)) throw new Error(`偵測到循環：${currentId}`)
      visited.add(currentId)

      const node = nodeMap.get(currentId)
      if (!node) break

      switch (node.type) {
        case 'http-call': {
          // 開發環境用 mock，生產環境 call 真實 upstream
          const path = interpolate(node.config.path, ctx)
          const upstreamName = node.config.upstream.name
          if (mockUpstreams?.[upstreamName]) {
            ctx.vars[node.outputVar] = mockUpstreams[upstreamName]
          } else {
            const baseUrl = node.config.upstream.provider === 'url'
              ? node.config.upstream.url
              : `/proxy/${upstreamName}`
            const res = await fetch(`${baseUrl}${path}`, {
              method:  node.config.method ?? 'GET',
              headers: node.config.headers,
              signal:  AbortSignal.timeout(node.config.timeout ?? 3000),
            })
            ctx.vars[node.outputVar] = await res.json()
          }
          currentId = getNextNode(ir, currentId, null)
          break
        }

        case 'condition': {
          const result = await jsonata(node.config.expression).evaluate(ctx.vars)
          const handle = result ? 'true' : 'false'
          currentId = getNextNode(ir, currentId, handle)
          break
        }

        case 'transform': {
          const expr = await jsonata(node.config.expression).evaluate(ctx.vars)
          ctx.vars[node.outputVar] = expr
          currentId = getNextNode(ir, currentId, null)
          break
        }

        case 'fork': {
          // 找出 fork 的所有出邊目標，並行執行到各自的 join
          const branches = ir.edges.filter(e => e.source === currentId).map(e => e.target)
          const branchPromises = branches.map(async branchId => {
            // 簡化：每個分支只執行一個節點（完整版需遞迴執行到 join）
            const branchNode = nodeMap.get(branchId)
            if (branchNode?.type === 'http-call') {
              const path = interpolate(branchNode.config.path, ctx)
              const upstreamName = branchNode.config.upstream.name
              if (mockUpstreams?.[upstreamName]) {
                ctx.vars[branchNode.outputVar] = mockUpstreams[upstreamName]
              } else {
                const baseUrl = branchNode.config.upstream.provider === 'url'
                  ? branchNode.config.upstream.url
                  : `/proxy/${upstreamName}`
                const res = await fetch(`${baseUrl}${path}`, {
                  method: branchNode.config.method ?? 'GET',
                  signal: AbortSignal.timeout(branchNode.config.timeout ?? 3000),
                })
                ctx.vars[branchNode.outputVar] = await res.json()
              }
            }
          })

          if (node.config.strategy === 'race') {
            await Promise.race(branchPromises)
          } else if (node.config.strategy === 'allSettled') {
            await Promise.allSettled(branchPromises)
          } else {
            await Promise.all(branchPromises)
          }

          // fork 執行完後，找下一個 join 節點
          const joinEdge = ir.edges.find(e =>
            branches.some(b => {
              const outEdges = ir.edges.filter(oe => oe.source === b)
              return outEdges.some(oe => oe.target === e.target)
            }) && nodeMap.get(e.target)?.type === 'join'
          )
          currentId = joinEdge?.target
          break
        }

        case 'join': {
          // 合併所有入邊節點的 outputVar
          const inputVars = ir.edges
            .filter(e => e.target === currentId)
            .map(e => nodeMap.get(e.source))
            .filter(Boolean)
            .map(n => ctx.vars[(n as any).outputVar])
            .filter(Boolean)

          if (node.config.strategy === 'merge') {
            ctx.vars[node.outputVar] = Object.assign({}, ...inputVars)
          } else if (node.config.strategy === 'array') {
            ctx.vars[node.outputVar] = inputVars
          } else if (node.config.strategy === 'custom' && node.config.expression) {
            ctx.vars[node.outputVar] = await jsonata(node.config.expression).evaluate(ctx.vars)
          }
          currentId = getNextNode(ir, currentId, null)
          break
        }

        case 'response': {
          const body = await jsonata(node.config.body).evaluate(ctx.vars)
          return { statusCode: node.config.statusCode ?? 200, body, headers: node.config.headers }
        }
      }
    }

    throw new Error('流程未到達 response 節點')
  }

  return { execute }
}

function getNextNode(ir: GatewayIR, currentId: string, handle: string | null): string | undefined {
  const edge = ir.edges.find(e =>
    e.source === currentId && (handle === null || e.sourceHandle === handle || !e.sourceHandle)
  )
  return edge?.target
}

function interpolate(template: string, ctx: ExecutionContext): string {
  return template.replace(/\$\{ctx\.params\.(\w+)\}/g, (_, key) => ctx.params[key] ?? '')
}
```

---

## 八、Go Codegen

### IR 型別定義

```go
// codegen/ir/types.go
package ir

type GatewayIR struct {
    Version        string          `json:"version"`
    ID             string          `json:"id"`
    Name           string          `json:"name"`
    Trigger        Trigger         `json:"trigger"`
    Nodes          []Node          `json:"nodes"`
    Edges          []Edge          `json:"edges"`
    ExecutionHints *ExecutionHints `json:"executionHints,omitempty"`
}

type Trigger struct {
    Method string `json:"method"`
    Path   string `json:"path"`
}

type Upstream struct {
    Name     string `json:"name"`
    Provider string `json:"provider"` // "kong" | "k8s-service" | "url"
    URL      string `json:"url,omitempty"`
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
```

### Kong Plugin (Lua) 生成策略

```go
// codegen/targets/kong/generator.go
package kong

import (
    "text/template"
    "github.com/your-org/gateway-codegen/ir"
)

func Generate(flow ir.GatewayIR) (string, error) {
    // 1. 拓樸排序節點，確定執行順序
    ordered := topologicalSort(flow.Nodes, flow.Edges)

    // 2. JSONata 表達式全部走 runtime（luajit-jsonata），不做靜態翻譯
    // 3. 把 ${ctx.params.xxx} 插值轉成 kong.request.get_uri_captures()["xxx"]
    // 4. 渲染 Lua template（function-per-node 模式）

    tmpl := template.Must(template.ParseFiles("templates/plugin.lua.tmpl"))
    // ...render
}
```

**Lua template 設計（plugin.lua.tmpl）— function-per-node 模式：**

```lua
-- Auto-generated by gateway-codegen
-- Flow: {{ .Name }} ({{ .ID }})
-- DO NOT EDIT MANUALLY

local cjson    = require "cjson"
local http     = require "resty.http"
local jsonata  = require "resty.jsonata"  -- luajit-jsonata runtime

local _M = {}

-- ── 節點函數定義 ──────────────────────────────────────

local nodes = {}

{{ range .OrderedNodes }}
{{ if eq .Type "http-call" }}
-- Node: {{ .ID }} (http-call → {{ .Config.upstream.name }})
nodes["{{ .ID }}"] = function(ctx)
  local httpc = http.new()
  local path  = {{ pathExpr .Config.path }}
  local res, err = httpc:request_uri(
    "http://{{ .Config.upstream.name }}" .. path,
    { method = "{{ .Config.method }}", timeout = {{ .Config.timeout }} }
  )
  if err then
    {{ if .Config.fallback }}
    ctx.vars["{{ .OutputVar }}"] = {{ fallbackValue .Config.fallback }}
    {{ else }}
    kong.response.exit(502, { error = err })
    {{ end }}
  else
    ctx.vars["{{ .OutputVar }}"] = cjson.decode(res.body)
  end
  return "{{ .NextNode }}"
end

{{ else if eq .Type "condition" }}
-- Node: {{ .ID }} (condition)
nodes["{{ .ID }}"] = function(ctx)
  local result = jsonata.evaluate({{ quote .Config.expression }}, ctx.vars)
  if result then
    return "{{ .TrueBranch }}"
  else
    return "{{ .FalseBranch }}"
  end
end

{{ else if eq .Type "transform" }}
-- Node: {{ .ID }} (transform)
nodes["{{ .ID }}"] = function(ctx)
  ctx.vars["{{ .OutputVar }}"] = jsonata.evaluate({{ quote .Config.expression }}, ctx.vars)
  return "{{ .NextNode }}"
end

{{ else if eq .Type "fork" }}
-- Node: {{ .ID }} (fork, strategy={{ .Config.strategy }})
nodes["{{ .ID }}"] = function(ctx)
  local threads = {}
  {{ range .Branches }}
  table.insert(threads, ngx.thread.spawn(function()
    local next = "{{ . }}"
    while next and nodes[next] do
      local node_type = "{{ nodeType . }}"
      if node_type == "join" then break end
      next = nodes[next](ctx)
    end
  end))
  {{ end }}
  for _, t in ipairs(threads) do ngx.thread.wait(t) end
  return "{{ .JoinNode }}"
end

{{ else if eq .Type "join" }}
-- Node: {{ .ID }} (join)
nodes["{{ .ID }}"] = function(ctx)
  {{ if eq .Config.strategy "merge" }}
  local merged = {}
  {{ range .InputVars }}
  if ctx.vars["{{ . }}"] then
    for k, v in pairs(ctx.vars["{{ . }}"]) do merged[k] = v end
  end
  {{ end }}
  ctx.vars["{{ .OutputVar }}"] = merged
  {{ else if eq .Config.strategy "array" }}
  ctx.vars["{{ .OutputVar }}"] = { {{ range .InputVars }}ctx.vars["{{ . }}"], {{ end }} }
  {{ else if eq .Config.strategy "custom" }}
  ctx.vars["{{ .OutputVar }}"] = jsonata.evaluate({{ quote .Config.expression }}, ctx.vars)
  {{ end }}
  return "{{ .NextNode }}"
end

{{ else if eq .Type "response" }}
-- Node: {{ .ID }} (response)
nodes["{{ .ID }}"] = function(ctx)
  local body = jsonata.evaluate({{ quote .Config.body }}, ctx.vars)
  kong.response.exit({{ .Config.statusCode }}, body)
  return nil
end
{{ end }}
{{ end }}

-- ── Dispatcher ────────────────────────────────────────

function _M.execute(kong)
  local ctx = {
    params = kong.request.get_uri_captures() or {},
    vars   = {},
  }

  local next_node = "{{ .RootNode }}"
  while next_node do
    local fn = nodes[next_node]
    if not fn then
      kong.response.exit(500, { error = "unknown node: " .. next_node })
      return
    end
    next_node = fn(ctx)
  end
end

return _M
```

### Go Handler 生成策略

```go
// codegen/targets/golang/generator.go
// Go template 生成 net/http handler

const handlerTemplate = `
// Auto-generated — DO NOT EDIT
package handler

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
    "github.com/blues/jsonata-go"
    "github.com/your-org/gateway-runtime/upstream"
    "golang.org/x/sync/errgroup"
)

func Handle{{ .FuncName }}(w http.ResponseWriter, r *http.Request) {
    vars := make(map[string]any)
    mu   := &sync.Mutex{}  // 保護 vars 在並行寫入時的安全

    {{- range .OrderedNodes }}
    {{- if eq .Type "http-call" }}
    // {{ .ID }}: call {{ .Config.upstream.name }}
    {{ .OutputVar }}, err := upstream.Call("{{ .Config.upstream.name }}",
        fmt.Sprintf("{{ goPathExpr .Config.path }}", {{ pathArgs .Config.path }}),
        upstream.WithMethod("{{ .Config.method }}"),
        upstream.WithTimeout({{ .Config.timeout }}*time.Millisecond),
        upstream.WithProvider("{{ .Config.upstream.provider }}"),
    )
    if err != nil {
        {{- if .Config.fallback }}
        {{ .OutputVar }} = {{ goFallbackValue .Config.fallback }}
        {{- else }}
        http.Error(w, err.Error(), 502); return
        {{- end }}
    }
    vars["{{ .OutputVar }}"] = {{ .OutputVar }}

    {{- else if eq .Type "condition" }}
    // {{ .ID }}: condition
    if {{ goConditionExpr .Config.expression }} {
        goto {{ .TrueBranch }}
    } else {
        goto {{ .FalseBranch }}
    }

    {{- else if eq .Type "transform" }}
    // {{ .ID }}: transform
    {
        expr := jsonata.MustCompile(` + "`" + `{{ .Config.expression }}` + "`" + `)
        result, err := expr.Eval(vars)
        if err != nil { http.Error(w, err.Error(), 500); return }
        vars["{{ .OutputVar }}"] = result
    }

    {{- else if eq .Type "fork" }}
    // {{ .ID }}: fork (strategy={{ .Config.strategy }})
    {
        g, _ := errgroup.WithContext(r.Context())
        {{- range .Branches }}
        g.Go(func() error {
            {{- /* 每個分支的節點序列由 codegen 展開 */ -}}
            {{ goBranchCode . }}
            return nil
        })
        {{- end }}
        if err := g.Wait(); err != nil {
            http.Error(w, err.Error(), 502); return
        }
    }

    {{- else if eq .Type "join" }}
    // {{ .ID }}: join (strategy={{ .Config.strategy }})
    {
        {{- if eq .Config.strategy "merge" }}
        merged := make(map[string]any)
        {{- range .InputVars }}
        if v, ok := vars["{{ . }}"]; ok {
            if m, ok := v.(map[string]any); ok {
                for k, val := range m { merged[k] = val }
            }
        }
        {{- end }}
        vars["{{ .OutputVar }}"] = merged
        {{- else if eq .Config.strategy "array" }}
        vars["{{ .OutputVar }}"] = []any{ {{- range .InputVars }}vars["{{ . }}"], {{- end }} }
        {{- else if eq .Config.strategy "custom" }}
        expr := jsonata.MustCompile(` + "`" + `{{ .Config.expression }}` + "`" + `)
        result, _ := expr.Eval(vars)
        vars["{{ .OutputVar }}"] = result
        {{- end }}
    }

    {{- else if eq .Type "response" }}
{{ .ID }}:
    w.WriteHeader({{ .Config.statusCode }})
    json.NewEncoder(w).Encode({{ goResponseExpr .Config.body }})
    return
    {{- end }}
    {{- end }}
}
`
```

---

## 九、Codegen API（後端接口）

```
POST /api/codegen
Content-Type: application/json

{
  "ir":     { ...GatewayIR },
  "target": "golang" | "kong-plugin"
}

Response:
{
  "code":          "...生成的程式碼字串...",
  "filename":      "handler.go" | "plugin.lua",
  "prerequisites": {
    "upstreams": ["user-service", "vip-service"]   // 需要預先存在的 upstream
  },
  "warnings":      []
}
```

---

## 十、設計決策（2026-04-07 討論確認）

### 前端層

1. **拖拉新增節點的實作**：NodePalette 用 HTML5 drag & drop，VueFlow 容器的 `@drop` handler 使用 `screenToFlowCoordinate({ x: event.clientX, y: event.clientY })` 做座標轉換。不使用 `event.offsetX/Y`（相對於觸發元素，容易出錯）。VueFlow 內部已處理 viewport transform，不會有 offset 偏移問題。容器需加 `@dragover.prevent` 否則 drop 事件不觸發。

2. **ConfigPanel 的更新機制**：ConfigPanel 內部用 `@change` 收集修改，呼叫 `useVueFlow().updateNodeData(id, newData)` 回寫（shallow merge）。自動存草稿不依賴 `@nodes-change`（該事件主要是 position/dimension/select 變化），改用 `watchDebounced` 監聽 `getNodes` + `getEdges`，每 30 秒或 data 變化時存 localStorage。

3. **condition 節點的 Handle 標示**：三層標示確保用戶不會混淆——Handle 本身用顏色區分（true: `#22c55e` 綠色，false: `#ef4444` 紅色，與 edge 顏色一致）、Handle 旁加 T/F 文字、hover 顯示 tooltip（「條件為真」/「條件為假」）。節點 body 內顯示表達式摘要（截斷前 30 字元）。

4. **Fork/Join 節點取代 Aggregate**：移除 `AggregateNodeSchema`，改用 `fork` + `join` 兩個節點顯式表達並行語義。fork 的出邊數量 ≥ 2，每條出邊代表一個並行分支。join 的 target Handle 天然支援多條入邊（VueFlow 原生支援），UI 上加 badge 顯示已連接的輸入數量。

5. **測試執行結果的 UI 呈現**：Phase 1 在每個節點右下角顯示狀態 badge（✓ 綠色 / ✗ 紅色 / ⏳ 執行中），點擊展開 JSON 預覽（`JSON.stringify(data, null, 2)`）。執行器改為 yield 每步結果（callback），寫入 reactive state 供節點組件 watch。Phase 3 加底部 output panel 顯示完整執行 trace。

### Codegen 層

6. **JSONata → Lua 轉譯策略**：全部走 runtime 執行（luajit-jsonata），不做靜態翻譯。避免維護「靜態翻譯器 + runtime 執行器」兩套路徑，以及「簡單」與「複雜」表達式分界線模糊的問題。Lua codegen 排在 Go 之後（Phase 3），先用 Go codegen 驗證 IR 設計合理性，再移植到 Lua。需先驗證 luajit-jsonata 在 OpenResty/Kong 環境的相容性。

7. **JSONata → Go 轉譯策略**：全部走 runtime 執行（`github.com/blues/jsonata-go`）。Go 是靜態型別語言，靜態翻譯 JSONata → Go 的 transpiler 複雜度過高（型別推導、null safety、陣列操作），且 `jsonata-go` 效能足夠。表達式以字串形式嵌入生成的 Go code，runtime evaluate。

8. **Lua 程式碼生成模式**：採用 function-per-node 模式，取代 flat goto。每個節點生成一個 `local function`，由 dispatcher 按拓樸順序呼叫，每個 function 回傳下一個 node id。好處：獨立 scope 無 scoping 問題、容易加 debug logging（wrap function）、未來加 retry/timeout per node 自然。fork 節點用 `ngx.thread.spawn` + `ngx.thread.wait` 實現並行。

9. **upstream 抽象層**：IR 中 upstream 改為 object `{ name, provider, url? }`，`provider` 支援 `'kong' | 'k8s-service' | 'url'`，預留多 target 擴充。Phase 1 只實作 `kong` provider。不在 IR 中記錄 host:port（由 Kong upstream 動態管理）。codegen response 加 `prerequisites` 欄位列出需要預先存在的 upstream 名稱，方便 CI/CD 檢查。

10. **並行調用**：由 fork/join 顯式節點表達，用戶在 UI 上明確畫出並行結構，不依賴 codegen 自動偵測。IR 中加 optional 的 `executionHints.parallelGroups` 供 codegen 參考，前端 UI 可顯示偵測結果讓用戶手動調整。Lua 用 `ngx.thread.spawn/wait`，Go 用 `errgroup.Group` 實現並行。

### 部署層

11. **codegen 服務部署方式**：Phase 1-2 做 CLI 工具（`gateway-codegen generate --target kong --input flow.json --output plugin.lua`），可直接整合 CI pipeline。Phase 3 加 HTTP service（前端「導出」按鈕直接 call API，支援多租戶、codegen cache）。核心邏輯一開始就抽成 Go package（`codegen/core/`），CLI 和 HTTP service 都是 thin wrapper，避免 Phase 3 重構。

12. **Kong 部署方式**：以 decK（declarative config）為主，支援 git-based workflow、diff 預覽、CI/CD 友好。流程：codegen 生成 plugin.lua → 自動生成對應 kong.yaml → `deck diff` 預覽 → `deck sync` 套用。提供 `--deploy-mode=admin-api` 選項給需要即時部署的場景。

### API Composition 擴充

13. **sub-flow 節點**：新增 `sub-flow` 節點類型，引用另一個 GatewayIR 的 id，透過 `inputMap` 做參數映射。採用 **inline 展開**策略——codegen 時 flatten 成單一流程，不做 runtime resolve。展開時處理：node id 加 prefix 避免衝突、outputVar 加 namespace、inputMap 替換 `${ctx.params.xxx}` 插值、sub-flow 的 response 節點替換為寫入 outputVar。防護機制：循環引用偵測（visited set）、展開深度限制（預設 5 層）、展開後重新 Zod 驗證。sub-flow resolve 順序：先查 API（已儲存的 flow）→ 再查 localStorage 草稿（`flow-draft:{flowId}`），確保開發中尚未存檔的 sub-flow 也能被引用。

14. **http-call 錯誤處理**：http-call 節點新增 optional 的 `retry`（maxAttempts / backoff / delay）和 `fallback`（default-value / skip / error）配置。Phase 3 可加 `circuitBreaker`。確保 API Composition 中單一上游失敗不會導致整個 response 失敗。

---

## 十一、技術棧版本

| 套件 | 版本 | 用途 |
|------|------|------|
| `@vue-flow/core` | ^1.x | 流程圖引擎 |
| `@vue-flow/minimap` | ^1.x | 縮略圖 |
| `@vue-flow/controls` | ^1.x | 縮放控制 |
| `zod` | ^3.x | IR schema 驗證 |
| `jsonata` | ^2.x | 瀏覽器端表達式執行 |
| `nanoid` | ^5.x | 節點 ID 生成 |
| Go | 1.21+ | Codegen service |
| `github.com/blues/jsonata-go` | latest | Go 端 JSONata runtime |
| `luajit-jsonata` | latest | Lua 端 JSONata runtime |

---

## 十二、開發進度與規劃

### 已完成

```
Phase 1（MVP）✅ 2026-04-07
  ├── IR Schema 定義（Zod）— 8 種節點（含 switch）
  ├── Vue Flow 編輯器（8 種節點 + 連線驗證 + 拖拉 + ConfigPanel）
  ├── vueFlowToIR() + irToVueFlow()（序列化往返 + 自動佈局）
  └── 瀏覽器端測試執行器（mock upstream + 執行 trace）

Phase 2（Go Codegen）✅ 2026-04-07
  ├── Go codegen CLI（codegen/cmd/main.go）
  │   └── gateway-codegen -input flow.json [-target golang] [-output handler.go]
  ├── Go handler 生成（codegen/targets/golang/generator.go）
  │   ├── 8 種節點全支援（含 switch 多路分支）
  │   ├── 智慧 label 生成（只有 goto 目標才有 label）
  │   ├── fork/join — errgroup 真正並行 + mu.Lock 保護
  │   └── JSONata 表達式 runtime evaluate（jsonata-go）
  ├── codegen prerequisites 檢查（upstream 清單）
  ├── runtime/upstream.go — MockUpstream + HTTPUpstream
  └── demo/main.go — IR interpreter server（E2E 驗證通過）

Phase 3（API Composition）✅ 2026-04-07
  ├── sub-flow inline 展開（useSubFlowExpander.ts）
  │   ├── 循環引用偵測 + 深度限制（5 層）
  │   ├── Node ID / outputVar prefix 避免衝突
  │   └── resolve 順序：API → localStorage 草稿
  ├── fork 並行升級
  │   ├── Go codegen: 分支邏輯 inline 進 errgroup goroutine
  │   └── 前端執行器: Promise.all / race / allSettled
  └── http-call retry（maxAttempts + delay）+ fallback（default-value / skip / error）

Phase 4（完整功能）✅ 2026-04-08
  ├── ConfigPanel 完整表單
  │   ├── 所有 8 種節點的完整欄位（含 upstream provider/url）
  │   ├── 條件式欄位顯示（retry 開關、custom expression）
  │   ├── Hints tooltip、checkbox toggle
  │   └── 內嵌執行結果顯示（output + duration + error）
  ├── 執行結果視覺化
  │   ├── 節點 CSS class: exec-success / exec-error / exec-running（pulse 動畫）
  │   └── watch nodeResults → 即時更新節點外觀
  └── Kong Plugin Lua codegen（targets/kong/generator.go）
      ├── function-per-node + dispatcher 模式
      ├── fork: ngx.thread.spawn/wait 並行
      ├── JSONata 全 runtime（resty.jsonata）
      └── CLI: -target kong 可用
```

### 待開發

```
Phase 5（部署整合 + 進階功能）✅ 2026-04-08
  ├── codegen HTTP service（server/server.go）
  │   ├── POST /api/codegen — golang | kong targets
  │   ├── SHA256 response cache
  │   └── CORS middleware + health endpoint
  ├── Kong 部署整合
  │   ├── targets/kong/deck.go — 生成 kong.yaml（decK declarative config）
  │   └── CLI --deck flag
  └── 前端 codegen 整合
      ├── ⚙ Go / ⚙ Lua toolbar buttons → call codegen API
      ├── Codegen result modal + Download
      └── Vite proxy → codegen service

未來可擴充：
  - circuit breaker
  - upstream provider 擴充（k8s-service, url）
  - A/B testing（同一 trigger 多版本）
  - 監控儀表板
```
