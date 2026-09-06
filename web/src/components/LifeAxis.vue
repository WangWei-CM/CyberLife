<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createApproach, reducedMotion } from '../lib/motion'
import { dateFromIndex, dayIndex, daysInMonth, iso } from '../lib/dates'
import AppIcon from './AppIcon.vue'

/**
 * 可缩放人生时间轴（三期 §3）
 * - 日期→像素线性映射；缩放 = 改变每像素对应天数，displayDays 为连续浮点值，由 rAF 逼近 targetDays
 * - 刻度密度按 pxPerDay 连续决定：天级 → 过渡带（每 N 天一个标签） → 月级 → 年级，增删带 150ms 淡入淡出
 * - 滚轮围绕光标缩放，左键拖动平移，点击精确选中某一天（任何刻度级别都精确到日）
 * - 竖置：今天在底部，注册日在顶部
 */
type Tick = { key: string; level: 'day' | 'month' | 'year'; top: number; height: number; label: string; strong: boolean; index: number }
/** lifeStart：注册日（接口暂未提供时为空，轴下限回退为今天之前十年）。 */
const props = defineProps<{ lifeStart?: string; today: string; selected: string; milestones: string[]; mirror?: boolean }>()
const FALLBACK_SPAN_DAYS = 3650
const emit = defineEmits<{ (event: 'update:selected', value: string): void; (event: 'range', from: string, to: string): void; (event: 'toggle-dock'): void }>()

const root = ref<HTMLElement>()
const height = ref(600)
const displayDays = ref(14)
const bottom = ref(dayIndex(props.today) + 1)
const zooming = ref(false)
const dragging = ref(false)
let anchorDay = dayIndex(props.today) + .5
let anchorFraction = .5
let dragStart: { y: number; bottom: number; moved: boolean } | null = null
let rangeTimer: number | undefined
let observer: ResizeObserver | undefined

const todayIndex = computed(() => dayIndex(props.today))
const startIndex = computed(() => props.lifeStart ? Math.min(dayIndex(props.lifeStart), todayIndex.value - 6) : todayIndex.value - FALLBACK_SPAN_DAYS)
const totalDays = computed(() => todayIndex.value + 1 - startIndex.value)
const pxPerDay = computed(() => height.value / displayDays.value)
const topDay = computed(() => bottom.value - displayDays.value)
const level = computed<'day' | 'month' | 'year'>(() => pxPerDay.value >= 6 ? 'day' : pxPerDay.value >= .8 ? 'month' : 'year')
const labelEvery = computed(() => Math.max(1, Math.ceil(18 / pxPerDay.value)))
const milestoneSet = computed(() => new Set(props.milestones))

const approach = createApproach({
  factor: .12,
  epsilon: .005,
  onFrame(value) {
    displayDays.value = value
    const nextTop = anchorDay - anchorFraction * value
    bottom.value = clampBottom(nextTop + value)
  },
  onSettle() { zooming.value = false; scheduleRange() },
})
function clampBottom(value: number) {
  const max = todayIndex.value + 1
  const min = Math.min(max, startIndex.value + displayDays.value)
  return Math.max(min, Math.min(max, value))
}
/** 某个日序号（浮点）在轴上的 y 坐标：今天在底部。 */
function y(day: number) { return height.value - (day - topDay.value) * pxPerDay.value }

