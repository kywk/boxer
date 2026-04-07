<script setup lang="ts">
import { computed } from 'vue'
import type { Node } from '@vue-flow/core'

const props = defineProps<{ node: Node }>()
const emit = defineEmits<{ update: [data: Record<string, any>] }>()

const fields = computed(() => {
  switch (props.node.type) {
    case 'http-call':
      return [
        { key: 'upstream.name', label: 'Upstream', type: 'text' },
        { key: 'path', label: 'Path', type: 'text' },
        { key: 'method', label: 'Method', type: 'select', options: ['GET','POST','PUT','DELETE','PATCH'] },
        { key: 'timeout', label: 'Timeout (ms)', type: 'number' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
      ]
    case 'condition':
      return [{ key: 'expression', label: 'Expression (JSONata)', type: 'textarea' }]
    case 'transform':
      return [
        { key: 'expression', label: 'Expression', type: 'textarea' },
        { key: 'outputVar', label: 'Output Variable', type: 'text' },
      ]
    case 'fork':
      return [{ key: 'strategy', label: 'Strategy', type: 'select', options: ['all','race','allSettled'] }]
    case 'join':
      return [
        { key: 'strategy', label: 'Strategy', type: 'select', options: ['merge','array','custom'] },
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

function getValue(key: string): any {
  const parts = key.split('.')
  let val: any = props.node.data
  for (const p of parts) val = val?.[p]
  return val ?? ''
}

function onFieldChange(key: string, value: any) {
  const data = { ...props.node.data }
  const parts = key.split('.')
  if (parts.length === 2) {
    data[parts[0]] = { ...data[parts[0]], [parts[1]]: value }
  } else {
    data[key] = value
  }
  emit('update', data)
}
</script>

<template>
  <div class="config-panel">
    <div class="panel-title">{{ node.type }}</div>
    <div v-for="field in fields" :key="field.key" class="field">
      <label>{{ field.label }}</label>
      <select v-if="field.type === 'select'" :value="getValue(field.key)" @change="onFieldChange(field.key, ($event.target as HTMLSelectElement).value)">
        <option v-for="opt in field.options" :key="opt" :value="opt">{{ opt }}</option>
      </select>
      <textarea v-else-if="field.type === 'textarea'" :value="getValue(field.key)" @change="onFieldChange(field.key, ($event.target as HTMLTextAreaElement).value)" rows="3" />
      <input v-else :type="field.type" :value="getValue(field.key)" @change="onFieldChange(field.key, field.type === 'number' ? Number(($event.target as HTMLInputElement).value) : ($event.target as HTMLInputElement).value)" />
    </div>
  </div>
</template>

<style scoped>
.config-panel { width: 280px; padding: 12px; background: white; border-left: 1px solid #e5e7eb; overflow-y: auto; height: 100%; }
.panel-title { font-weight: 600; font-size: 14px; margin-bottom: 12px; text-transform: capitalize; }
.field { margin-bottom: 10px; }
.field label { display: block; font-size: 12px; color: #6b7280; margin-bottom: 4px; }
.field input, .field select, .field textarea { width: 100%; padding: 6px 8px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 13px; box-sizing: border-box; }
.field textarea { resize: vertical; font-family: monospace; }
</style>
