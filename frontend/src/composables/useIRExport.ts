// frontend/src/composables/useIRExport.ts
import { useVueFlow } from '@vue-flow/core'
import { GatewayIRSchema, type GatewayIR } from '@/ir/schema'

export function useIRExport() {
  const { toObject } = useVueFlow()

  function vueFlowToIR(flowId: string, name: string, trigger: GatewayIR['trigger']): GatewayIR {
    const snapshot = toObject()

    const irNodes = snapshot.nodes.map(node => {
      const base = { id: node.id, type: node.type }
      const d = node.data ?? {}

      switch (node.type) {
        case 'http-call':
          return {
            ...base, outputVar: d.outputVar,
            config: {
              upstream: { name: d.upstream?.name ?? d.upstream, provider: d.upstream?.provider ?? 'kong', ...(d.upstream?.url && { url: d.upstream.url }) },
              path: d.path, method: d.method ?? 'GET', timeout: d.timeout ?? 3000,
              ...(d.headers && { headers: d.headers }),
              ...(d.body && { body: d.body }),
              ...(d.retry && { retry: d.retry }),
              ...(d.fallback && { fallback: d.fallback }),
            },
          }
        case 'condition':
          return { ...base, config: { expression: d.expression } }
        case 'transform':
          return { ...base, outputVar: d.outputVar, config: { engine: d.engine ?? 'jsonata', expression: d.expression } }
        case 'fork':
          return { ...base, config: { strategy: d.strategy ?? 'all', ...(d.timeout && { timeout: d.timeout }) } }
        case 'join':
          return { ...base, outputVar: d.outputVar, config: { strategy: d.strategy ?? 'merge', ...(d.expression && { expression: d.expression }) } }
        case 'sub-flow':
          return { ...base, outputVar: d.outputVar, config: { flowId: d.flowId, inputMap: d.inputMap ?? {} } }
        case 'response':
          return { ...base, config: { statusCode: d.statusCode ?? 200, body: d.body, ...(d.headers && { headers: d.headers }) } }
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
