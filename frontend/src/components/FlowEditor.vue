<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { VueFlow, Panel, useVueFlow, type Node } from '@vue-flow/core'
import { MiniMap } from '@vue-flow/minimap'
import { Controls } from '@vue-flow/controls'
import { Background } from '@vue-flow/background'
import { nanoid } from 'nanoid'

import HttpCallNode from './nodes/HttpCallNode.vue'
import ConditionNode from './nodes/ConditionNode.vue'
import SwitchNode from './nodes/SwitchNode.vue'
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
import { useIRExecutor, type ExecutionResult } from '@/composables/useIRExecutor'

const nodeTypes = {
  'http-call':  HttpCallNode,
  'condition':  ConditionNode,
  'switch':     SwitchNode,
  'transform':  TransformNode,
  'fork':       ForkNode,
  'join':       JoinNode,
  'sub-flow':   SubFlowNode,
  'response':   ResponseNode,
} as any

const { screenToFlowCoordinate, updateNodeData, addNodes, getNodes } = useVueFlow()
const { onConnect } = useFlowValidator()
const { vueFlowToIR } = useIRExport()
const { loadFromJSON } = useIRImport()
const { execute, isRunning, nodeResults } = useIRExecutor()

const selectedNode = ref<Node | null>(null)
const executionResult = ref<ExecutionResult | null>(null)
const executionError = ref<string | null>(null)
const showTestPanel = ref(false)
const mockParamsJson = ref('{}')
const mockUpstreamsJson = ref('{}')

// ── Node execution status → CSS class ────────────────

watch(nodeResults, (results) => {
  for (const node of getNodes.value) {
    const nr = results.get(node.id)
    node.class = nr ? `exec-${nr.status}` : ''
  }
}, { deep: true })

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
    navigator.clipboard.writeText(json)
    alert('IR JSON copied to clipboard')
  } catch (e: any) {
    alert('Export failed: ' + e.message)
  }
}

const codegenResult = ref<{ code: string; filename: string; target: string } | null>(null)

async function handleCodegen(target: string) {
  try {
    const ir = vueFlowToIR('flow-' + nanoid(6), 'Untitled Flow', { method: 'GET', path: '/api/example' })
    const res = await fetch('/api/codegen', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ir, target }),
    })
    if (!res.ok) {
      const err = await res.json()
      alert('Codegen failed: ' + (err.error || res.statusText))
      return
    }
    const data = await res.json()
    codegenResult.value = { code: data.code, filename: data.filename, target }
  } catch (e: any) {
    alert('Codegen failed: ' + e.message)
  }
}

