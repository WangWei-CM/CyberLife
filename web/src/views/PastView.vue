<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MetricLine from '../components/MetricLine.vue'
import { api, type HistoryDay, type HistoryRange } from '../api/client'

type Scale = 'day' | 'month' | 'year'
type Tick = { key: string; date: string; label: string; milestone: boolean; today: boolean }
const today = new Date()
const iso = (value: Date) => value.toISOString().slice(0, 10)
const add = (value: Date, days: number) => { const result = new Date(value); result.setDate(result.getDate() + days); return result }
const monthStart = (value: Date) => new Date(value.getFullYear(), value.getMonth(), 1)
const yearStart = (value: Date) => new Date(value.getFullYear(), 0, 1)
const spanDays = ref(14)
const anchor = ref(today)
const range = ref<HistoryRange>({ from: iso(add(today, -13)), to: iso(today), days: [], points: [] })
const selected = ref(iso(today))
const error = ref('')
let dragStart: { y: number; date: Date } | null = null

const scale = computed<Scale>(() => spanDays.value <= 35 ? 'day' : spanDays.value <= 730 ? 'month' : 'year')
const selectedDay = computed<HistoryDay | undefined>(() => range.value.days.find(day => day.date === selected.value))
const mood = computed(() => range.value.points.map(point => point.mood ?? 0))
const body = computed(() => range.value.points.map(point => point.body ?? 0))
const ticks = computed<Tick[]>(() => {
  const milestoneDates = new Set(range.value.days.filter(day => day.milestoneCount > 0).map(day => day.date))
  const start = new Date(`${range.value.from}T00:00:00`)
  const end = new Date(`${range.value.to}T00:00:00`)
  const result: Tick[] = []
  if (scale.value === 'day') {
    for (let value = start; value <= end; value = add(value, 1)) {
      const date = iso(value)
      const crossMonth = value.getDate() === 1 || date === range.value.from
      result.push({ key: date, date, label: crossMonth ? `${value.getMonth() + 1}月${value.getDate()}日` : `${value.getDate()}日`, milestone: milestoneDates.has(date), today: date === iso(today) })
    }
  } else if (scale.value === 'month') {
    for (let value = monthStart(start); value <= end; value = new Date(value.getFullYear(), value.getMonth() + 1, 1)) {
      const date = iso(value)
      const milestone = [...milestoneDates].some(item => item.startsWith(date.slice(0, 7)))
      result.push({ key: date, date, label: `${value.getMonth() + 1}月`, milestone, today: iso(today).startsWith(date.slice(0, 7)) })
    }
  } else {
    for (let value = yearStart(start); value <= end; value = new Date(value.getFullYear() + 1, 0, 1)) {
      const date = iso(value)
      const milestone = [...milestoneDates].some(item => item.startsWith(String(value.getFullYear())))
      result.push({ key: date, date, label: `${value.getFullYear()}年`, milestone, today: today.getFullYear() === value.getFullYear() })
    }
  }
  return result
})

async function load() {
  const end = anchor.value > today ? today : anchor.value
  const start = add(end, -spanDays.value + 1)
  try {
    range.value = await api.history(iso(start), iso(end))
    if (selected.value < range.value.from || selected.value > range.value.to) selected.value = range.value.to
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
function wheel(event: WheelEvent) { event.preventDefault(); spanDays.value = Math.max(5, Math.min(36500, Math.round(spanDays.value * (event.deltaY < 0 ? .65 : 1.55)))); load() }
function down(event: PointerEvent) { dragStart = { y: event.clientY, date: new Date(anchor.value) }; (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId) }
function move(event: PointerEvent) { if (!dragStart) return; anchor.value = add(dragStart.date, Math.round((dragStart.y - event.clientY) * Math.max(1, spanDays.value / 280))); if (anchor.value > today) anchor.value = today; load() }
function up() { dragStart = null }
function selectTick(tick: Tick) { const date = new Date(`${tick.date}T00:00:00`); if (scale.value === 'month') date.setDate(Math.min(new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate(), today.getDate())); if (scale.value === 'year') date.setMonth(today.getMonth(), today.getDate()); selected.value = iso(date > today ? today : date) }
onMounted(load)
</script>

<template>
  <main class="page past-page">
    <p v-if="error" class="error">{{ error }}</p>
    <section class="past-layout">
      <aside class="life-axis" :class="`axis-${scale}`" @wheel="wheel" @pointerdown="down" @pointermove="move" @pointerup="up">
        <button v-for="tick in ticks" :key="tick.key" :class="{ selected: selected.startsWith(tick.date.slice(0, scale === 'day' ? 10 : scale === 'month' ? 7 : 4)), today: tick.today }" @click="selectTick(tick)">
          <span v-if="tick.milestone" class="medal" aria-label="里程碑">✦</span><time>{{ tick.label }}</time><i v-if="tick.today" aria-label="今天" />
        </button>
      </aside>
      <section class="past-content">
        <div class="past-trends"><article class="past-card"><h2>心情</h2><MetricLine :values="mood" /></article><article class="past-card"><h2>身体</h2><MetricLine :values="body" /></article></div>
        <div v-if="selectedDay" class="past-day"><article class="past-card diary-render"><header><h2>{{ selectedDay.date }}</h2><span v-if="selectedDay.milestoneCount" class="medal">✦</span></header><div v-if="selectedDay.diary.id" class="markdown-view">{{ selectedDay.diary.content }}</div><div v-else class="empty">这一天没有可查看的日记</div></article><article class="past-card"><h2>日程</h2><ul class="past-tasks"><li v-for="task in selectedDay.tasks" :key="task.id" :class="{ done: task.done }"><span>{{ task.title }}</span><small>{{ task.description }}</small></li><li v-if="!selectedDay.tasks.length" class="empty">这一天没有可查看的日程</li></ul></article></div>
      </section>
    </section>
  </main>
</template>
