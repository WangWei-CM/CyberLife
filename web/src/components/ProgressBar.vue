<script setup lang="ts">
import { computed } from 'vue'
import { useCountUp } from '../lib/motion'

const props = withDefaults(defineProps<{ value: number; tone?: 'accent' | 'accent-2' | 'dim'; label?: boolean; height?: number; title?: string }>(), { tone: 'accent', label: false, height: 4 })
const clamped = computed(() => Math.max(0, Math.min(100, Number.isFinite(props.value) ? props.value : 0)))
const display = useCountUp(() => clamped.value)
</script>

<template>
  <div class="progress-wrap" :class="`tone-${tone}`" :title="title">
    <span class="progress" :style="{ height: `${height}px` }"><i :style="{ width: `${clamped}%` }" /></span>
    <b v-if="label" class="progress-label mono">{{ Math.round(display) }}%</b>
  </div>
</template>
