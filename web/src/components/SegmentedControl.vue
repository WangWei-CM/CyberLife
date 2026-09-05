<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { positionSegmentThumb } from '../lib/motion'

const props = defineProps<{ modelValue: string; options: { value: string; label: string }[]; ariaLabel?: string }>()
const emit = defineEmits<{ (event: 'update:modelValue', value: string): void }>()
const root = ref<HTMLElement>()
let observer: ResizeObserver | undefined

const reposition = () => nextTick(() => positionSegmentThumb(root.value ?? null))
watch(() => [props.modelValue, props.options], reposition, { deep: true })
onMounted(() => { reposition(); if ('ResizeObserver' in window && root.value) { observer = new ResizeObserver(() => positionSegmentThumb(root.value ?? null)); observer.observe(root.value) } })
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div ref="root" class="segmented" role="tablist" :aria-label="ariaLabel">
    <i class="segmented-thumb" aria-hidden="true" />
    <button v-for="option in options" :key="option.value" type="button" role="tab" :aria-selected="option.value === modelValue" :class="{ active: option.value === modelValue }" @click="emit('update:modelValue', option.value)">{{ option.label }}</button>
  </div>
</template>
