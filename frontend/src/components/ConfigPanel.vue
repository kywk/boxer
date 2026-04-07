<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Node } from '@vue-flow/core'
import type { NodeResult } from '@/composables/useIRExecutor'

const props = defineProps<{
  node: Node
  nodeResult?: NodeResult | null
}>()
const emit = defineEmits<{ update: [data: Record<string, any>] }>()

// ── Field definitions per node type ──────────────────

interface Field {
  key: string
  label: string
  type: 'text' | 'number' | 'select' | 'textarea' | 'checkbox' | 'group'
  options?: string[]
  hint?: string
  children?: Field[]
  condition?: (data: Record<string, any>) => boolean
}

const fields = computed<Field[]>(() => {
  switch (props.node.type) {
    case 'http-call':
      return [
        { key: 'upstream.name', label: 'Upstream Name', type: 'text', hint: '上游服務名稱' },
        { key: 'upstream.provider', label: 'Provider', type: 'select', options: ['kong', 'k8s-service', 'url'] },
        { key: 'upstream.url', label: 'URL', type: 'text', hint: 'provider=url 時填寫', condition: (d) => d.upstream?.provider === 'url' },
        { key: 'path', label: 'Path', type: 'text', hint: '支援 ${ctx.params.xxx} 插值' },
        { key: 'method', label: 'Method', type: 'select', options: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'] },
        { key: 'timeout', label: 'Timeout (ms)', type: 'number' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
        { key: 'body', label: 'Request Body (JSONata)', type: 'textarea', hint: 'POST/PUT 時的 body 表達式' },
        { key: '_retryEnabled', label: 'Enable Retry', type: 'checkbox' },
        { key: 'retry.maxAttempts', label: 'Max Attempts', type: 'number', condition: (d) => !!d._retryEnabled },
        { key: 'retry.backoff', label: 'Backoff', type: 'select', options: ['fixed', 'exponential'], condition: (d) => !!d._retryEnabled },
        { key: 'retry.delay', label: 'Delay (ms)', type: 'number', condition: (d) => !!d._retryEnabled },
        { key: 'fallback.strategy', label: 'Fallback', type: 'select', options: ['error', 'default-value', 'skip'] },
      ]
    case 'condition':
      return [
        { key: 'expression', label: 'Expression (JSONata)', type: 'textarea', hint: '布林表達式，true/false 分支' },
      ]
    case 'switch':
      return [
        { key: 'expression', label: 'Expression (JSONata)', type: 'textarea', hint: '求值結果匹配 cases' },
        { key: '_casesStr', label: 'Cases (逗號分隔)', type: 'text', hint: '例如: physical,digital,subscription' },
        { key: 'hasDefault', label: 'Has Default Branch', type: 'checkbox' },
      ]
    case 'transform':
      return [
        { key: 'engine', label: 'Engine', type: 'select', options: ['jsonata', 'jmespath'] },
        { key: 'expression', label: 'Expression', type: 'textarea' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
      ]
    case 'fork':
      return [
        { key: 'strategy', label: 'Strategy', type: 'select', options: ['all', 'race', 'allSettled'] },
        { key: 'timeout', label: 'Timeout (ms)', type: 'number', hint: '整體超時，0=無限' },
      ]
    case 'join':
      return [
        { key: 'strategy', label: 'Strategy', type: 'select', options: ['merge', 'array', 'custom'] },
        { key: 'expression', label: 'Custom Expression', type: 'textarea', condition: (d) => d.strategy === 'custom' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
      ]
    case 'sub-flow':
      return [
        { key: 'flowId', label: 'Flow ID', type: 'text' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
      ]
    case 'response':
      return [
        { key: 'statusCode', label: 'Status Code', type: 'number' },
        { key: 'body', label: 'Body (JSONata)', type: 'textarea' },
      ]
    default:
      return []
  }
})

const visibleFields = computed(() =>
  fields.value.filter(f => !f.condition || f.condition(props.node.data ?? {}))
)

// ── Value get/set ────────────────────────────────────

function getValue(key: string): any {
  if (key === '_casesStr') {
    const cases = props.node.data?.cases
    return Array.isArray(cases) ? cases.join(', ') : ''
  }
  if (key === '_retryEnabled') return !!props.node.data?.retry
  const parts = key.split('.')
  let val: any = props.node.data
  for (const p of parts) val = val?.[p]
  return val ?? ''
}

function onFieldChange(key: string, value: any) {
  const data = { ...props.node.data }

  if (key === '_casesStr') {
    data.cases = String(value).split(',').map((s: string) => s.trim()).filter(Boolean)
    emit('update', data)
    return
  }
  if (key === '_retryEnabled') {
    if (value) {
      data.retry = data.retry || { maxAttempts: 3, backoff: 'fixed', delay: 1000 }
    } else {
      delete data.retry
    }
    data._retryEnabled = value
    emit('update', data)
    return
  }

  const parts = key.split('.')
  if (parts.length === 2) {
    data[parts[0]] = { ...(data[parts[0]] || {}), [parts[1]]: value }
  } else {
    data[key] = value
  }
  emit('update', data)
}

// ── Output preview ───────────────────────────────────

const showOutput = ref(false)
</script>

<template>
  <div class="config-panel">
    <div class="panel-header">
      <span class="panel-title">{{ node.type }}</span>
      <span class="node-id">{{ node.id }}</span>
    </div>

    <!-- Fields -->
    <div v-for="field in visibleFields" :key="field.key" class="field">
      <label>
        {{ field.label }}
        <span v-if="field.hint" class="hint" :title="field.hint">?</span>
      </label>

      <select v-if="field.type === 'select'" :value="getValue(field.key)" @change="onFieldChange(field.key, ($event.target as HTMLSelectElement).value)">
        <option v-for="opt in field.options" :key="opt" :value="opt">{{ opt }}</option>
      </select>

      <label v-else-if="field.type === 'checkbox'" class="checkbox-label">
        <input type="checkbox" :checked="!!getValue(field.key)" @change="onFieldChange(field.key, ($event.target as HTMLInputElement).checked)" />
        {{ field.label }}
      </label>

      <textarea v-else-if="field.type === 'textarea'" :value="getValue(field.key)" @input="onFieldChange(field.key, ($event.target as HTMLTextAreaElement).value)" rows="3" spellcheck="false" />

      <input v-else :type="field.type" :value="getValue(field.key)" @input="onFieldChange(field.key, field.type === 'number' ? Number(($event.target as HTMLInputElement).value) : ($event.target as HTMLInputElement).value)" />
    </div>

    <!-- Execution Result -->
    <div v-if="nodeResult" class="result-section">
      <div class="result-header" @click="showOutput = !showOutput">
        <span :class="'badge-' + nodeResult.status">
          {{ nodeResult.status === 'success' ? '✓' : nodeResult.status === 'error' ? '✗' : '⏳' }}
        </span>
        <span>{{ nodeResult.duration }}ms</span>
        <span class="toggle">{{ showOutput ? '▼' : '▶' }}</span>
      </div>
      <div v-if="showOutput" class="result-body">
        <pre v-if="nodeResult.error" class="result-error">{{ nodeResult.error }}</pre>
        <pre v-else class="result-json">{{ JSON.stringify(nodeResult.output, null, 2) }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.config-panel { width: 300px; padding: 12px; background: white; border-left: 1px solid #e5e7eb; overflow-y: auto; height: 100%; }
.panel-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.panel-title { font-weight: 600; font-size: 14px; text-transform: capitalize; }
.node-id { font-size: 11px; color: #9ca3af; font-family: monospace; }
.field { margin-bottom: 10px; }
.field > label { display: block; font-size: 12px; color: #6b7280; margin-bottom: 4px; }
.hint { display: inline-block; width: 14px; height: 14px; line-height: 14px; text-align: center; font-size: 10px; background: #e5e7eb; border-radius: 50%; cursor: help; margin-left: 4px; }
.field input[type="text"], .field input[type="number"], .field select, .field textarea { width: 100%; padding: 6px 8px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 13px; box-sizing: border-box; }
.field textarea { resize: vertical; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; line-height: 1.5; }
.checkbox-label { display: flex !important; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; }
.checkbox-label input { width: auto; }

.result-section { margin-top: 16px; border-top: 1px solid #e5e7eb; padding-top: 12px; }
.result-header { display: flex; align-items: center; gap: 8px; font-size: 13px; cursor: pointer; user-select: none; }
.badge-success { color: #22c55e; }
.badge-error { color: #ef4444; }
.badge-running { color: #f59e0b; }
.toggle { margin-left: auto; color: #9ca3af; font-size: 10px; }
.result-body { margin-top: 8px; }
.result-json { background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 4px; padding: 8px; font-size: 11px; font-family: monospace; overflow-x: auto; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow-y: auto; }
.result-error { background: #fef2f2; border: 1px solid #fecaca; border-radius: 4px; padding: 8px; font-size: 11px; color: #dc2626; white-space: pre-wrap; }
</style>
