# Gateway IR Specification v1.0

Gateway IR（Intermediate Representation）是 Boxer 系統的核心合約，定義了 API Gateway 流程的資料結構。前端編輯器產出 IR JSON，codegen 消費 IR JSON 生成目標程式碼。

- 前端定義：`frontend/src/ir/schema.ts`（Zod）
- Go 對應型別：`codegen/ir/types.go`

---

## 頂層結構

```json
{
  "version": "1.0",
  "id": "string",
  "name": "string",
  "trigger": { ... },
  "nodes": [ ... ],
  "edges": [ ... ],
  "executionHints": { ... },   // optional
  "metadata": { ... }          // optional
}
```

| 欄位 | 型別 | 必填 | 說明 |
|------|------|------|------|
| `version` | `"1.0"` | ✅ | IR 版本，固定為 `"1.0"` |
| `id` | string | ✅ | 流程唯一識別碼 |
| `name` | string | ✅ | 流程名稱（顯示用） |
| `trigger` | Trigger | ✅ | 觸發條件（HTTP method + path） |
| `nodes` | Node[] | ✅ | 節點陣列，至少 1 個 |
| `edges` | Edge[] | ✅ | 邊陣列，定義節點間的連接 |
| `executionHints` | ExecutionHints | ❌ | 執行提示（如並行群組） |
| `metadata` | Metadata | ❌ | 元資料（建立/更新時間、作者） |

---

## Trigger

