<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { FutureTask, Plan } from '../api/client'
import { createApproach } from '../lib/motion'
import { dateFromIndex, dayIndex, daysInMonth, iso } from '../lib/dates'

/**
 * 自研可缩放待办日历（未来页）：横向时间条，连续缩放 7 天 → 100 年，
 * 列粒度按每像素天数连续过渡（天 → 月 → 年），规划以横条落位，任务按日期落位。
 */
type Column = { key: string; level: 'day' | 'month' | 'year'; left: number; width: number; start: number; span: number; label: string; sub: string; past: boolean; today: boolean; weekend: boolean }
const props = defineProps<{ tasks: FutureTask[]; plans: Plan[]; selected: string; selectedPlan?: string; today: string }>()
const emit = defineEmits<{ (event: 'update:selected', value: string): void; (event: 'select-plan', id: string): void; (event: 'range', from: string, to: string): void }>()

const MIN_DAYS = 7
const MAX_DAYS = 36500
const root = ref<HTMLElement>()
const width = ref(1000)
const displayDays = ref(MIN_DAYS)
const left = ref(dayIndex(props.today))
const zooming = ref(false)
const dragging = ref(false)
let anchorDay = dayIndex(props.today)
let anchorFraction = 0
let dragStart: { x: number; left: number; moved: boolean } | null = null
let rangeTimer: number | undefined
let observer: ResizeObserver | undefined

const todayIndex = computed(() => dayIndex(props.today))
const pxPerDay = computed(() => width.value / displayDays.value)
const level = computed<'day' | 'month' | 'year'>(() => pxPerDay.value >= 22 ? 'day' : pxPerDay.value >= 1.4 ? 'month' : 'year')
const right = computed(() => left.value + displayDays.value)
const approach = createApproach({
  factor: .12,
  epsilon: .005,
  onFrame(value) { displayDays.value = value; left.value = anchorDay - anchorFraction * value },
  onSettle() { zooming.value = false; scheduleRange() },
})
const x = (day: number) => (day - left.value) * pxPerDay.value

const columns = computed<Column[]>(() => {
  const result: Column[] = []
  const first = Math.floor(left.value) - 1
  const last = Math.ceil(right.value) + 1
  if (level.value === 'day') {
    for (let index = first; index <= last; index++) {
      const date = dateFromIndex(index)
      const weekday = date.getDay()
      const labelEvery = Math.max(1, Math.ceil(56 / pxPerDay.value))
      const show = date.getDate() === 1 || (index - todayIndex.value) % labelEvery === 0
      result.push({ key: `d${index}`, level: 'day', left: x(index), width: pxPerDay.value, start: index, span: 1, label: show ? `${date.getDate()}` : '', sub: show ? (date.getDate() === 1 ? `${date.getMonth() + 1}月` : ['日', '一', '二', '三', '四', '五', '六'][weekday]) : '', past: index < todayIndex.value, today: index === todayIndex.value, weekend: weekday === 0 || weekday === 6 })
    }
    return result
  }
  const firstDate = dateFromIndex(first)
  if (level.value === 'month') {
    let cursor = new Date(firstDate.getFullYear(), firstDate.getMonth(), 1)
    while (dayIndex(cursor) <= last) {
      const start = dayIndex(cursor)
      const span = daysInMonth(cursor.getFullYear(), cursor.getMonth())
      const wide = span * pxPerDay.value > 54
      result.push({ key: `m${cursor.getFullYear()}-${cursor.getMonth()}`, level: 'month', left: x(start), width: span * pxPerDay.value, start, span, label: wide || cursor.getMonth() % 3 === 0 ? `${cursor.getMonth() + 1}月` : '', sub: cursor.getMonth() === 0 || (result.length === 0 && wide) ? `${cursor.getFullYear()}` : '', past: start + span <= todayIndex.value, today: start <= todayIndex.value && todayIndex.value < start + span, weekend: false })
      cursor = new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1)
    }
    return result
  }
  let year = firstDate.getFullYear()
  while (dayIndex(new Date(year, 0, 1)) <= last) {
    const start = dayIndex(new Date(year, 0, 1))
    const span = dayIndex(new Date(year + 1, 0, 1)) - start
    const labelEvery = Math.max(1, Math.ceil(48 / (span * pxPerDay.value)))
    result.push({ key: `y${year}`, level: 'year', left: x(start), width: span * pxPerDay.value, start, span, label: year % labelEvery === 0 ? `${year}` : '', sub: '', past: start + span <= todayIndex.value, today: start <= todayIndex.value && todayIndex.value < start + span, weekend: false })
    year++
  }
  return result
})
const taskItems = computed(() => {
  const groups = new Map<string, FutureTask[]>()
  for (const task of props.tasks) {
    const index = dayIndex(task.date)
    if (index < left.value - 1 || index > right.value + 1) continue
    const column = level.value === 'day' ? `d${index}` : level.value === 'month' ? `m${task.date.slice(0, 7)}` : `y${task.date.slice(0, 4)}`
    if (!groups.has(column)) groups.set(column, [])
    groups.get(column)!.push(task)
  }
  return groups
})
function tasksFor(column: Column) {
  const key = column.level === 'day' ? column.key : column.level === 'month' ? `m${iso(dateFromIndex(column.start)).slice(0, 7)}` : `y${iso(dateFromIndex(column.start)).slice(0, 4)}`
  return taskItems.value.get(key) ?? []
}
const planBars = computed(() => {
  const laneEnds: number[] = []
  return props.plans
    .map(plan => ({ plan, start: dayIndex(plan.startDate), end: dayIndex(plan.endDate) + 1 }))
    .sort((a, b) => a.start - b.start || a.end - b.end)
    .map(item => {
      let lane = laneEnds.findIndex(end => end <= item.start)
      if (lane < 0) { lane = laneEnds.length; laneEnds.push(item.end) } else laneEnds[lane] = item.end
      return { ...item, lane: lane % 3, left: x(item.start), width: Math.max(4, (item.end - item.start) * pxPerDay.value) }
    })
    .filter(item => item.end >= left.value - 1 && item.start <= right.value + 1)
})
const todayX = computed(() => x(todayIndex.value))
const selectedX = computed(() => ({ left: x(dayIndex(props.selected)), width: Math.max(2, pxPerDay.value) }))
const rangeLabel = computed(() => {
  const from = dateFromIndex(Math.floor(left.value))
  const to = dateFromIndex(Math.ceil(right.value) - 1)
  const days = Math.round(displayDays.value)
  const scale = days < 60 ? `${days} 天` : days < 730 ? `${Math.round(days / 30.4)} 个月` : `${Math.round(days / 365)} 年`
  return { from: `${from.getFullYear()}年${from.getMonth() + 1}月${from.getDate()}日`, to: `${to.getFullYear()}年${to.getMonth() + 1}月${to.getDate()}日`, scale }
})

