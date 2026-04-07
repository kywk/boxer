<script setup lang="ts">
import { NODE_TYPES, type NodeType } from '@/ir/schema'

const nodeLabels: Record<NodeType, string> = {
  'http-call':  'HTTP Call',
  'condition':  'Condition',
  'switch':     'Switch',
  'transform':  'Transform',
  'fork':       'Fork',
  'join':       'Join',
  'sub-flow':   'Sub-Flow',
  'response':   'Response',
}

const nodeColors: Record<NodeType, string> = {
  'http-call':  '#3b82f6',
  'condition':  '#f59e0b',
  'switch':     '#d946ef',
  'transform':  '#8b5cf6',
  'fork':       '#06b6d4',
  'join':       '#14b8a6',
  'sub-flow':   '#ec4899',
  'response':   '#22c55e',
}

function onDragStart(event: DragEvent, type: NodeType) {
  event.dataTransfer!.setData('application/gateway-node-type', type)
  event.dataTransfer!.effectAllowed = 'move'
}
</script>

<template>
  <div class="node-palette">
    <div class="palette-title">Nodes</div>
    <div
      v-for="type in NODE_TYPES"
      :key="type"
      class="palette-item"
      :style="{ borderLeftColor: nodeColors[type] }"
      draggable="true"
      @dragstart="onDragStart($event, type)"
    >
      {{ nodeLabels[type] }}
    </div>
  </div>
</template>

<style scoped>
.node-palette { display: flex; flex-direction: column; gap: 4px; padding: 8px; background: white; border-radius: 8px; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }
.palette-title { font-size: 12px; font-weight: 600; color: #6b7280; margin-bottom: 4px; }
.palette-item { padding: 6px 10px; font-size: 13px; border-left: 3px solid; border-radius: 4px; cursor: grab; background: #f9fafb; }
.palette-item:hover { background: #f3f4f6; }
</style>
