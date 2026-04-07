// frontend/src/composables/useIRImport.ts
import { useVueFlow } from '@vue-flow/core'
import { GatewayIRSchema, type GatewayIR } from '@/ir/schema'

export function useIRImport() {
  const { fromObject, fitView } = useVueFlow()

  function irToVueFlow(ir: GatewayIR) {
    const depths = computeNodeDepths(ir)

    const nodes = ir.nodes.map(node => ({
      id:       node.id,
      type:     node.type,
      position: { x: (depths.get(node.id) ?? 0) * 280 + 40, y: getYOffset(ir, node.id, depths) },
      data:     { ...('config' in node ? node.config : {}), ...('outputVar' in node ? { outputVar: node.outputVar } : {}) },
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

  function loadFromJSON(json: unknown): GatewayIR {
    const ir = GatewayIRSchema.parse(json)
    irToVueFlow(ir)
    return ir
  }

  return { irToVueFlow, loadFromJSON }
}

// BFS 拓樸深度（用於 x 座標）
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
    ir.edges.filter(e => e.source === id).forEach(e =>
      queue.push({ id: e.target, depth: depth + 1 })
    )
  }
  return depths
}

// 同深度的節點垂直分散
function getYOffset(ir: GatewayIR, nodeId: string, depths: Map<string, number>): number {
  const myDepth = depths.get(nodeId) ?? 0
  const siblings = ir.nodes.filter(n => depths.get(n.id) === myDepth)
  const idx = siblings.findIndex(n => n.id === nodeId)
  return idx * 140 + 40
}
