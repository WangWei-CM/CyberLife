<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import interactionPlugin from '@fullcalendar/interaction'
import { MdEditor } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import MetricLine from '../components/MetricLine.vue'
import { api, type MoodTag, type NowData, type Plan } from '../api/client'

const emit = defineEmits<{ (event: 'navigate-future'): void }>()
const data = ref<NowData>({ diary: { id: '', entryDate: '', content: '', secret: false, commentable: false }, moods: [], bodies: [], tasks: [] })
const tags = ref<MoodTag[]>([])
const plans = ref<Plan[]>([])
const selectedTags = ref<string[]>([])
const moodNote = ref('')
const bodyNote = ref('')
const bodyScore = ref(70)
const savedAt = ref('')
const now = ref(new Date())
const leftWidth = ref(Number(localStorage.getItem('now-left-width') || 45))
const dragging = ref(false)
const error = ref('')
const activePlan = ref(0)
const carouselPaused = ref(false)
const carouselInterval = ref(Number(localStorage.getItem('now-plan-carousel-ms') || 6000))
let timer: number | undefined
let clock: number | undefined
let carouselTimer: number | undefined

const lunar = computed(() => new Intl.DateTimeFormat('zh-CN-u-ca-chinese', { month: 'long', day: 'numeric' }).format(now.value))
const dateLabel = computed(() => new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(now.value))
const timeLabel = computed(() => now.value.toLocaleTimeString('zh-CN', { hour12: false }))
const moodValues = computed(() => data.value.moods.map(x => x.value))
const bodyValues = computed(() => data.value.bodies.slice(-7).map(x => x.score))
const events = computed(() => data.value.tasks.map(x => ({ id: x.id, title: x.title, date: x.taskDate, classNames: x.done ? ['fc-task-done'] : [] })))
const activePlans = computed(() => {
  const today = new Date().toISOString().slice(0, 10)
  return plans.value.filter(plan => plan.startDate <= today && today <= plan.endDate && plan.progress < 100)
})
const currentPlan = computed(() => activePlans.value[activePlan.value] ?? null)
const calendarOptions = computed(() => ({ plugins: [dayGridPlugin, interactionPlugin], initialView: 'dayGridDay', initialDate: new Date(), headerToolbar: { left: 'prev,next', center: 'title', right: 'dayGridDay,dayGridMonth' }, height: 'auto', events: events.value, dateClick: (info: { dateStr: string }) => { bodyNote.value = `${info.dateStr} 的待办` } }))