function downloadCodegenResult() {
  if (!codegenResult.value) return
  const blob = new Blob([codegenResult.value.code], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = codegenResult.value.filename
  a.click()
  URL.revokeObjectURL(url)
}

const showImportModal = ref(false)
const importJson = ref('')

function handleImport() {
  showImportModal.value = true
  importJson.value = ''
}

function confirmImport() {
  if (!importJson.value.trim()) return
  try {
    loadFromJSON(JSON.parse(importJson.value))
    showImportModal.value = false
  } catch (e: any) {
    alert('Import failed: ' + e.message)
  }
}

// ── Test Execution ───────────────────────────────────

async function handleRunTest() {
  executionResult.value = null
  executionError.value = null

  try {
    const ir = vueFlowToIR('flow-test', 'Test Run', { method: 'GET', path: '/test' })
    const params = JSON.parse(mockParamsJson.value)
    const upstreams = JSON.parse(mockUpstreamsJson.value)
    executionResult.value = await execute(ir, params, upstreams)
  } catch (e: any) {
    executionError.value = e.message
  }
}

const selectedNodeResult = computed(() => {
  if (!selectedNode.value) return null
  return nodeResults.value.get(selectedNode.value.id) ?? null
})
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
            <button @click="showTestPanel = !showTestPanel">
              {{ showTestPanel ? '✕ Close Test' : '▶ Test' }}
            </button>
            <button @click="handleExport">Export IR</button>
            <button @click="handleImport">Import IR</button>
            <button @click="handleCodegen('golang')">⚙ Go</button>
            <button @click="handleCodegen('kong')">⚙ Lua</button>
          </div>
        </Panel>

        <MiniMap />
        <Controls />
        <Background :gap="16" />
      </VueFlow>
    </div>

    <!-- 右側面板：ConfigPanel 或 Test Panel -->
    <div v-if="showTestPanel" class="side-panel">
      <div class="panel-section">
        <div class="panel-title">Test Execution</div>
        <label>Mock Params (JSON)</label>
        <textarea v-model="mockParamsJson" rows="3" class="mono-input" placeholder='{ "userId": "123" }' />
        <label>Mock Upstreams (JSON)</label>
        <textarea v-model="mockUpstreamsJson" rows="5" class="mono-input" placeholder='{ "user-service": { "id": "123", "name": "Alice" } }' />
        <button class="run-btn" :disabled="isRunning" @click="handleRunTest">
          {{ isRunning ? '⏳ Running...' : '▶ Run' }}
        </button>
      </div>

      <!-- Execution Result -->
      <div v-if="executionError" class="panel-section result-error">
        <div class="panel-title">Error</div>
        <pre>{{ executionError }}</pre>
      </div>

      <div v-if="executionResult" class="panel-section">
        <div class="panel-title">Response: {{ executionResult.statusCode }}</div>
        <pre class="result-json">{{ JSON.stringify(executionResult.body, null, 2) }}</pre>

        <div class="panel-title" style="margin-top: 12px">Trace</div>
        <div v-for="step in executionResult.trace" :key="step.nodeId" class="trace-item" :class="'trace-' + step.status">
          <span class="trace-badge">{{ step.status === 'success' ? '✓' : step.status === 'error' ? '✗' : '⏳' }}</span>
          <span class="trace-id">{{ step.nodeId }}</span>
          <span class="trace-type">{{ step.nodeType }}</span>
          <span class="trace-time">{{ step.duration }}ms</span>
        </div>
      </div>

      <!-- Selected node output -->
      <div v-if="selectedNodeResult" class="panel-section">
        <div class="panel-title">Node Output: {{ selectedNodeResult.nodeId }}</div>
        <pre class="result-json">{{ JSON.stringify(selectedNodeResult.output, null, 2) }}</pre>
      </div>
    </div>

    <ConfigPanel
      v-if="!showTestPanel && selectedNode"
      :node="selectedNode"
      :node-result="selectedNodeResult"
      @update="onConfigUpdate"
    />

    <!-- Import IR Modal -->
    <div v-if="showImportModal" class="modal-overlay" @click.self="showImportModal = false">
      <div class="modal">
        <div class="modal-title">Import IR JSON</div>
        <textarea
          v-model="importJson"
          class="import-textarea"
          placeholder="Paste IR JSON here..."
          spellcheck="false"
          autofocus
        />
        <div class="modal-actions">
          <button class="btn-cancel" @click="showImportModal = false">Cancel</button>
          <button class="btn-confirm" @click="confirmImport">Import</button>
        </div>
      </div>
    </div>

    <!-- Codegen Result Modal -->
    <div v-if="codegenResult" class="modal-overlay" @click.self="codegenResult = null">
      <div class="modal">
        <div class="modal-title">{{ codegenResult.filename }}</div>
        <pre class="import-textarea" style="overflow: auto; white-space: pre; cursor: text;">{{ codegenResult.code }}</pre>
        <div class="modal-actions">
          <button class="btn-cancel" @click="codegenResult = null">Close</button>
          <button class="btn-confirm" @click="downloadCodegenResult">Download</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.editor-container { display: flex; height: 100vh; width: 100vw; }
.flow-area { flex: 1; }
.toolbar { display: flex; gap: 6px; }
.toolbar button { padding: 6px 12px; font-size: 13px; border: 1px solid #d1d5db; border-radius: 4px; background: white; cursor: pointer; }
.toolbar button:hover { background: #f3f4f6; }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; border-radius: 12px; padding: 20px; width: 640px; max-width: 90vw; max-height: 80vh; display: flex; flex-direction: column; box-shadow: 0 8px 32px rgba(0,0,0,0.2); }
.modal-title { font-weight: 600; font-size: 16px; margin-bottom: 12px; }
.import-textarea { width: 100%; height: 400px; padding: 12px; border: 1px solid #d1d5db; border-radius: 8px; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; line-height: 1.5; resize: vertical; box-sizing: border-box; }
.import-textarea:focus { outline: none; border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,0.2); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.btn-cancel { padding: 8px 16px; border: 1px solid #d1d5db; border-radius: 6px; background: white; cursor: pointer; font-size: 14px; }
.btn-confirm { padding: 8px 16px; border: none; border-radius: 6px; background: #3b82f6; color: white; cursor: pointer; font-size: 14px; }
.btn-confirm:hover { background: #2563eb; }

.side-panel { width: 320px; border-left: 1px solid #e5e7eb; background: white; overflow-y: auto; padding: 12px; }
.panel-section { margin-bottom: 16px; }
.panel-title { font-weight: 600; font-size: 14px; margin-bottom: 8px; }
.panel-section label { display: block; font-size: 12px; color: #6b7280; margin: 8px 0 4px; }
.mono-input { width: 100%; padding: 6px 8px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 12px; font-family: monospace; resize: vertical; box-sizing: border-box; }
.run-btn { margin-top: 10px; width: 100%; padding: 8px; font-size: 14px; font-weight: 600; border: none; border-radius: 6px; background: #3b82f6; color: white; cursor: pointer; }
.run-btn:hover { background: #2563eb; }
.run-btn:disabled { background: #93c5fd; cursor: not-allowed; }

.result-json { background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 4px; padding: 8px; font-size: 11px; font-family: monospace; overflow-x: auto; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow-y: auto; }
.result-error { color: #dc2626; }
.result-error pre { background: #fef2f2; border-color: #fecaca; padding: 8px; border-radius: 4px; font-size: 12px; white-space: pre-wrap; }

.trace-item { display: flex; align-items: center; gap: 6px; padding: 4px 0; font-size: 12px; border-bottom: 1px solid #f3f4f6; }
.trace-badge { width: 16px; text-align: center; }
.trace-success .trace-badge { color: #22c55e; }
.trace-error .trace-badge { color: #ef4444; }
.trace-id { font-family: monospace; font-weight: 500; }
.trace-type { color: #6b7280; flex: 1; }
.trace-time { color: #9ca3af; font-size: 11px; }
</style>
