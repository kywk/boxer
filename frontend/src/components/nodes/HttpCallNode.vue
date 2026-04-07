<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-http-call">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">HTTP Call</div>
    <div class="node-body">
      <div class="node-label">{{ data.upstream?.name || 'upstream' }}</div>
      <div class="node-detail">{{ data.method || 'GET' }} {{ data.path || '/' }}</div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.node-http-call { --node-color: #3b82f6; }
</style>
