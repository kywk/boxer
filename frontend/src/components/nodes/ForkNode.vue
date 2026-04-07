<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-fork">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Fork</div>
    <div class="node-body">
      <div class="node-detail">{{ data.strategy || 'all' }}</div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.node-fork { --node-color: #06b6d4; }
</style>