function scheduleRange() {
  if (rangeTimer) clearTimeout(rangeTimer)
  rangeTimer = window.setTimeout(() => emit('range', iso(dateFromIndex(Math.floor(left.value))), iso(dateFromIndex(Math.ceil(right.value)))), 120)
}
function dayAt(clientX: number) { const bounds = root.value!.getBoundingClientRect(); return left.value + (clientX - bounds.left) / pxPerDay.value }
function onWheel(event: WheelEvent) {
  event.preventDefault()
  const bounds = root.value!.getBoundingClientRect()
  anchorDay = dayAt(event.clientX)
  anchorFraction = (event.clientX - bounds.left) / width.value
  const factor = Math.exp(Math.max(-1, Math.min(1, event.deltaY / 60)) * Math.log(1.08))
  zooming.value = true
  approach.set(Math.max(MIN_DAYS, Math.min(MAX_DAYS, approach.target * factor)))
}
function onDown(event: PointerEvent) { if (event.button !== 0) return; dragStart = { x: event.clientX, left: left.value, moved: false }; root.value?.setPointerCapture(event.pointerId) }
function onMove(event: PointerEvent) {
  if (!dragStart) return
  const dx = event.clientX - dragStart.x
  if (Math.abs(dx) > 4) { dragStart.moved = true; dragging.value = true }
  if (!dragStart.moved) return
  left.value = dragStart.left - dx / pxPerDay.value
  anchorDay = left.value + anchorFraction * displayDays.value
}
function onUp(event: PointerEvent) {
  if (!dragStart) return
  const moved = dragStart.moved
  dragStart = null
  dragging.value = false
  if (moved) { scheduleRange(); return }
  const target = event.target as HTMLElement
  const planId = target.closest<HTMLElement>('[data-plan]')?.dataset.plan
  if (planId) { emit('select-plan', planId); return }
  emit('update:selected', iso(dateFromIndex(Math.floor(dayAt(event.clientX)))))
}
function zoomBy(factor: number) { const bounds = root.value!.getBoundingClientRect(); anchorDay = dayAt(bounds.left + width.value / 2); anchorFraction = .5; zooming.value = true; approach.set(Math.max(MIN_DAYS, Math.min(MAX_DAYS, approach.target * factor))) }
function goToday() { left.value = todayIndex.value - displayDays.value * .15; anchorDay = left.value + anchorFraction * displayDays.value; scheduleRange() }
watch(() => props.selected, value => { const index = dayIndex(value); if (index < left.value || index + 1 > right.value) { left.value = index - displayDays.value * .15; anchorDay = left.value + anchorFraction * displayDays.value; scheduleRange() } })
onMounted(() => {
  const measure = () => { width.value = Math.max(320, root.value?.clientWidth ?? 1000); scheduleRange() }
  measure()
  if ('ResizeObserver' in window && root.value) { observer = new ResizeObserver(measure); observer.observe(root.value) }
  approach.set(MIN_DAYS, true)
})
onBeforeUnmount(() => { approach.stop(); observer?.disconnect(); if (rangeTimer) clearTimeout(rangeTimer) })
defineExpose({ zoomBy, goToday, rangeLabel })
</script>

