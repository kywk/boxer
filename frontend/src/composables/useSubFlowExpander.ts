// frontend/src/composables/useSubFlowExpander.ts
import { GatewayIRSchema, type GatewayIR, type IRNode, type IREdge } from '@/ir/schema'

export interface FlowResolver {
  resolve(flowId: string): GatewayIR | null
}

// LocalStorageResolver resolves sub-flows from API first, then localStorage drafts.
export class LocalStorageResolver implements FlowResolver {
  private cache = new Map<string, GatewayIR>()

  register(ir: GatewayIR) {
    this.cache.set(ir.id, ir)
  }

  resolve(flowId: string): GatewayIR | null {
    if (this.cache.has(flowId)) return this.cache.get(flowId)!
    const raw = localStorage.getItem(`flow-draft:${flowId}`)
    if (!raw) return null
    try {
      const ir = GatewayIRSchema.parse(JSON.parse(raw))
      this.cache.set(flowId, ir)
      return ir
    } catch {
      return null
    }
  }
}

const MAX_DEPTH = 5

export function expandSubFlows(
  ir: GatewayIR,
  resolver: FlowResolver,
  visited: Set<string> = new Set(),
  depth = 0,
): GatewayIR {
  if (depth > MAX_DEPTH) throw new Error(`sub-flow 展開深度超過 ${MAX_DEPTH} 層`)
  if (visited.has(ir.id)) throw new Error(`循環引用偵測：${ir.id}`)
  visited.add(ir.id)

  const newNodes: IRNode[] = []
  const newEdges: IREdge[] = []
  const subFlowNodes = ir.nodes.filter(n => n.type === 'sub-flow')

  if (subFlowNodes.length === 0) return ir

  // Copy non-sub-flow nodes
  for (const node of ir.nodes) {
    if (node.type !== 'sub-flow') {
      newNodes.push(node)
    }
  }

  // Copy edges not involving sub-flow nodes
  const subFlowIds = new Set(subFlowNodes.map(n => n.id))
  for (const edge of ir.edges) {
    if (!subFlowIds.has(edge.source) && !subFlowIds.has(edge.target)) {
      newEdges.push(edge)
    }
  }

  // Expand each sub-flow node
  for (const sfNode of subFlowNodes) {
    if (sfNode.type !== 'sub-flow') continue
    const { flowId, inputMap } = sfNode.config
    const prefix = `${sfNode.id}__`

    let subIR = resolver.resolve(flowId)
    if (!subIR) throw new Error(`無法解析 sub-flow: ${flowId}`)

    // Recursively expand nested sub-flows
    subIR = expandSubFlows(subIR, resolver, new Set(visited), depth + 1)

    // Find sub-flow's root (no incoming edges) and terminal (response node)
    const hasIncoming = new Set(subIR.edges.map(e => e.target))
    const rootId = subIR.nodes.find(n => !hasIncoming.has(n.id))?.id
    const responseNode = subIR.nodes.find(n => n.type === 'response')

    if (!rootId) throw new Error(`sub-flow ${flowId}: 找不到根節點`)
    if (!responseNode) throw new Error(`sub-flow ${flowId}: 找不到 response 節點`)

    // Add prefixed nodes (skip response node, replace with transform that writes outputVar)
    for (const node of subIR.nodes) {
      if (node.id === responseNode.id) {
        // Replace response with a transform that writes to the sub-flow's outputVar
        newNodes.push({
          id: prefix + node.id,
          type: 'transform',
          config: {
            engine: 'jsonata' as const,
            expression: responseNode.type === 'response' ? responseNode.config.body : '""',
          },
          outputVar: sfNode.outputVar,
        })
        continue
      }

      const prefixed = { ...node, id: prefix + node.id } as IRNode
      // Prefix outputVar
      if ('outputVar' in prefixed && prefixed.outputVar) {
        (prefixed as any).outputVar = prefix + prefixed.outputVar
      }
      // Replace inputMap references in http-call paths
      if (node.type === 'http-call') {
        let path = node.config.path
        for (const [param, value] of Object.entries(inputMap)) {
          path = path.replace(`\${ctx.params.${param}}`, `\${${value}}`)
        }
        prefixed.config = { ...node.config, path }
      }
      newNodes.push(prefixed)
    }

    // Add prefixed edges (remap response node edges)
    for (const edge of subIR.edges) {
      if (edge.target === responseNode.id) {
        newEdges.push({ source: prefix + edge.source, target: prefix + responseNode.id, sourceHandle: edge.sourceHandle })
      } else {
        newEdges.push({ source: prefix + edge.source, target: prefix + edge.target, sourceHandle: edge.sourceHandle })
      }
    }

    // Rewire: incoming edges to sub-flow node → sub-flow root
    for (const edge of ir.edges) {
      if (edge.target === sfNode.id) {
        newEdges.push({ source: edge.source, target: prefix + rootId, sourceHandle: edge.sourceHandle })
      }
    }

    // Rewire: sub-flow terminal → outgoing edges from sub-flow node
    for (const edge of ir.edges) {
      if (edge.source === sfNode.id) {
        newEdges.push({ source: prefix + responseNode.id, target: edge.target, sourceHandle: edge.sourceHandle })
      }
    }
  }

  // Validate expanded IR
  const expanded: GatewayIR = {
    ...ir,
    nodes: newNodes,
    edges: newEdges,
  }

  return GatewayIRSchema.parse(expanded)
}
