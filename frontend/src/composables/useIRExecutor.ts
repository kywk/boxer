// frontend/src/composables/useIRExecutor.ts
import jsonata from 'jsonata'
import { ref, type Ref } from 'vue'
import type { GatewayIR, IRNode } from '@/ir/schema'

// ── Types ────────────────────────────────────────────

export interface ExecutionContext {
  params:  Record<string, any>
  vars:    Record<string, any>
  request: Record<string, any>
}

export interface NodeResult {
  nodeId:   string
  nodeType: string
  status:   'running' | 'success' | 'error' | 'skipped'
  output?:  any
  error?:   string
  duration: number  // ms
}

export interface ExecutionResult {
  statusCode: number
  body:       any
  headers?:   Record<string, string>
  trace:      NodeResult[]
}

// ── Executor ─────────────────────────────────────────

export function useIRExecutor() {
  const isRunning = ref(false)
  const nodeResults: Ref<Map<string, NodeResult>> = ref(new Map())

  async function execute(
    ir: GatewayIR,
    mockParams: Record<string, any>,
    mockUpstreams?: Record<string, any>,
  ): Promise<ExecutionResult> {
    isRunning.value = true
    nodeResults.value = new Map()
    const trace: NodeResult[] = []

    const ctx: ExecutionContext = {
      params:  mockParams,
      vars:    {},
      request: mockParams,
    }

    const nodeMap = new Map<string, IRNode>(ir.nodes.map(n => [n.id, n]))

    // 找根節點（無入邊）
    const hasIncoming = new Set(ir.edges.map(e => e.target))
    let currentId: string | undefined = ir.nodes.find(n => !hasIncoming.has(n.id))?.id
    if (!currentId) throw fail('找不到根節點')

    const visited = new Set<string>()

    try {
      while (currentId) {
        if (visited.has(currentId)) throw fail(`偵測到循環：${currentId}`)
        visited.add(currentId)

        const node = nodeMap.get(currentId)
        if (!node) throw fail(`找不到節點：${currentId}`)

        const start = performance.now()
        emitStatus(node.id, node.type, 'running')

        try {
          const result = await executeNode(node, ctx, ir, nodeMap, mockUpstreams)

          if (result.done) {
            // response 節點，結束執行
            const nr = emitStatus(node.id, node.type, 'success', result.output, start)
            trace.push(nr)
            isRunning.value = false
            return { statusCode: result.statusCode!, body: result.output, headers: result.headers, trace }
          }

          const nr = emitStatus(node.id, node.type, 'success', result.output, start)
          trace.push(nr)
          currentId = result.nextId
        } catch (e: any) {
          const nr = emitStatus(node.id, node.type, 'error', undefined, start, e.message)
          trace.push(nr)
          throw e
        }
      }

      throw fail('流程未到達 response 節點')
    } finally {
      isRunning.value = false
    }
  }

  // ── 單節點執行 ──────────────────────────────────────

  interface StepResult {
    done:        boolean
    nextId?:     string
    output?:     any
    statusCode?: number
    headers?:    Record<string, string>
  }

  async function executeNode(
    node: IRNode,
    ctx: ExecutionContext,
    ir: GatewayIR,
    nodeMap: Map<string, IRNode>,
    mockUpstreams?: Record<string, any>,
  ): Promise<StepResult> {
    switch (node.type) {
      case 'http-call': {
        const path = interpolate(node.config.path, ctx)
        const upstreamName = node.config.upstream.name
        let data: any

        if (mockUpstreams?.[upstreamName]) {
          // mock 模式：直接用 mock data
          const mock = mockUpstreams[upstreamName]
          data = typeof mock === 'function' ? mock(path, node.config.method) : mock
        } else {
          // 真實呼叫（透過 proxy）
          const baseUrl = node.config.upstream.provider === 'url'
            ? node.config.upstream.url
            : `/proxy/${upstreamName}`
          const res = await fetch(`${baseUrl}${path}`, {
            method:  node.config.method ?? 'GET',
            headers: node.config.headers,
            ...(node.config.body && { body: await evalExpr(node.config.body, ctx.vars) }),
            signal:  AbortSignal.timeout(node.config.timeout ?? 3000),
          })
          data = await res.json()
        }

        ctx.vars[node.outputVar] = data
        return { done: false, nextId: getNext(ir, node.id, null), output: data }
      }

      case 'condition': {
        const result = await evalExpr(node.config.expression, ctx.vars)
        const handle = result ? 'true' : 'false'
        return { done: false, nextId: getNext(ir, node.id, handle), output: result }
      }

      case 'switch': {
        const value = await evalExpr(node.config.expression, ctx.vars)
        const strValue = String(value)
        const caseIndex = node.config.cases.findIndex(c => c === strValue)

        let handle: string
        if (caseIndex >= 0) {
          handle = `case:${caseIndex}`
        } else if (node.config.hasDefault) {
          handle = 'default'
        } else {
          throw new Error(`switch: no matching case for "${strValue}" and no default`)
        }

        return { done: false, nextId: getNext(ir, node.id, handle), output: { value: strValue, matched: handle } }
      }

      case 'transform': {
        const result = await evalExpr(node.config.expression, ctx.vars)
        ctx.vars[node.outputVar] = result
        return { done: false, nextId: getNext(ir, node.id, null), output: result }
      }

      case 'fork': {
        // Phase 1: 簡化版 — 依序執行所有分支（Phase 3 改為 Promise.all）
        const branches = ir.edges.filter(e => e.source === node.id).map(e => e.target)
        for (const branchId of branches) {
          const branchNode = nodeMap.get(branchId)
          if (branchNode && branchNode.type === 'http-call') {
            await executeNode(branchNode, ctx, ir, nodeMap, mockUpstreams)
          }
        }
        // fork 完成後找 join
        const joinId = findDownstreamJoin(ir, node.id, branches)
        return { done: false, nextId: joinId, output: { branches } }
      }

      case 'join': {
        const inputNodes = ir.edges
          .filter(e => e.target === node.id)
          .map(e => nodeMap.get(e.source))
          .filter(Boolean)

        const inputVars = inputNodes
          .map(n => (n && 'outputVar' in n) ? ctx.vars[(n as any).outputVar] : undefined)
          .filter(v => v !== undefined)

        let result: any
        if (node.config.strategy === 'merge') {
          result = Object.assign({}, ...inputVars)
        } else if (node.config.strategy === 'array') {
          result = inputVars
        } else if (node.config.strategy === 'custom' && node.config.expression) {
          result = await evalExpr(node.config.expression, ctx.vars)
        }

        ctx.vars[node.outputVar] = result
        return { done: false, nextId: getNext(ir, node.id, null), output: result }
      }

      case 'sub-flow': {
        // Phase 1: placeholder — sub-flow 需要 resolve 機制，暫時跳過
        ctx.vars[node.outputVar] = { _placeholder: `sub-flow:${node.config.flowId}` }
        return { done: false, nextId: getNext(ir, node.id, null), output: ctx.vars[node.outputVar] }
      }

      case 'response': {
        const body = await evalExpr(node.config.body, ctx.vars)
        return { done: true, output: body, statusCode: node.config.statusCode ?? 200, headers: node.config.headers }
      }

      default:
        throw new Error(`未知節點類型：${(node as any).type}`)
    }
  }

  // ── Helpers ─────────────────────────────────────────

  function emitStatus(nodeId: string, nodeType: string, status: NodeResult['status'], output?: any, startTime?: number, error?: string): NodeResult {
    const nr: NodeResult = {
      nodeId, nodeType, status, output, error,
      duration: startTime ? Math.round(performance.now() - startTime) : 0,
    }
    nodeResults.value.set(nodeId, nr)
    return nr
  }

  return { execute, isRunning, nodeResults }
}

// ── Utility functions ────────────────────────────────

async function evalExpr(expression: string, data: any): Promise<any> {
  return jsonata(expression).evaluate(data)
}

function interpolate(template: string, ctx: ExecutionContext): string {
  return template.replace(/\$\{ctx\.params\.(\w+)\}/g, (_, key) => ctx.params[key] ?? '')
}

function getNext(ir: GatewayIR, nodeId: string, handle: string | null): string | undefined {
  const edge = ir.edges.find(e =>
    e.source === nodeId &&
    (handle === null ? !e.sourceHandle : e.sourceHandle === handle)
  )
  return edge?.target
}

function findDownstreamJoin(ir: GatewayIR, _forkId: string, branches: string[]): string | undefined {
  // 找所有分支的出邊目標中，type 為 join 的節點
  for (const branchId of branches) {
    const outEdges = ir.edges.filter(e => e.source === branchId)
    for (const e of outEdges) {
      const target = ir.nodes.find(n => n.id === e.target)
      if (target?.type === 'join') return target.id
    }
  }
  return undefined
}

function fail(msg: string): Error {
  return new Error(msg)
}
