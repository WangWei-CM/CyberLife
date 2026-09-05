<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import interactionPlugin from '@fullcalendar/interaction'
import MetricLine, { type MetricPoint } from '../components/MetricLine.vue'
import PlanCarousel from '../components/PlanCarousel.vue'
import DiaryEditor from '../components/DiaryEditor.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import { api, type MoodTag, type NowData, type Plan, type Task, type TrendPoint } from '../api/client'
import { authState } from '../stores/auth'
import { useCountUp } from '../lib/motion'
import { addDaysISO, beijingNow, lunarLabel, monthDayLabel, timeLabel, todayISO, weekdayLabel } from '../lib/dates'

const props = defineProps<{ secret: boolean }>()
const emit = defineEmits<{ (event: 'navigate-future'): void }>()

const data = ref<NowData>({ diary: { id: '', entryDate: '', content: '', secret: false, commentable: false }, moods: [], bodies: [], tasks: [] })
const trend = ref<TrendPoint[]>([])
const tags = ref<MoodTag[]>([])
const plans = ref<Plan[]>([])
const selectedTags = ref<string[]>([])
const moodNote = ref('')
const bodyNote = ref('')
const bodyScore = ref(70)
const savedAt = ref('')
const saving = ref(false)
const now = ref(beijingNow())
const leftWidth = ref(Number(localStorage.getItem('now-left-width') || 45))
const dragging = ref(false)
const error = ref('')
const newTask = ref('')
const newTagOpen = ref(false)
const newTag = ref({ emoji: '🙂', name: '', value: 60 })
const calendarView = ref<'dayGridMonth' | 'dayGridDay'>('dayGridMonth')
const carouselInterval = Number(localStorage.getItem('now-plan-carousel-ms') || 6000)
const theme = computed(() => (document.querySelector('.app-shell')?.classList.contains('light') ? 'light' : 'dark') as 'light' | 'dark')
let saveTimer: number | undefined
let clock: number | undefined

const today = todayISO()
const dateLabel = computed(() => monthDayLabel(now.value))
const weekday = computed(() => weekdayLabel(now.value))
const lunar = computed(() => lunarLabel(now.value))
const hhmm = computed(() => timeLabel(now.value))
const seconds = computed(() => String(now.value.getSeconds()).padStart(2, '0'))
const activePlans = computed(() => plans.value.filter(plan => plan.startDate <= today && today <= plan.endDate && plan.progress < 100).sort((a, b) => a.endDate.localeCompare(b.endDate)))
const bodyPoints = computed<MetricPoint[]>(() => trend.value.map((point, index) => ({ x: index, y: point.body, label: monthDayLabel(point.date) })))
const moodPoints = computed<MetricPoint[]>(() => data.value.moods.map(record => { const at = new Date(record.recordedAt); return { x: at.getTime(), y: record.value, label: `${timeLabel(record.recordedAt)} ${record.tags.map(tag => tag.emoji).join('')}` } }).sort((a, b) => a.x - b.x))
const pendingCount = computed(() => data.value.tasks.filter(task => !task.done).length)
const pendingDisplay = useCountUp(() => pendingCount.value, 400)
const sortedTasks = computed(() => [...data.value.tasks].sort((a, b) => Number(a.done) - Number(b.done)))
const events = computed(() => data.value.tasks.map(task => ({ id: task.id, title: task.title, date: task.taskDate, classNames: [task.done ? 'fc-task-done' : 'fc-task-open', `fc-priority-${task.priority}`] })))
const calendarOptions = computed(() => ({
  plugins: [dayGridPlugin, interactionPlugin], initialView: calendarView.value, initialDate: today, now: today, locale: 'zh-cn', firstDay: 1,
  headerToolbar: { left: 'prev,next', center: 'title', right: 'dayGridDay,dayGridMonth' }, buttonText: { day: '今日', month: '本月' },
  height: 'auto', fixedWeekCount: false, dayMaxEventRows: 3, events: events.value,
  viewDidMount: (info: { view: { type: string } }) => { calendarView.value = info.view.type as 'dayGridMonth' | 'dayGridDay' },
}))
const vaultKey = computed(() => `${authState.actor?.lifeId ?? 'life'}:${today}`)

