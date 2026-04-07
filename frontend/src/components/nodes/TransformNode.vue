<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-transform">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Transform</div>
    <div class="node-body">
      <div class="node-detail">{{ (data.expression || '').slice(0, 30) }}</div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.node-transform { --node-color: #8b5cf6; }
</style>
