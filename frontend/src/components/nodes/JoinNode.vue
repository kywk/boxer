<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-join">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Join</div>
    <div class="node-body">
      <div class="node-detail">{{ data.strategy || 'merge' }}</div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.node-join { --node-color: #14b8a6; }
</style>