async function load() {
  try {
    const [today, tagData, planData] = await Promise.all([api.today(), api.moodTags(), api.plans()])
    data.value = today
    tags.value = tagData.items
    plans.value = planData.items
    activePlan.value = Math.min(activePlan.value, Math.max(activePlans.value.length - 1, 0))
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
async function recordMood() { try { await api.addMood(selectedTags.value, moodNote.value, false); selectedTags.value = []; moodNote.value = ''; await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } }
async function recordBody() { try { await api.addBody(bodyScore.value, bodyNote.value, false); bodyNote.value = ''; await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } }
async function toggleTask(id: string, done: boolean) { try { await api.setTaskDone(id, done); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '更新失败' } }
function saveDiary() { if (timer) clearTimeout(timer); timer = window.setTimeout(async () => { try { await api.saveDraft(data.value.diary.content); await api.saveDiary(data.value.diary.content); savedAt.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } }, 700) }
function startResize(event: PointerEvent) { dragging.value = true; (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId); const move = (next: PointerEvent) => { leftWidth.value = Math.max(32, Math.min(68, next.clientX / window.innerWidth * 100)); localStorage.setItem('now-left-width', String(leftWidth.value)) }; const up = () => { dragging.value = false; window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }; window.addEventListener('pointermove', move); window.addEventListener('pointerup', up) }
function choosePlan(index: number) { activePlan.value = index; restartCarousel() }
function restartCarousel() { if (carouselTimer) clearInterval(carouselTimer); if (!carouselPaused.value && activePlans.value.length > 1) carouselTimer = window.setInterval(() => { activePlan.value = (activePlan.value + 1) % activePlans.value.length }, carouselInterval.value) }
function pauseCarousel(paused: boolean) { carouselPaused.value = paused; restartCarousel() }
watch([activePlans, carouselInterval], restartCarousel)
onMounted(() => { clock = window.setInterval(() => now.value = new Date(), 1000); load(); restartCarousel() })
onBeforeUnmount(() => { if (clock) clearInterval(clock); if (timer) clearTimeout(timer); if (carouselTimer) clearInterval(carouselTimer) })
</script>

<template>
  <main class="page now-page">
    <p v-if="error" class="error">{{ error }}</p>
    <section class="now-layout" :style="{ '--left-width': `${leftWidth}%` }">
      <div class="now-left">
        <section class="now-clock"><strong>{{ dateLabel }}</strong><span>{{ lunar }}</span><time><span>{{ timeLabel.slice(0, 5) }}</span><small :key="timeLabel.slice(5)" class="clock-seconds">{{ timeLabel.slice(5) }}</small></time></section>
        <section v-if="currentPlan" class="plan-carousel" @mouseenter="pauseCarousel(true)" @mouseleave="pauseCarousel(false)" @focusin="pauseCarousel(true)" @focusout="pauseCarousel(false)">
          <Transition name="carousel" mode="out-in"><button :key="currentPlan.id" class="plan-banner" @click="emit('navigate-future')"><b>{{ currentPlan.name }}</b><span class="banner-bar"><i :style="{ width: `${currentPlan.timeProgress}%` }" /></span><span class="banner-bar plan"><i :style="{ width: `${currentPlan.progress}%` }" /></span></button></Transition>
          <div v-if="activePlans.length > 1" class="carousel-dots" aria-label="进行中规划">
            <button v-for="(_, index) in activePlans" :key="index" :class="{ active: index === activePlan }" :aria-label="`第 ${index + 1} 项规划`" @click="choosePlan(index)" />
          </div>
        </section>
        <article class="now-card metric-card"><div class="metric-chart"><MetricLine :values="bodyValues" /><span>近七日</span></div><form @submit.prevent="recordBody"><h2>身体</h2><output>{{ bodyScore }}</output><input v-model.number="bodyScore" type="range" min="0" max="100" /><input v-model="bodyNote" placeholder="备注" /><button class="primary">记录</button></form></article>
        <article class="now-card metric-card"><div class="metric-chart"><MetricLine :values="moodValues" /><span>今日</span></div><form @submit.prevent="recordMood"><h2>心情</h2><div class="tag-grid"><button v-for="tag in tags" :key="tag.id" type="button" class="tag-item glow-spot" :class="{ selected: selectedTags.includes(tag.id) }" @click="selectedTags.includes(tag.id) ? selectedTags = selectedTags.filter(x => x !== tag.id) : selectedTags.push(tag.id)"><i>{{ tag.emoji }}</i><span>{{ tag.name }}</span></button></div><input v-model="moodNote" placeholder="备注" /><button class="primary" :disabled="!selectedTags.length">记录</button></form></article>
        <article class="now-card diary-card"><header><h2>日记</h2><small v-if="savedAt">已保存 {{ savedAt }}</small></header><MdEditor v-model="data.diary.content" language="zh-CN" :preview="true" @onChange="saveDiary" @onSave="saveDiary" /></article>
      </div>
      <div class="now-divider" :class="{ dragging }" @pointerdown="startResize" />
      <aside class="now-right"><section class="task-head"><h2>待办</h2><span>{{ data.tasks.filter(task => !task.done).length }} 项</span></section><FullCalendar :options="calendarOptions" /><ul class="today-tasks"><li v-for="task in data.tasks" :key="task.id"><label><input type="checkbox" :checked="task.done" @change="toggleTask(task.id, !task.done)" /><span :class="{ done: task.done }">{{ task.title }}</span></label></li></ul></aside>
    </section>
  </main>
</template>
