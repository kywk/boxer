<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-sub-flow">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Sub-Flow</div>
    <div class="node-body">
      <div class="node-detail">{{ data.flowId || '(none)' }}</div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.node-sub-flow { --node-color: #ec4899; }
</style>