const ticks = computed<Tick[]>(() => {
  const result: Tick[] = []
  const first = Math.max(startIndex.value, Math.floor(topDay.value) - 1)
  const last = Math.min(todayIndex.value, Math.ceil(bottom.value))
  if (first > last) return result
  if (level.value === 'day') {
    const every = labelEvery.value
    for (let index = first; index <= last; index++) {
      const date = dateFromIndex(index)
      const monthStart = date.getDate() === 1
      const monthLength = daysInMonth(date.getFullYear(), date.getMonth())
      // 过渡带：每 N 天一个标签；月初必标，并让月初附近的常规标签让位，避免重叠
      const nearMonthEdge = every > 1 && (date.getDate() - 1 < every || monthLength - date.getDate() < every)
      const regular = every === 1 || ((index - startIndex.value) % every === 0 && !nearMonthEdge)
      const show = monthStart || index === startIndex.value || regular
      const label = show ? (monthStart || index === startIndex.value ? `${date.getMonth() + 1}月${date.getDate()}日` : `${date.getDate()}日`) : ''
      result.push({ key: `d${index}`, level: 'day', top: y(index + 1), height: pxPerDay.value, label, strong: monthStart, index })
    }
    return result
  }
  const firstDate = dateFromIndex(first)
  if (level.value === 'month') {
    let cursor = new Date(firstDate.getFullYear(), firstDate.getMonth(), 1)
    while (dayIndex(cursor) <= last) {
      const start = Math.max(dayIndex(cursor), startIndex.value)
      const end = Math.min(dayIndex(cursor) + daysInMonth(cursor.getFullYear(), cursor.getMonth()), todayIndex.value + 1)
      const label = cursor.getMonth() === 0 || dayIndex(cursor) <= startIndex.value ? `${cursor.getFullYear()}年${cursor.getMonth() + 1}月` : `${cursor.getMonth() + 1}月`
      result.push({ key: `m${cursor.getFullYear()}-${cursor.getMonth()}`, level: 'month', top: y(end), height: (end - start) * pxPerDay.value, label, strong: cursor.getMonth() === 0, index: start })
      cursor = new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1)
    }
    return result
  }
  let year = firstDate.getFullYear()
  while (dayIndex(new Date(year, 0, 1)) <= last) {
    const start = Math.max(dayIndex(new Date(year, 0, 1)), startIndex.value)
    const end = Math.min(dayIndex(new Date(year + 1, 0, 1)), todayIndex.value + 1)
    result.push({ key: `y${year}`, level: 'year', top: y(end), height: (end - start) * pxPerDay.value, label: `${year}`, strong: true, index: start })
    year++
  }
  return result
})
const medals = computed(() => props.milestones
  .map(date => dayIndex(date))
  .filter(index => index >= topDay.value - 1 && index <= bottom.value + 1)
  .map(index => ({ key: `medal${index}`, date: iso(dateFromIndex(index)), y: y(index + .5) })))
const selectedIndex = computed(() => dayIndex(props.selected))
const selectedStyle = computed(() => ({ transform: `translateY(${y(selectedIndex.value + 1)}px)`, height: `${Math.max(2, pxPerDay.value)}px` }))
const todayStyle = computed(() => ({ transform: `translateY(${y(todayIndex.value + .5)}px)` }))
const rangeLabel = computed(() => {
  const from = dateFromIndex(Math.max(startIndex.value, Math.floor(topDay.value)))
  const to = dateFromIndex(Math.min(todayIndex.value, Math.ceil(bottom.value) - 1))
  return { from: `${from.getMonth() + 1}月${from.getDate()}日`, to: `${to.getMonth() + 1}月${to.getDate()}日`, days: Math.round(displayDays.value) }
})

