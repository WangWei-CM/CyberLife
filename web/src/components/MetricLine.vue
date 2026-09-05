<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { DUR, reducedMotion } from '../lib/motion'

/**
 * 折线趋势图（不标数值）。
 * - 数据变化时在两条曲线之间 lerp 500ms（点数一致）或以描边绘制动画切换（点数变化）
 * - hover 高亮最近的数据点，只显示该点的时间标签
 */
export type MetricPoint = { x: number; y: number | null; label?: string }
const props = withDefaults(defineProps<{ points: MetricPoint[]; min?: number; max?: number; height?: number; area?: boolean; empty?: string }>(), { height: 120, area: true, empty: '' })

const W = 100
const H = 100
const PAD = 8
const hover = ref<number | null>(null)
const drawKey = ref(0)
const rendered = ref<{ x: number; y: number | null }[]>([])
let frame = 0

function normalize(points: MetricPoint[]) {
  const valid = points.filter(point => point.y !== null && Number.isFinite(point.y as number))
  if (!points.length || !valid.length) return []
  const xs = points.map(point => point.x)
  const xMin = Math.min(...xs)
  const xMax = Math.max(...xs)
  const ys = valid.map(point => point.y as number)
  let yMin = props.min ?? Math.min(...ys)
  let yMax = props.max ?? Math.max(...ys)
  if (yMax - yMin < 1e-6) { yMin -= 1; yMax += 1 }
  if (props.min === undefined && props.max === undefined) { const pad = (yMax - yMin) * .15; yMin -= pad; yMax += pad }
  const spanX = xMax - xMin || 1
  return points.map(point => ({
    x: PAD + ((point.x - xMin) / spanX) * (W - PAD * 2),
    y: point.y === null ? null : H - PAD - ((point.y - yMin) / (yMax - yMin)) * (H - PAD * 2),
  }))
}

/** 单调三次插值（Fritsch–Carlson），避免过冲；null 断开为独立段。 */
function pathFor(points: { x: number; y: number | null }[]): string {
  const segments: { x: number; y: number }[][] = []
  let current: { x: number; y: number }[] = []
  for (const point of points) {
    if (point.y === null) { if (current.length) segments.push(current); current = []; continue }
    current.push(point as { x: number; y: number })
  }
  if (current.length) segments.push(current)
  return segments.map(segment => {
    if (segment.length === 1) return `M${segment[0].x.toFixed(2)} ${segment[0].y.toFixed(2)}h0.01`
    const n = segment.length
    const dx: number[] = []
    const dy: number[] = []
    const m: number[] = []
    for (let i = 0; i < n - 1; i++) { dx.push(segment[i + 1].x - segment[i].x || 1e-6); dy.push(segment[i + 1].y - segment[i].y); m.push(dy[i] / dx[i]) }
    const tangents: number[] = [m[0]]
    for (let i = 1; i < n - 1; i++) tangents.push(m[i - 1] * m[i] <= 0 ? 0 : (m[i - 1] + m[i]) / 2)
    tangents.push(m[n - 2])
    for (let i = 0; i < n - 1; i++) {
      if (m[i] === 0) { tangents[i] = 0; tangents[i + 1] = 0; continue }
      const a = tangents[i] / m[i]
      const b = tangents[i + 1] / m[i]
      const s = a * a + b * b
      if (s > 9) { const t = 3 / Math.sqrt(s); tangents[i] = t * a * m[i]; tangents[i + 1] = t * b * m[i] }
    }
    let d = `M${segment[0].x.toFixed(2)} ${segment[0].y.toFixed(2)}`
    for (let i = 0; i < n - 1; i++) {
      const p0 = segment[i]
      const p1 = segment[i + 1]
      const h = dx[i] / 3
      d += `C${(p0.x + h).toFixed(2)} ${(p0.y + tangents[i] * h).toFixed(2)} ${(p1.x - h).toFixed(2)} ${(p1.y - tangents[i + 1] * h).toFixed(2)} ${p1.x.toFixed(2)} ${p1.y.toFixed(2)}`
    }
    return d
  }).join(' ')
}

