<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <div class="node node-response">
    <Handle type="target" :position="Position.Left" />
    <div class="node-header">Response</div>
    <div class="node-body">
      <div class="node-detail">{{ data.statusCode || 200 }}</div>
    </div>
  </div>
</template>

<style scoped>
.node-response { --node-color: #22c55e; }
</style>
