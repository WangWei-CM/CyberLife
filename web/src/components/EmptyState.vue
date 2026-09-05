<script setup lang="ts">
import AppIcon from './AppIcon.vue'
withDefaults(defineProps<{ icon?: string; text: string; compact?: boolean }>(), { icon: 'inbox', compact: false })
</script>

<template>
  <div class="empty empty-state" :class="{ compact }">
    <AppIcon :name="icon" :size="compact ? 22 : 30" :stroke-width="1.25" />
    <p>{{ text }}</p>
  </div>
</template>
