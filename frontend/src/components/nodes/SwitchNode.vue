<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'

const props = defineProps<{ data: Record<string, any> }>()

const cases = computed<string[]>(() => props.data.cases ?? [])
const hasDefault = computed(() => props.data.hasDefault !== false)

const caseColors = ['#3b82f6', '#8b5cf6', '#f59e0b', '#ec4899', '#06b6d4', '#84cc16']
</script>

<template>
  <div class="node node-switch">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Switch</div>
    <div class="node-body">
      <div class="node-detail">{{ (data.expression || '').slice(0, 30) }}</div>
      <div class="case-list">
        <div v-for="(c, i) in cases" :key="i" class="case-item" :style="{ color: caseColors[i % caseColors.length] }">
          {{ c }}
        </div>
        <div v-if="hasDefault" class="case-item case-default">default</div>
      </div>
    </div>
    <Handle
      v-for="(_, i) in cases"
      :key="'case:' + i"
      :id="'case:' + i"
      type="source"
      :position="Position.Right"
      :style="{ top: ((i + 1) / (cases.length + (hasDefault ? 2 : 1))) * 100 + '%', background: caseColors[i % caseColors.length] }"
    />
    <Handle
      v-if="hasDefault"
      id="default"
      type="source"
      :position="Position.Right"
      :style="{ top: ((cases.length + 1) / (cases.length + 2)) * 100 + '%', background: '#6b7280' }"
    />
    <!-- Labels -->
    <span
      v-for="(c, i) in cases"
      :key="'label:' + i"
      class="handle-label"
      :style="{ top: ((i + 1) / (cases.length + (hasDefault ? 2 : 1))) * 100 + '%', color: caseColors[i % caseColors.length] }"
    >{{ c }}</span>
    <span
      v-if="hasDefault"
      class="handle-label"
      :style="{ top: ((cases.length + 1) / (cases.length + 2)) * 100 + '%', color: '#6b7280' }"
    >default</span>
  </div>
</template>

<style scoped>
.node-switch { --node-color: #d946ef; min-height: 80px; }
.case-list { display: flex; flex-direction: column; gap: 2px; margin-top: 4px; }
.case-item { font-size: 10px; font-family: monospace; }
.case-default { color: #6b7280; }
.handle-label { position: absolute; right: -50px; font-size: 9px; font-weight: bold; transform: translateY(-50%); }
</style>
