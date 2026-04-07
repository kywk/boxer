<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-condition">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Condition</div>
    <div class="node-body">
      <div class="node-detail">{{ (data.expression || '').slice(0, 30) }}</div>
    </div>
    <Handle id="true" type="source" :position="Position.Right" :style="{ top: '35%', background: '#22c55e' }" />
    <Handle id="false" type="source" :position="Position.Right" :style="{ top: '65%', background: '#ef4444' }" />
    <span class="handle-label handle-true">T</span>
    <span class="handle-label handle-false">F</span>
  </div>
</template>

<style scoped>
.node-condition { --node-color: #f59e0b; }
.handle-label { position: absolute; right: -18px; font-size: 10px; font-weight: bold; }
.handle-true  { top: 30%; color: #22c55e; }
.handle-false { top: 60%; color: #ef4444; }
</style>