<template>
  <div class="zoom-calendar-wrap">
    <div class="zoom-toolbar">
      <span class="cyber-heading">{{ rangeLabel.from }} — {{ rangeLabel.to }}</span>
      <span class="zoom-scale mono">{{ rangeLabel.scale }}</span>
      <div class="zoom-actions">
        <button class="icon-button" aria-label="放大" @click="zoomBy(.7)"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4.5-4.5M11 8v6M8 11h6" /></svg></button>
        <button class="icon-button" aria-label="缩小" @click="zoomBy(1.45)"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4.5-4.5M8 11h6" /></svg></button>
        <button class="text-button" @click="goToday">今天</button>
      </div>
    </div>
    <div ref="root" class="zoom-calendar" :class="[`level-${level}`, { zooming, dragging }]" tabindex="0" role="application" aria-label="待办日历，滚轮缩放，拖动平移，点击选中日期" @wheel="onWheel" @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp" @pointercancel="onUp">
      <div class="cal-content">
        <TransitionGroup name="tick" tag="div" class="cal-columns">
          <div v-for="column in columns" :key="column.key" class="cal-col" :class="[`col-${column.level}`, { past: column.past, today: column.today, weekend: column.weekend }]" :style="{ transform: `translateX(${column.left}px) skewX(-15deg)`, width: `${column.width}px` }">
            <div class="cal-col-content">
              <Transition name="tick"><header v-if="column.label" class="cal-label"><b>{{ column.label }}</b><small v-if="column.sub">{{ column.sub }}</small></header></Transition>
              <div class="cal-events">
                <template v-if="column.level === 'day'">
                  <span v-for="task in tasksFor(column).slice(0, 6)" :key="task.id" class="cal-event" :class="{ done: task.done, overdue: !task.done && column.past, high: task.priority === 'high' }" :title="task.title">{{ task.title }}</span>
                  <small v-if="tasksFor(column).length > 6" class="cal-more mono">+{{ tasksFor(column).length - 6 }}</small>
                </template>
                <template v-else>
                  <span v-if="tasksFor(column).length" class="cal-count mono" :class="{ wide: column.width > 40 }"><i v-for="task in tasksFor(column).slice(0, 12)" :key="task.id" :class="{ done: task.done, overdue: !task.done && dayIndex(task.date) < todayIndex }" /><b v-if="column.width > 40">{{ tasksFor(column).length }}</b></span>
                </template>
              </div>
            </div>
          </div>
        </TransitionGroup>
        <div class="cal-plans">
          <TransitionGroup name="tick">
            <button v-for="item in planBars" :key="item.plan.id" class="cal-plan" :class="{ active: item.plan.id === selectedPlan, done: item.plan.progress >= 100 }" :data-plan="item.plan.id" :style="{ transform: `translateX(${item.left}px)`, width: `${item.width}px`, top: `${item.lane * 22}px` }" :title="item.plan.name" type="button">
              <i :style="{ width: `${item.plan.progress}%` }" /><span v-if="item.width > 60">{{ item.plan.name }}</span>
            </button>
          </TransitionGroup>
        </div>
        <i class="cal-selected" :style="{ transform: `translateX(${selectedX.left}px) skewX(-15deg)`, width: `${selectedX.width}px` }" aria-hidden="true" />
        <i class="cal-today" :style="{ transform: `translateX(${todayX}px) skewX(-15deg)` }" aria-hidden="true" />
      </div>
    </div>
  </div>
</template>
