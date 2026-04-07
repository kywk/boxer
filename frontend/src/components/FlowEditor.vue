<script setup lang="ts">
import { ref } from 'vue'
import { VueFlow, Panel, useVueFlow, type Node } from '@vue-flow/core'
import { MiniMap } from '@vue-flow/minimap'
import { Controls } from '@vue-flow/controls'
import { Background } from '@vue-flow/background'
import { nanoid } from 'nanoid'

import HttpCallNode from './nodes/HttpCallNode.vue'
import ConditionNode from './nodes/ConditionNode.vue'
import TransformNode from './nodes/TransformNode.vue'
import ForkNode from './nodes/ForkNode.vue'
import JoinNode from './nodes/JoinNode.vue'
import SubFlowNode from './nodes/SubFlowNode.vue'
import ResponseNode from './nodes/ResponseNode.vue'
import NodePalette from './NodePalette.vue'
import ConfigPanel from './ConfigPanel.vue'

import { useFlowValidator } from '@/composables/useFlowValidator'
import { useIRExport } from '@/composables/useIRExport'
import { useIRImport } from '@/composables/useIRImport'

const nodeTypes = {
  'http-call':  HttpCallNode,
  'condition':  ConditionNode,
  'transform':  TransformNode,
  'fork':       ForkNode,
  'join':       JoinNode,
  'sub-flow':   SubFlowNode,
  'response':   ResponseNode,
} as any

const { screenToFlowCoordinate, updateNodeData, addNodes } = useVueFlow()
const { onConnect } = useFlowValidator()
const { vueFlowToIR } = useIRExport()
const { loadFromJSON } = useIRImport()

const selectedNode = ref<Node | null>(null)

// ── Drag & Drop ──────────────────────────────────────

function onDragOver(event: DragEvent) {
  event.preventDefault()
  event.dataTransfer!.dropEffect = 'move'
}

function onDrop(event: DragEvent) {
  const type = event.dataTransfer?.getData('application/gateway-node-type')
  if (!type) return

  const position = screenToFlowCoordinate({ x: event.clientX, y: event.clientY })
  addNodes([{ id: nanoid(8), type, position, data: {} }])
}

// ── Node selection ───────────────────────────────────

function onNodeClick({ node }: { node: Node }) {
  selectedNode.value = node
}

function onPaneClick() {
  selectedNode.value = null
}

function onConfigUpdate(data: Record<string, any>) {
  if (!selectedNode.value) return
  updateNodeData(selectedNode.value.id, data)
}

// ── Export / Import ──────────────────────────────────

function handleExport() {
  try {
    const ir = vueFlowToIR('flow-' + nanoid(6), 'Untitled Flow', { method: 'GET', path: '/api/example' })
    const json = JSON.stringify(ir, null, 2)
    console.log('Exported IR:', json)
    // 複製到剪貼簿
    navigator.clipboard.writeText(json)
    alert('IR JSON copied to clipboard')
  } catch (e: any) {
    alert('Export failed: ' + e.message)
  }
}

function handleImport() {
  const input = prompt('Paste IR JSON:')
  if (!input) return
  try {
    loadFromJSON(JSON.parse(input))
  } catch (e: any) {
    alert('Import failed: ' + e.message)
  }
}
</script>

<template>
  <div class="editor-container">
    <div class="flow-area">
      <VueFlow
        :node-types="nodeTypes"
        @connect="onConnect"
        @node-click="onNodeClick"
        @pane-click="onPaneClick"
        @dragover="onDragOver"
        @drop="onDrop"
      >
        <Panel position="top-left">
          <NodePalette />
        </Panel>

        <Panel position="top-right">
          <div class="toolbar">
            <button @click="handleExport">Export IR</button>
            <button @click="handleImport">Import IR</button>
          </div>
        </Panel>

        <MiniMap />
        <Controls />
        <Background :gap="16" />
      </VueFlow>
    </div>

    <ConfigPanel
      v-if="selectedNode"
      :node="selectedNode"
      @update="onConfigUpdate"
    />
  </div>
</template>

<style scoped>
.editor-container { display: flex; height: 100vh; width: 100vw; }
.flow-area { flex: 1; }
.toolbar { display: flex; gap: 6px; }
.toolbar button { padding: 6px 12px; font-size: 13px; border: 1px solid #d1d5db; border-radius: 4px; background: white; cursor: pointer; }
.toolbar button:hover { background: #f3f4f6; }
</style>
