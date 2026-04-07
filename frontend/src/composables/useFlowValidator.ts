// frontend/src/composables/useFlowValidator.ts
import { useVueFlow, type Connection } from '@vue-flow/core'

const ALLOWED_CONNECTIONS: Record<string, string[]> = {
  'http-call':  ['condition', 'switch', 'transform', 'join', 'response', 'fork'],
  'condition':  ['http-call', 'transform', 'fork', 'join', 'response', 'sub-flow'],
  'switch':     ['http-call', 'transform', 'fork', 'join', 'response', 'sub-flow'],
  'transform':  ['condition', 'switch', 'join', 'response', 'fork'],
  'fork':       ['http-call', 'transform', 'condition', 'switch', 'sub-flow'],
  'join':       ['transform', 'condition', 'switch', 'response', 'fork'],
  'sub-flow':   ['condition', 'switch', 'transform', 'join', 'response', 'fork'],
  'response':   [],
}

export function useFlowValidator() {
  const { findNode, addEdges } = useVueFlow()

  function isValidConnection(connection: Connection): boolean {
    const sourceType = findNode(connection.source)?.type
    const targetType = findNode(connection.target)?.type
    if (!sourceType || !targetType) return false
    return ALLOWED_CONNECTIONS[sourceType]?.includes(targetType) ?? false
  }

  function onConnect(connection: Connection) {
    if (!isValidConnection(connection)) return
    addEdges([{
      ...connection,
      ...(connection.sourceHandle === 'true'  && { style: { stroke: '#22c55e' }, animated: true }),
      ...(connection.sourceHandle === 'false' && { style: { stroke: '#ef4444' }, animated: true }),
    }])
  }

  return { onConnect, isValidConnection }
}