```json
{
  "method": "GET",
  "path": "/api/user/:userId"
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `method` | `"GET"` \| `"POST"` \| `"PUT"` \| `"DELETE"` \| `"PATCH"` \| `"ANY"` | HTTP method |
| `path` | string | 路由路徑，支援 `:param` 格式的路徑參數 |

---

## Edge

```json
{
  "source": "n1",
  "target": "n2",
  "sourceHandle": "true"
}
```

| 欄位 | 型別 | 必填 | 說明 |
|------|------|------|------|
| `source` | string | ✅ | 來源節點 ID |
| `target` | string | ✅ | 目標節點 ID |
| `sourceHandle` | string \| null | ❌ | 來源 handle ID，用於分支節點 |

`sourceHandle` 的值依來源節點類型而定：

| 來源節點 | sourceHandle 值 |
|----------|----------------|
| condition | `"true"` 或 `"false"` |
| switch | `"case:0"`, `"case:1"`, ..., `"default"` |
| 其他 | `null` 或省略 |

---

## 節點類型

所有節點共用欄位：

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | string | 節點唯一識別碼 |
| `type` | string | 節點類型（discriminator） |
| `config` | object | 節點配置（依類型不同） |
| `outputVar` | string | 輸出變數名稱（部分節點） |

---

### http-call

呼叫上游服務，將回應存入 `ctx.vars[outputVar]`。

```json
{
  "id": "n1",
  "type": "http-call",
  "config": {
    "upstream": { "name": "user-service", "provider": "kong" },
    "path": "/users/${ctx.params.userId}",
    "method": "GET",
    "timeout": 3000,
    "headers": { "X-Request-Id": "abc" },
    "body": "{ 'key': value }",
    "retry": { "maxAttempts": 3, "backoff": "fixed", "delay": 1000 },
    "fallback": { "strategy": "default-value", "value": {} }
  },
  "outputVar": "userInfo"
}
```

**config：**

| 欄位 | 型別 | 必填 | 預設 | 說明 |
|------|------|------|------|------|
| `upstream` | Upstream | ✅ | — | 上游服務定義 |
| `path` | string | ✅ | — | 請求路徑，支援 `${ctx.params.xxx}` 插值 |
| `method` | enum | ❌ | `"GET"` | HTTP method |
| `timeout` | number | ❌ | `3000` | 超時（毫秒） |
| `headers` | Record\<string, string\> | ❌ | — | 額外 HTTP headers |
| `body` | string | ❌ | — | Request body（JSONata 表達式） |
| `retry` | RetryConfig | ❌ | — | 重試配置 |
| `fallback` | FallbackConfig | ❌ | — | 失敗回退策略 |

**Upstream：**

| 欄位 | 型別 | 必填 | 預設 | 說明 |
|------|------|------|------|------|
| `name` | string | ✅ | — | 上游服務邏輯名稱 |
| `provider` | `"kong"` \| `"k8s-service"` \| `"url"` | ❌ | `"kong"` | 服務發現方式 |
| `url` | string | ❌ | — | `provider="url"` 時的完整 URL |

**RetryConfig：**

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `maxAttempts` | number | `1` | 最大嘗試次數 |
| `backoff` | `"fixed"` \| `"exponential"` | `"fixed"` | 退避策略 |
| `delay` | number | `1000` | 重試間隔（毫秒） |

**FallbackConfig：**

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `strategy` | `"default-value"` \| `"skip"` \| `"error"` | `"error"` | 回退策略 |
| `value` | any | — | `strategy="default-value"` 時的預設值 |

---

### condition

布林分支，依 JSONata 表達式結果走 `true` 或 `false` 分支。

```json
{
  "id": "n2",
  "type": "condition",
  "config": {
    "expression": "userInfo.role = 'vip'"
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `expression` | string | JSONata 布林表達式，對 `ctx.vars` 求值 |

出邊透過 `sourceHandle` 區分：`"true"` / `"false"`。

---

### switch

多路分支，依 JSONata 表達式結果匹配 cases。

```json
{
  "id": "n2",
  "type": "switch",
  "config": {
    "expression": "order.type",
    "cases": ["physical", "digital", "subscription"],
    "hasDefault": true
  }
}
```

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `expression` | string | — | JSONata 表達式，求值結果轉為字串後匹配 cases |
| `cases` | string[] | — | 匹配值列表 |
| `hasDefault` | boolean | `true` | 是否有 default 分支 |

出邊透過 `sourceHandle` 區分：`"case:0"`, `"case:1"`, ..., `"default"`。

---

### transform

資料轉換，將表達式結果存入 `ctx.vars[outputVar]`。

```json
{
  "id": "n3",
  "type": "transform",
  "config": {
    "engine": "jsonata",
    "expression": "{ 'user': userInfo, 'level': 'premium' }"
  },
  "outputVar": "result"
}
```

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `engine` | `"jsonata"` \| `"jmespath"` | `"jsonata"` | 表達式引擎 |
| `expression` | string | — | 轉換表達式，對 `ctx.vars` 求值 |

---

### fork

並行分支起點，所有出邊的目標節點並行執行。

```json
{
  "id": "n1",
  "type": "fork",
  "config": {
    "strategy": "all",
    "timeout": 10000
  }
}
```

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `strategy` | `"all"` \| `"race"` \| `"allSettled"` | `"all"` | 並行策略 |
| `timeout` | number | — | 整體超時（毫秒），省略表示無限 |

策略說明：
- `all` — 等全部分支完成（任一失敗則整體失敗）
- `race` — 第一個完成的分支即繼續
- `allSettled` — 等全部分支結束（含失敗）

Codegen 實作：Go 用 `errgroup`，Lua 用 `ngx.thread.spawn/wait`。

---

### join

並行分支合併點，收集所有入邊節點的 outputVar 並合併。

```json
{
  "id": "n5",
  "type": "join",
  "config": {
    "strategy": "merge",
    "expression": ""
  },
  "outputVar": "dashboard"
}
```

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `strategy` | `"merge"` \| `"array"` \| `"custom"` | `"merge"` | 合併策略 |
| `expression` | string | — | `strategy="custom"` 時的 JSONata 表達式 |

策略說明：
- `merge` — `Object.assign({}, ...inputVars)`，淺合併所有入邊的 outputVar
- `array` — `[inputVar1, inputVar2, ...]`，收集為陣列
- `custom` — 用 JSONata 表達式自訂合併邏輯

---

### sub-flow

引用另一個流程，codegen 時 inline 展開為單一流程。

```json
{
  "id": "n6",
  "type": "sub-flow",
  "config": {
    "flowId": "flow-get-user-detail",
    "inputMap": { "userId": "ctx.vars.currentUserId" }
  },
  "outputVar": "userDetail"
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `flowId` | string | 被引用的 GatewayIR ID |
| `inputMap` | Record\<string, string\> | 參數映射：sub-flow 的 param → 當前 ctx 的值 |

展開規則：
- Node ID 加 prefix（`{subFlowNodeId}__{originalId}`）避免衝突
- outputVar 加 namespace
- inputMap 替換 `${ctx.params.xxx}` 插值
- sub-flow 的 response 節點替換為 transform（寫入 outputVar）
- 循環引用偵測（visited set）
- 展開深度限制：5 層
- resolve 順序：API → localStorage 草稿

---

### response

回傳 HTTP response，終端節點（無出邊）。

```json
{
  "id": "n5",
  "type": "response",
  "config": {
    "statusCode": 200,
    "body": "result",
    "headers": { "X-Custom": "value" }
  }
}
```

| 欄位 | 型別 | 預設 | 說明 |
|------|------|------|------|
| `statusCode` | number | `200` | HTTP status code |
| `body` | string | — | Response body（JSONata 表達式，對 `ctx.vars` 求值） |
| `headers` | Record\<string, string\> | — | 額外 response headers |

---

## ExecutionHints

```json
{
  "executionHints": {
    "parallelGroups": [["n2", "n3"], ["n5", "n6"]]
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `parallelGroups` | string[][] | 可並行執行的節點群組，供 codegen 參考 |

---

## Metadata

```json
{
  "metadata": {
    "createdAt": "2026-04-07T10:00:00.000Z",
    "updatedAt": "2026-04-07T12:00:00.000Z",
    "author": "alice"
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `createdAt` | string (ISO 8601) | 建立時間 |
| `updatedAt` | string (ISO 8601) | 最後更新時間 |
| `author` | string | 作者（選填） |

---

## 執行上下文（Runtime Context）

節點執行時共享一個 context 物件：

```
ctx = {
  params:  { userId: "42", ... }     // trigger path params
  vars:    { userInfo: {...}, ... }   // 節點 outputVar 存放處
  request: { ... }                   // 原始 request 資訊
}
```

- `${ctx.params.xxx}` — 在 http-call 的 `path` 中插值
- JSONata 表達式（condition / switch / transform / response body）對 `ctx.vars` 求值
- 每個有 `outputVar` 的節點執行後，結果寫入 `ctx.vars[outputVar]`

---

## 連線規則

| 來源節點 | 可連接的目標節點 |
|----------|----------------|
| http-call | condition, switch, transform, join, response, fork |
| condition | http-call, transform, fork, join, response, sub-flow |
| switch | http-call, transform, fork, join, response, sub-flow |
| transform | condition, switch, join, response, fork |
| fork | http-call, transform, condition, switch, sub-flow |
| join | transform, condition, switch, response, fork |
| sub-flow | condition, switch, transform, join, response, fork |
| response | （無，終端節點） |

---

## 完整範例

### 條件分支

```json
{
  "version": "1.0",
  "id": "flow-user-profile",
  "name": "取得用戶資訊",
  "trigger": { "method": "GET", "path": "/api/user/:userId" },
  "nodes": [
    { "id": "n1", "type": "http-call", "config": { "upstream": { "name": "user-service", "provider": "kong" }, "path": "/users/${ctx.params.userId}", "method": "GET", "timeout": 3000 }, "outputVar": "userInfo" },
    { "id": "n2", "type": "condition", "config": { "expression": "userInfo.role = 'vip'" } },
    { "id": "n3", "type": "transform", "config": { "engine": "jsonata", "expression": "{ 'user': userInfo, 'level': 'premium' }" }, "outputVar": "result" },
    { "id": "n4", "type": "transform", "config": { "engine": "jsonata", "expression": "{ 'user': userInfo, 'level': 'standard' }" }, "outputVar": "result" },
    { "id": "n5", "type": "response", "config": { "statusCode": 200, "body": "result" } }
  ],
  "edges": [
    { "source": "n1", "target": "n2" },
    { "source": "n2", "target": "n3", "sourceHandle": "true" },
    { "source": "n2", "target": "n4", "sourceHandle": "false" },
    { "source": "n3", "target": "n5" },
    { "source": "n4", "target": "n5" }
  ]
}
```

### Switch 多路分支

```json
{
  "version": "1.0",
  "id": "flow-order-switch",
  "name": "訂單類型路由",
  "trigger": { "method": "POST", "path": "/api/order/:orderId" },
  "nodes": [
    { "id": "n1", "type": "http-call", "config": { "upstream": { "name": "order-service", "provider": "kong" }, "path": "/orders/${ctx.params.orderId}", "method": "GET", "timeout": 3000 }, "outputVar": "order" },
    { "id": "n2", "type": "switch", "config": { "expression": "order.type", "cases": ["physical", "digital", "subscription"], "hasDefault": true } },
    { "id": "n3", "type": "http-call", "config": { "upstream": { "name": "shipping-service", "provider": "kong" }, "path": "/ship", "method": "POST", "timeout": 5000 }, "outputVar": "result" },
    { "id": "n4", "type": "http-call", "config": { "upstream": { "name": "download-service", "provider": "kong" }, "path": "/license", "method": "POST", "timeout": 3000 }, "outputVar": "result" },
    { "id": "n5", "type": "http-call", "config": { "upstream": { "name": "subscription-service", "provider": "kong" }, "path": "/subscribe", "method": "POST", "timeout": 3000 }, "outputVar": "result" },
    { "id": "n6", "type": "response", "config": { "statusCode": 400, "body": "{ 'error': 'unknown type' }" } },
    { "id": "n7", "type": "response", "config": { "statusCode": 200, "body": "{ 'orderId': order.id, 'result': result }" } }
  ],
  "edges": [
    { "source": "n1", "target": "n2" },
    { "source": "n2", "target": "n3", "sourceHandle": "case:0" },
    { "source": "n2", "target": "n4", "sourceHandle": "case:1" },
    { "source": "n2", "target": "n5", "sourceHandle": "case:2" },
    { "source": "n2", "target": "n6", "sourceHandle": "default" },
    { "source": "n3", "target": "n7" },
    { "source": "n4", "target": "n7" },
    { "source": "n5", "target": "n7" }
  ]
}
```

### Fork/Join 並行

```json
{
  "version": "1.0",
  "id": "flow-dashboard",
  "name": "Dashboard 聚合",
  "trigger": { "method": "GET", "path": "/api/dashboard/:userId" },
  "nodes": [
    { "id": "n1", "type": "fork", "config": { "strategy": "all" } },
    { "id": "n2", "type": "http-call", "config": { "upstream": { "name": "user-service", "provider": "kong" }, "path": "/users/${ctx.params.userId}", "method": "GET", "timeout": 3000 }, "outputVar": "user" },
    { "id": "n3", "type": "http-call", "config": { "upstream": { "name": "order-service", "provider": "kong" }, "path": "/orders?userId=${ctx.params.userId}", "method": "GET", "timeout": 3000 }, "outputVar": "orders" },
    { "id": "n4", "type": "http-call", "config": { "upstream": { "name": "notification-service", "provider": "kong" }, "path": "/notifications?userId=${ctx.params.userId}", "method": "GET", "timeout": 3000 }, "outputVar": "notifications" },
    { "id": "n5", "type": "join", "config": { "strategy": "merge" }, "outputVar": "dashboard" },
    { "id": "n6", "type": "response", "config": { "statusCode": 200, "body": "dashboard" } }
  ],
  "edges": [
    { "source": "n1", "target": "n2" },
    { "source": "n1", "target": "n3" },
    { "source": "n1", "target": "n4" },
    { "source": "n2", "target": "n5" },
    { "source": "n3", "target": "n5" },
    { "source": "n4", "target": "n5" },
    { "source": "n5", "target": "n6" }
  ]
}
```