const target = computed(() => normalize(props.points))
const linePath = computed(() => pathFor(rendered.value))
const areaPath = computed(() => {
  const points = rendered.value.filter(point => point.y !== null) as { x: number; y: number }[]
  if (points.length < 2) return ''
  return `${pathFor(rendered.value)} L${points[points.length - 1].x.toFixed(2)} ${H - 1} L${points[0].x.toFixed(2)} ${H - 1} Z`
})
const hasData = computed(() => target.value.some(point => point.y !== null))
/** 孤立数据点（前后都是空档）：折线画不出来，用小圆点标出。 */
const isolated = computed(() => rendered.value.filter((point, index, list) => point.y !== null && (list[index - 1]?.y ?? null) === null && (list[index + 1]?.y ?? null) === null))
const hoverPoint = computed(() => hover.value === null ? null : rendered.value[hover.value])
const hoverLabel = computed(() => hover.value === null ? '' : props.points[hover.value]?.label ?? '')

function morph(from: { x: number; y: number | null }[], to: { x: number; y: number | null }[]) {
  const started = performance.now()
  const step = (now: number) => {
    const progress = Math.min(1, (now - started) / 500)
    const eased = 1 - Math.pow(1 - progress, 3)
    rendered.value = to.map((point, index) => {
      const previous = from[index]
      if (point.y === null || previous.y === null) return point
      return { x: previous.x + (point.x - previous.x) * eased, y: previous.y + (point.y - previous.y) * eased }
    })
    if (progress < 1) frame = requestAnimationFrame(step)
    else frame = 0
  }
  if (frame) cancelAnimationFrame(frame)
  frame = requestAnimationFrame(step)
}
watch(target, (next, previous) => {
  hover.value = null
  const sameShape = previous && previous.length === next.length && previous.every((point, index) => (point.y === null) === (next[index].y === null))
  if (sameShape && previous?.length && !reducedMotion()) { morph(previous, next); return }
  if (frame) { cancelAnimationFrame(frame); frame = 0 }
  rendered.value = next
  drawKey.value++
}, { immediate: true })

function onMove(event: MouseEvent) {
  if (!rendered.value.length) return
  const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const x = ((event.clientX - bounds.left) / bounds.width) * W
  let best = -1
  let distance = Infinity
  rendered.value.forEach((point, index) => { if (point.y === null) return; const d = Math.abs(point.x - x); if (d < distance) { distance = d; best = index } })
  hover.value = best >= 0 ? best : null
}
onBeforeUnmount(() => { if (frame) cancelAnimationFrame(frame) })
</script>

<template>
  <div class="metric" :style="{ height: `${height}px`, '--draw': `${DUR.slow}ms` }" @mousemove="onMove" @mouseleave="hover = null">
    <svg class="metric-svg" viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label="趋势折线">
      <defs>
        <linearGradient :id="`metric-fill-${drawKey}`" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="currentColor" stop-opacity=".22" /><stop offset="1" stop-color="currentColor" stop-opacity="0" /></linearGradient>
      </defs>
      <line class="metric-base" x1="0" :y1="H - 1" x2="100" :y2="H - 1" />
      <g v-if="hasData" :key="drawKey" class="metric-draw">
        <path v-if="area && areaPath" class="metric-area" :d="areaPath" :fill="`url(#metric-fill-${drawKey})`" />
        <path class="metric-path" :d="linePath" pathLength="1" />
      </g>
    </svg>
    <i v-for="(point, index) in isolated" :key="`iso-${index}`" class="metric-point" :style="{ left: `${point.x}%`, top: `${point.y}%` }" />
    <template v-if="hoverPoint && hoverPoint.y !== null">
      <i class="metric-dot" :style="{ left: `${hoverPoint.x}%`, top: `${hoverPoint.y}%` }" />
      <span v-if="hoverLabel" class="metric-tip" :style="{ left: `${hoverPoint.x}%` }">{{ hoverLabel }}</span>
    </template>
    <p v-if="!hasData && empty" class="metric-empty">{{ empty }}</p>
  </div>
</template>