function scheduleRange() {
  if (rangeTimer) clearTimeout(rangeTimer)
  rangeTimer = window.setTimeout(() => {
    const from = Math.max(startIndex.value, Math.floor(topDay.value))
    const to = Math.min(todayIndex.value, Math.ceil(bottom.value) - 1)
    if (from <= to) emit('range', iso(dateFromIndex(from)), iso(dateFromIndex(to)))
  }, 120)
}
function dayAt(clientY: number) {
  const bounds = root.value!.getBoundingClientRect()
  const localY = clientY - bounds.top
  return topDay.value + (height.value - localY) / pxPerDay.value
}
function onWheel(event: WheelEvent) {
  event.preventDefault()
  const bounds = root.value!.getBoundingClientRect()
  anchorDay = dayAt(event.clientY)
  anchorFraction = (height.value - (event.clientY - bounds.top)) / height.value
  const factor = Math.exp(Math.max(-1, Math.min(1, event.deltaY / 60)) * Math.log(1.06))
  const next = Math.max(3, Math.min(totalDays.value, approach.target * factor))
  zooming.value = true
  approach.set(next)
}
function onDown(event: PointerEvent) {
  if (event.button !== 0) return
  dragStart = { y: event.clientY, bottom: bottom.value, moved: false }
  root.value?.setPointerCapture(event.pointerId)
}
function onMove(event: PointerEvent) {
  if (!dragStart) return
  const dy = event.clientY - dragStart.y
  if (Math.abs(dy) > 4) { dragStart.moved = true; dragging.value = true }
  if (!dragStart.moved) return
  bottom.value = clampBottom(dragStart.bottom - dy / pxPerDay.value)
  anchorDay = bottom.value - anchorFraction * displayDays.value
}
function onUp(event: PointerEvent) {
  if (!dragStart) return
  const moved = dragStart.moved
  dragStart = null
  dragging.value = false
  if (moved) { scheduleRange(); return }
  const index = Math.floor(dayAt(event.clientY))
  const clamped = Math.max(startIndex.value, Math.min(todayIndex.value, index))
  emit('update:selected', iso(dateFromIndex(clamped)))
}
function onKey(event: KeyboardEvent) {
  const delta = event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0
  if (!delta) return
  event.preventDefault()
  const next = Math.max(startIndex.value, Math.min(todayIndex.value, selectedIndex.value + delta))
  emit('update:selected', iso(dateFromIndex(next)))
}
function ensureVisible(index: number) {
  if (index + 1 > bottom.value) bottom.value = clampBottom(index + 1)
  else if (index < topDay.value) bottom.value = clampBottom(index + displayDays.value)
  anchorDay = bottom.value - anchorFraction * displayDays.value
}
watch(() => props.selected, value => { ensureVisible(dayIndex(value)); scheduleRange() })
onMounted(() => {
  const measure = () => { height.value = Math.max(240, root.value?.clientHeight ?? 600); scheduleRange() }
  measure()
  if ('ResizeObserver' in window && root.value) { observer = new ResizeObserver(measure); observer.observe(root.value) }
  approach.set(Math.min(14, totalDays.value), true)
  if (reducedMotion()) zooming.value = false
})
onBeforeUnmount(() => { approach.stop(); observer?.disconnect(); if (rangeTimer) clearTimeout(rangeTimer) })
defineExpose({ rangeLabel })
</script>

<template>
  <aside ref="root" class="life-axis" :class="[`level-${level}`, { zooming, dragging, mirror }]" tabindex="0" role="slider" aria-label="人生时间轴，滚轮缩放，拖动平移，点击选中某一天" :aria-valuetext="selected" @wheel="onWheel" @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp" @pointercancel="onUp" @keydown="onKey">
    <button class="axis-dock icon-button" type="button" :title="mirror ? '把时间轴放到左侧' : '把时间轴放到右侧'" :aria-label="mirror ? '把时间轴放到左侧' : '把时间轴放到右侧'" @pointerdown.stop @click.stop="emit('toggle-dock')"><AppIcon name="layout" :size="14" /></button>
    <div class="axis-rail" aria-hidden="true">
      <div class="axis-track" />
      <i class="axis-selected" :style="selectedStyle" />
      <TransitionGroup name="tick" tag="div" class="axis-ticks">
        <div v-for="tick in ticks" :key="tick.key" class="tick" :class="[`tick-${tick.level}`, { strong: tick.strong, selected: tick.level === 'day' && tick.index === selectedIndex }]" :style="{ transform: `translateY(${tick.top}px)`, height: `${tick.height}px` }">
          <Transition name="tick"><time v-if="tick.label" class="tick-label">{{ tick.label }}</time></Transition>
          <span class="tick-mark" />
        </div>
      </TransitionGroup>
      <TransitionGroup name="medal" tag="div" class="axis-medals">
        <span v-for="medal in medals" :key="medal.key" class="axis-medal" :class="{ selected: medal.date === selected }" :style="{ top: `${medal.y}px` }" :title="medal.date"><AppIcon name="star" :size="10" :stroke-width="2.2" /></span>
      </TransitionGroup>
      <span class="axis-today" :style="todayStyle"><Transition name="tick"><em v-if="pxPerDay >= 22">今</em></Transition></span>
    </div>
    <footer class="axis-range mono" aria-hidden="true">{{ rangeLabel.from }} – {{ rangeLabel.to }}<small>{{ rangeLabel.days }} 天</small></footer>
  </aside>
</template>
