<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

/**
 * 自研图片放大层（三期 §5.1）：portal 到 body，主题色遮罩、滚轮缩放、拖拽平移、Esc / 点遮罩关闭。
 */
const props = defineProps<{ src: string; alt?: string; themeClass: string }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const scale = ref(1)
const offset = ref({ x: 0, y: 0 })
const dragging = ref(false)
let dragStart: { x: number; y: number; ox: number; oy: number } | null = null

function onWheel(event: WheelEvent) {
  event.preventDefault()
  const factor = Math.exp(-event.deltaY * .0015)
  const next = Math.min(6, Math.max(.4, scale.value * factor))
  const stage = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const cx = event.clientX - stage.left - stage.width / 2
  const cy = event.clientY - stage.top - stage.height / 2
  const ratio = next / scale.value
  offset.value = { x: cx - (cx - offset.value.x) * ratio, y: cy - (cy - offset.value.y) * ratio }
  scale.value = next
}
function onDown(event: PointerEvent) { dragging.value = true; dragStart = { x: event.clientX, y: event.clientY, ox: offset.value.x, oy: offset.value.y }; (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId) }
function onMove(event: PointerEvent) { if (!dragStart) return; offset.value = { x: dragStart.ox + event.clientX - dragStart.x, y: dragStart.oy + event.clientY - dragStart.y } }
function onUp(event: PointerEvent) {
  const moved = dragStart ? Math.hypot(event.clientX - dragStart.x, event.clientY - dragStart.y) : 0
  dragStart = null
  dragging.value = false
  if (moved < 4 && (event.target as HTMLElement).classList.contains('lightbox-stage')) emit('close')
}
function reset() { scale.value = 1; offset.value = { x: 0, y: 0 } }
function onKey(event: KeyboardEvent) { if (event.key === 'Escape') emit('close'); if (event.key === '0') reset() }
watch(() => props.src, reset)
onMounted(() => { document.addEventListener('keydown', onKey); document.body.style.overflow = 'hidden' })
onBeforeUnmount(() => { document.removeEventListener('keydown', onKey); document.body.style.overflow = '' })
</script>

<template>
  <Teleport to="body">
    <div class="lightbox" :class="themeClass" role="dialog" aria-modal="true" aria-label="查看图片">
      <div class="lightbox-stage" :class="{ dragging }" @wheel="onWheel" @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp" @pointercancel="onUp" @dblclick="reset">
        <img :src="src" :alt="alt || ''" draggable="false" :style="{ transform: `translate(${offset.x}px, ${offset.y}px) scale(${scale})` }" />
      </div>
      <button class="lightbox-close icon-button" aria-label="关闭" @click="emit('close')"><AppIcon name="close" :size="22" /></button>
      <span class="lightbox-zoom mono">{{ Math.round(scale * 100) }}%</span>
    </div>
  </Teleport>
</template>