async function load() {
  try {
    const [todayData, tagData, planData, history] = await Promise.all([api.today(), api.moodTags(), api.plans(), api.history(addDaysISO(today, -6), today)])
    data.value = todayData
    tags.value = tagData.items
    plans.value = planData.items
    trend.value = history.points
    if (todayData.bodies.length) bodyScore.value = todayData.bodies[todayData.bodies.length - 1].score
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
async function refreshToday() { try { data.value = await api.today() } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' } }
async function recordMood() {
  if (!selectedTags.value.length) return
  try { await api.addMood(selectedTags.value, moodNote.value.trim(), props.secret); selectedTags.value = []; moodNote.value = ''; await refreshToday() } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function recordBody() {
  try {
    await api.addBody(bodyScore.value, bodyNote.value.trim(), props.secret)
    bodyNote.value = ''
    const [todayData, history] = await Promise.all([api.today(), api.history(addDaysISO(today, -6), today)])
    data.value = todayData
    trend.value = history.points
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function addTag() {
  if (!newTag.value.name.trim()) return
  try { await api.addMoodTag({ name: newTag.value.name.trim(), emoji: newTag.value.emoji.trim() || '🙂', value: newTag.value.value }); tags.value = (await api.moodTags()).items; newTag.value = { emoji: '🙂', name: '', value: 60 }; newTagOpen.value = false } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function toggleTask(task: Task) {
  const next = !task.done
  task.done = next
  try { await api.setTaskDone(task.id, next) } catch (cause) { task.done = !next; error.value = cause instanceof Error ? cause.message : '更新失败' }
}
async function addTask() {
  const title = newTask.value.trim()
  if (!title) return
  try { await api.addTask(title, '', 'normal'); newTask.value = ''; await refreshToday() } catch (cause) { error.value = cause instanceof Error ? cause.message : '添加失败' }
}
function toggleTag(id: string) { selectedTags.value = selectedTags.value.includes(id) ? selectedTags.value.filter(item => item !== id) : [...selectedTags.value, id] }
function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = window.setTimeout(async () => {
    saving.value = true
    try { await api.saveDraft(data.value.diary.content); await api.saveDiary(data.value.diary.content); savedAt.value = timeLabel(new Date()) } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { saving.value = false }
  }, 700)
}
function startResize(event: PointerEvent) {
  dragging.value = true
  document.body.classList.add('resizing-col')
  const move = (next: PointerEvent) => { leftWidth.value = Math.max(32, Math.min(68, (next.clientX / window.innerWidth) * 100)) }
  const up = () => { dragging.value = false; document.body.classList.remove('resizing-col'); localStorage.setItem('now-left-width', String(leftWidth.value)); window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up) }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
  event.preventDefault()
}
watch(() => props.secret, () => { error.value = '' })
onMounted(() => { clock = window.setInterval(() => { now.value = beijingNow() }, 1000); load() })
onBeforeUnmount(() => { if (clock) clearInterval(clock); if (saveTimer) clearTimeout(saveTimer) })
</script>

<template>
  <main class="page now-page">
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <section class="now-layout" :style="{ '--left-width': `${leftWidth}%` }">
      <div v-stagger class="now-left">
        <section class="now-clock" aria-live="off">
          <Transition name="fade" mode="out-in"><strong :key="dateLabel" class="clock-date">{{ dateLabel }}</strong></Transition>
          <span class="clock-week">{{ weekday }}</span>
          <Transition name="fade" mode="out-in"><span :key="lunar" class="clock-lunar">{{ lunar }}</span></Transition>
          <time class="clock-time mono" :datetime="now.toISOString()">{{ hhmm }}<Transition name="fade" mode="out-in"><small :key="seconds" class="clock-seconds">{{ seconds }}</small></Transition></time>
        </section>

        <PlanCarousel v-if="activePlans.length" :plans="activePlans" :interval="carouselInterval" @select="emit('navigate-future')" />

        <article class="card now-card body-card">
          <div class="now-chart">
            <h2 class="card-title">身体<small>近七日</small></h2>
            <MetricLine :points="bodyPoints" :min="0" :max="100" :height="128" empty="这七天还没有身体记录" />
          </div>
          <form class="now-form" @submit.prevent="recordBody">
            <div class="score-row"><span class="faint">今天的状态</span><output class="score mono">{{ bodyScore }}</output></div>
            <input v-model.number="bodyScore" type="range" min="0" max="100" aria-label="身体评分" :style="{ '--range-fill': `${bodyScore}%` }" />
            <input v-model="bodyNote" placeholder="备注（可选）" maxlength="200" />
            <button class="primary" type="submit">记录</button>
          </form>
        </article>

        <article class="card now-card mood-card">
          <div class="now-chart">
            <h2 class="card-title">心情<small>今天 {{ data.moods.length ? `${data.moods.length} 次` : '' }}</small></h2>
            <MetricLine :points="moodPoints" :min="0" :max="100" :height="128" empty="今天还没有心情记录" />
          </div>
          <form class="now-form" @submit.prevent="recordMood">
            <div class="tag-head">
              <span class="faint">选择标签</span>
              <div class="tag-manage">
                <button type="button" class="text-button" :aria-expanded="newTagOpen" @click="newTagOpen = !newTagOpen"><AppIcon name="plus" :size="14" />新标签</button>
                <Transition name="popover">
                  <div v-if="newTagOpen" class="popover tag-popover">
                    <div class="form-row"><input v-model="newTag.emoji" class="tag-emoji-input" maxlength="4" aria-label="emoji" /><input v-model="newTag.name" placeholder="名称" maxlength="8" aria-label="名称" /></div>
                    <label class="field"><span>情绪值 <b class="mono">{{ newTag.value }}</b></span><input v-model.number="newTag.value" type="range" min="1" max="100" :style="{ '--range-fill': `${newTag.value}%` }" /></label>
                    <div class="form-row"><button type="button" class="primary" @click="addTag">添加</button><button type="button" class="text-button" @click="newTagOpen = false">取消</button></div>
                  </div>
                </Transition>
              </div>
            </div>
            <div class="tag-grid" role="group" aria-label="心情标签">
              <button v-for="tag in tags" :key="tag.id" v-glow type="button" class="tag-item" :class="{ selected: selectedTags.includes(tag.id) }" :aria-pressed="selectedTags.includes(tag.id)" @click="toggleTag(tag.id)"><i>{{ tag.emoji }}</i><span>{{ tag.name }}</span></button>
            </div>
            <EmptyState v-if="!tags.length" icon="smile" text="先添加几个心情标签" compact />
            <input v-model="moodNote" placeholder="备注（可选）" maxlength="200" />
            <button class="primary" type="submit" :disabled="!selectedTags.length">记录</button>
          </form>
        </article>

        <article class="card diary-card">
          <header class="card-head">
            <h2>日记<span v-if="secret" class="secret-badge">绝密模式</span></h2>
            <Transition name="fade" mode="out-in"><small :key="savedAt + String(saving)" class="faint mono">{{ saving ? '' : savedAt ? `已保存 ${savedAt}` : '' }}</small></Transition>
          </header>
          <DiaryEditor v-model="data.diary.content" :theme="theme" :vault-key="vaultKey" :secret="secret" placeholder="今天……" @change="scheduleSave" />
        </article>
      </div>

      <div class="divider-v now-divider" :class="{ dragging }" role="separator" aria-orientation="vertical" aria-label="拖动调整左右栏宽度" @pointerdown="startResize" />

      <aside v-stagger class="now-right">
        <section class="task-head">
          <h2 class="card-title">待办<small>{{ Math.round(pendingDisplay) }} 项未完成</small></h2>
          <form class="task-add" @submit.prevent="addTask"><input v-model="newTask" placeholder="添加今天的待办，回车保存" maxlength="120" aria-label="新待办" /><button class="icon-button" type="submit" aria-label="添加" :disabled="!newTask.trim()"><AppIcon name="plus" /></button></form>
        </section>
        <section class="card calendar-card"><FullCalendar :options="calendarOptions" /></section>
        <section class="card tasks-card">
          <TransitionGroup v-if="sortedTasks.length" name="list" tag="ul" class="today-tasks">
            <li v-for="task in sortedTasks" :key="task.id" :class="{ done: task.done, [`priority-${task.priority}`]: true }">
              <label class="task-row">
                <input type="checkbox" :checked="task.done" @change="toggleTask(task)" />
                <span class="task-body">
                  <span class="task-title" :class="{ done: task.done }">{{ task.title }}</span>
                  <small v-if="task.description" class="faint">{{ task.description }}</small>
                </span>
                <i class="task-priority" :title="task.priority === 'high' ? '高优先级' : task.priority === 'low' ? '低优先级' : '普通'" />
              </label>
            </li>
          </TransitionGroup>
          <EmptyState v-else icon="check" text="今天还没有待办" />
        </section>
      </aside>
    </section>
  </main>
</template>
