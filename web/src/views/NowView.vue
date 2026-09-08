<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import MetricLine, { type MetricPoint } from '../components/MetricLine.vue'
import PlanCarousel from '../components/PlanCarousel.vue'
import DiaryEditor from '../components/DiaryEditor.vue'
import MarkdownPreview from '../components/MarkdownPreview.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import ScheduleNowCard from '../components/ScheduleNowCard.vue'
import { api, type MoodTag, type NowData, type Plan, type Task, type TrendPoint, type ScheduleAgenda } from '../api/client'
import { authState } from '../stores/auth'
import { useCountUp } from '../lib/motion'
import { addDaysISO, beijingNow, fullDateLabel, lunarLabel, monthDayLabel, timeLabel, todayISO, weekdayLabel } from '../lib/dates'

const props = defineProps<{ secret: boolean }>()
const emit = defineEmits<{ (event: 'navigate-future'): void }>()

type CardKey = 'metrics' | 'diary'
const DEFAULT_ORDER: CardKey[] = ['metrics', 'diary']
const data = ref<NowData>({ diary: { id: '', entryDate: '', content: '', secret: false, commentable: false }, secretDiary: { id: '', entryDate: '', content: '', secret: true, commentable: false }, moods: [], bodies: [], tasks: [] })
const agenda = ref<ScheduleAgenda | null>(null)
const trend = ref<TrendPoint[]>([])
const tags = ref<MoodTag[]>([])
const plans = ref<Plan[]>([])
const selectedTags = ref<string[]>([])
const editingMetric = ref<'mood' | 'body' | null>(null)
const deletingMetric = ref(false)
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
const selectedTask = ref<Task | null>(null)
const taskEditing = ref(false)
const taskDraft = ref({ title: '', description: '', priority: 'normal' as Task['priority'] })
const taskBusy = ref(false)
const presets = ref<{ id: string; name: string }[]>([])
const taskAccessDraft = ref({ presetId: '', secret: false, commentable: false })
const milestoneDraft = ref({ description: '', detail: '' })
const cardOrder = ref<CardKey[]>(readOrder())
const dragCard = ref<CardKey | null>(null)
const carouselInterval = Number(localStorage.getItem('now-plan-carousel-ms') || 6000)
const theme = computed(() => (document.querySelector('.app-shell')?.classList.contains('light') ? 'light' : 'dark') as 'light' | 'dark')
let saveTimer: number | undefined
let clock: number | undefined
let agendaTimer: number | undefined
let dayKey = todayISO()

const today = computed(() => dayKey)
const dateLabel = computed(() => fullDateLabel(now.value))
const weekday = computed(() => weekdayLabel(now.value))
const lunar = computed(() => lunarLabel(now.value))
const hhmm = computed(() => timeLabel(now.value))
const seconds = computed(() => String(now.value.getSeconds()).padStart(2, '0'))
const dayProgress = computed(() => ((now.value.getHours() * 60 + now.value.getMinutes()) / 1440) * 100)
const activePlans = computed(() => plans.value.filter(plan => plan.startDate <= today.value && today.value <= plan.endDate && plan.progress < 100).sort((a, b) => a.endDate.localeCompare(b.endDate)))
const bodyPoints = computed<MetricPoint[]>(() => {
  const records = [...data.value.bodies].sort((a, b) => a.recordedAt.localeCompare(b.recordedAt))
  return trend.value.flatMap(point => {
    const dayAtNoon = new Date(`${point.date}T12:00:00+08:00`).getTime()
    if (point.date !== today.value || !records.length) return [{ x: dayAtNoon, y: point.body, label: monthDayLabel(point.date) }]
    return records.map(record => ({ x: new Date(record.recordedAt).getTime(), y: record.score, label: timeLabel(record.recordedAt) }))
  })
})
const moodPoints = computed<MetricPoint[]>(() => data.value.moods.map(record => { const at = new Date(record.recordedAt); return { x: at.getTime(), y: record.value, label: `${timeLabel(record.recordedAt)} ${record.tags.map(tag => tag.emoji).join('')}` } }).sort((a, b) => a.x - b.x))
const pendingCount = computed(() => data.value.tasks.filter(task => !task.done).length)
const pendingDisplay = useCountUp(() => pendingCount.value, 400)
const sortedTasks = computed(() => [...data.value.tasks].sort((a, b) => Number(a.done) - Number(b.done)))
const themeClass = computed(() => document.querySelector('.app-shell')?.className.replace(/\b(shell-enter|drop-target|theme-shift)\b/g, '').trim() ?? '')
const vaultKey = computed(() => `${authState.actor?.lifeId ?? 'life'}:${today.value}${props.secret ? ':secret' : ''}`)
/** 绝密模式下编辑当天的绝密层日记，公开层不受影响。 */
const activeDiary = computed(() => props.secret ? data.value.secretDiary! : data.value.diary)
function normalize(payload: NowData): NowData {
  return { ...payload, secretDiary: payload.secretDiary ?? { id: '', entryDate: today.value, content: '', secret: true, commentable: false } }
}

function readOrder(): CardKey[] {
  try { const stored = JSON.parse(localStorage.getItem('now-card-order') || '[]') as CardKey[]; const valid = stored.filter(key => DEFAULT_ORDER.includes(key)); return valid.length === DEFAULT_ORDER.length ? valid : DEFAULT_ORDER } catch { return DEFAULT_ORDER }
}
/** 卡片顺序可拖拽（规格书 5.8）：拖动卡片标题旁的手柄，经过另一张卡片时实时换位。 */
function cardStyle(key: CardKey) { return { order: cardOrder.value.indexOf(key) + 2 } }
function onCardDragStart(key: CardKey, event: DragEvent) { dragCard.value = key; event.dataTransfer?.setData('text/plain', `card:${key}`); if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move' }
function onCardDragOver(key: CardKey, event: DragEvent) {
  if (!dragCard.value || dragCard.value === key) return
  event.preventDefault()
  const from = cardOrder.value.indexOf(dragCard.value)
  const to = cardOrder.value.indexOf(key)
  if (from < 0 || to < 0 || from === to) return
  const next = [...cardOrder.value]
  next.splice(from, 1)
  next.splice(to, 0, dragCard.value)
  cardOrder.value = next
}
function onCardDragEnd() { dragCard.value = null; localStorage.setItem('now-card-order', JSON.stringify(cardOrder.value)) }

async function load() {
  try {
    const [todayData, tagData, planData, history, scheduleData] = await Promise.all([api.today(), api.moodTags(), api.plans(), api.history(addDaysISO(today.value, -6), today.value), api.scheduleAgenda()])
    data.value = normalize(todayData)
    if (authState.actor?.type === 'writer') { try { presets.value = (await api.presets()).items } catch { presets.value = [] } }
    tags.value = tagData.items
    plans.value = planData.items
    trend.value = history.points
    agenda.value = scheduleData
    if (todayData.bodies.length) bodyScore.value = todayData.bodies[todayData.bodies.length - 1].score
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
async function refreshToday() { try { data.value = normalize(await api.today()) } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' } }
async function refreshAgenda() { try { agenda.value = await api.scheduleAgenda() } catch (cause) { error.value = cause instanceof Error ? cause.message : '课表读取失败' } }
async function recordMood() {
  if (!selectedTags.value.length) return
  try { await api.addMood(selectedTags.value, moodNote.value.trim(), props.secret); selectedTags.value = []; moodNote.value = ''; await refreshToday(); editingMetric.value = null } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function recordBody() {
  try {
    await api.addBody(bodyScore.value, bodyNote.value.trim(), props.secret)
    bodyNote.value = ''
    const [todayData, history] = await Promise.all([api.today(), api.history(addDaysISO(today.value, -6), today.value)])
    data.value = normalize(todayData)
    trend.value = history.points
    editingMetric.value = null
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function deleteLastState(kind: 'mood' | 'body') {
  const label = kind === 'mood' ? '心情' : '身体'
  if (!window.confirm(`确认删除今天最后一条${label}记录吗？`)) return
  if (deletingMetric.value) return
  deletingMetric.value = true
  try {
    await api.deleteLastState(kind, props.secret)
    const [todayData, history] = await Promise.all([api.today(), api.history(addDaysISO(today.value, -6), today.value)])
    data.value = normalize(todayData)
    trend.value = history.points
    if (kind === 'body') bodyScore.value = todayData.bodies.at(-1)?.score ?? 70
    if (kind === 'mood') { selectedTags.value = []; moodNote.value = '' }
    editingMetric.value = null
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败' } finally { deletingMetric.value = false }
}
async function addTag() {
  if (!newTag.value.name.trim()) return
  try { await api.addMoodTag({ name: newTag.value.name.trim(), emoji: newTag.value.emoji.trim() || '🙂', value: newTag.value.value }); tags.value = (await api.moodTags()).items; newTag.value = { emoji: '🙂', name: '', value: 60 }; newTagOpen.value = false } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
}
async function toggleTask(task: Task) {
  const next = !task.done
  task.done = next
  try { await api.setTaskDone(task.id, next, task.taskDate) } catch (cause) { task.done = !next; error.value = cause instanceof Error ? cause.message : '更新失败' }
}
async function toggleTaskProgress(task: Task) { if (task.done || taskBusy.value) return; taskBusy.value = true; try { const updated = await api.setTaskInProgress(task.id, !task.inProgress, task.taskDate); const index = data.value.tasks.findIndex(item => item.id === task.id); if (index >= 0) data.value.tasks[index] = updated; if (selectedTask.value?.id === task.id) selectedTask.value = updated } catch (cause) { error.value = cause instanceof Error ? cause.message : '进行状态更新失败' } finally { taskBusy.value = false } }
function activeSeconds(task: Task) { const live = task.inProgress && task.inProgressSince ? Math.max(0, Math.floor((now.value.getTime() - new Date(task.inProgressSince).getTime()) / 1000)) : 0; return task.accumulatedActiveSeconds + live }
function activeDuration(task: Task) { const seconds = activeSeconds(task); if (seconds < 60) return '不足 1 分钟'; const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60); return hours ? `${hours}小时 ${String(minutes).padStart(2, '0')}分` : `${minutes}分钟` }
async function addTask() {
  const title = newTask.value.trim()
  if (!title) return
  try { await api.addTask(title, '', 'normal'); newTask.value = ''; await refreshToday() } catch (cause) { error.value = cause instanceof Error ? cause.message : '添加失败' }
}
function openTask(task: Task) {
  selectedTask.value = task
  taskEditing.value = true
  taskDraft.value = { title: task.title, description: task.description, priority: task.priority }
  taskAccessDraft.value = { presetId: task.presetId ?? '', secret: !!task.secret, commentable: !!task.commentable }
}
async function saveTaskAccess() {
  const task = selectedTask.value
  if (!task || taskBusy.value) return
  taskBusy.value = true
  try { const updated = await api.setTaskAccess(task.id, taskAccessDraft.value.presetId, taskAccessDraft.value.secret, taskAccessDraft.value.commentable); Object.assign(task, updated); selectedTask.value = updated } catch (cause) { error.value = cause instanceof Error ? cause.message : '权限保存失败' } finally { taskBusy.value = false }
}
async function addTaskMilestone() {
  const task = selectedTask.value
  if (!task || !milestoneDraft.value.description.trim() || taskBusy.value) return
  taskBusy.value = true
  try { await api.addMilestone({ target_type: 'task', target_id: task.id, description: milestoneDraft.value.description.trim(), detail: milestoneDraft.value.detail.trim(), preset_id: taskAccessDraft.value.presetId, secret: taskAccessDraft.value.secret }); milestoneDraft.value = { description: '', detail: '' } } catch (cause) { error.value = cause instanceof Error ? cause.message : '里程碑保存失败' } finally { taskBusy.value = false }
}
function closeTask() { selectedTask.value = null; taskEditing.value = false }
async function saveTask() {
  const task = selectedTask.value
  if (!task || !taskDraft.value.title.trim() || taskBusy.value) return
  taskBusy.value = true
  try {
    const updated = await api.updateTask(task.id, task.taskDate, taskDraft.value)
    const index = data.value.tasks.findIndex(item => item.id === task.id)
    if (index >= 0) data.value.tasks[index] = updated
    selectedTask.value = updated
    taskEditing.value = false
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { taskBusy.value = false }
}
async function removeTask() {
  const task = selectedTask.value
  if (!task || taskBusy.value) return
  taskBusy.value = true
  try { await api.deleteTask(task.id, task.taskDate); data.value.tasks = data.value.tasks.filter(item => item.id !== task.id); closeTask() } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败' } finally { taskBusy.value = false }
}
async function toggleTaskFromDrawer() {
  const task = selectedTask.value
  if (!task) return
  await toggleTask(task)
  selectedTask.value = task
}
function toggleTag(id: string) { selectedTags.value = selectedTags.value.includes(id) ? selectedTags.value.filter(item => item !== id) : [...selectedTags.value, id] }
function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer)
  const layerSecret = props.secret
  saveTimer = window.setTimeout(async () => {
    const content = (layerSecret ? data.value.secretDiary : data.value.diary)?.content ?? ''
    saving.value = true
    try {
      await api.saveDraft(content, layerSecret)
      const saved = await api.saveDiary(content, layerSecret)
      if (layerSecret && data.value.secretDiary) data.value.secretDiary.id = saved.id
      else data.value.diary.id = saved.id
      savedAt.value = timeLabel(beijingNow())
    } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { saving.value = false }
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
onMounted(() => { clock = window.setInterval(() => { now.value = beijingNow(); const nextDay = todayISO(); if (nextDay !== dayKey) { dayKey = nextDay; refreshToday(); refreshAgenda() } }, 1000); agendaTimer = window.setInterval(refreshAgenda, 30000); load() })
onBeforeUnmount(() => { if (clock) clearInterval(clock); if (agendaTimer) clearInterval(agendaTimer); if (saveTimer) clearTimeout(saveTimer) })
</script>

<template>
  <main class="page now-page">
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <section class="now-layout" :style="{ '--left-width': `${leftWidth}%` }">
      <div v-stagger class="now-left" :class="{ reordering: dragCard }">
        <section class="now-clock" aria-live="off" style="order: 0">
          <div class="clock-display">
            <time class="clock-time mono" :datetime="now.toISOString()">
              <span class="clock-main">{{ hhmm }}</span><Transition name="fade" mode="out-in"><small :key="seconds" class="clock-seconds">{{ seconds }}</small></Transition>
            </time>
            <div class="clock-meta">
              <Transition name="fade" mode="out-in"><strong :key="dateLabel" class="clock-date">{{ dateLabel }}</strong></Transition>
              <Transition name="fade" mode="out-in"><span :key="lunar" class="clock-lunar">{{ lunar }}</span></Transition>
              <span class="clock-week">{{ weekday }}</span>
            </div>
          </div>
          <i class="day-progress" :title="`今天已过去 ${Math.round(dayProgress)}%`" aria-hidden="true"><b :style="{ width: `${dayProgress}%` }" /></i>
        </section>

        <PlanCarousel v-if="activePlans.length" style="order: 1" :plans="activePlans" :interval="carouselInterval" @select="emit('navigate-future')" />

        <article class="card metrics-card" :class="{ 'drag-source': dragCard === 'metrics' }" :style="cardStyle('metrics')" @dragover="onCardDragOver('metrics', $event)" @drop.prevent="onCardDragEnd">
          <div class="metrics-grid">
            <section class="metric-panel">
              <header class="card-title metric-panel-head">
                <span class="card-title-main"><span class="card-grip" draggable="true" title="拖动调整卡片顺序" @dragstart="onCardDragStart('metrics', $event)" @dragend="onCardDragEnd"><AppIcon name="grip" :size="14" /></span>心情 <small>今天 {{ data.moods.length ? `${data.moods.length} 次` : '' }}</small></span>
                <button class="icon-button metric-edit-trigger" type="button" :aria-label="editingMetric === 'mood' ? '返回心情图表' : '添加心情'" @click="editingMetric = editingMetric === 'mood' ? null : 'mood'"><AppIcon :name="editingMetric === 'mood' ? 'close' : 'plus'" :size="16" /></button>
              </header>
              <Transition name="fade-slide" mode="out-in">
                <div v-if="editingMetric !== 'mood'" key="mood-chart" class="metric-chart"><MetricLine :points="moodPoints" :min="0" :max="100" :height="128" empty="今天还没有心情记录" /></div>
                <form v-else key="mood-editor" class="now-form metric-editor" @submit.prevent="recordMood">
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
                  <div class="metric-editor-actions">
                    <button v-if="data.moods.some(item => !!item.secret === props.secret)" class="text-button danger" type="button" :disabled="deletingMetric" @click="deleteLastState('mood')"><AppIcon name="trash" :size="14" />删除上次状态</button>
                    <button class="primary" type="submit" :disabled="!selectedTags.length">记录</button>
                  </div>
                </form>
              </Transition>
            </section>

            <section class="metric-panel">
              <header class="card-title metric-panel-head">
                <span class="card-title-main">身体 <small>今天 {{ data.bodies.length ? `${data.bodies.length} 次` : '' }}</small></span>
                <button class="icon-button metric-edit-trigger" type="button" :aria-label="editingMetric === 'body' ? '返回身体图表' : '添加身体记录'" @click="editingMetric = editingMetric === 'body' ? null : 'body'"><AppIcon :name="editingMetric === 'body' ? 'close' : 'plus'" :size="16" /></button>
              </header>
              <Transition name="fade-slide" mode="out-in">
                <div v-if="editingMetric !== 'body'" key="body-chart" class="metric-chart"><MetricLine :points="bodyPoints" :min="0" :max="100" :height="128" empty="这七天还没有身体记录" /></div>
                <form v-else key="body-editor" class="now-form metric-editor" @submit.prevent="recordBody">
                  <div class="score-row"><span class="faint">今天的状态</span><output class="score mono">{{ bodyScore }}</output></div>
                  <input v-model.number="bodyScore" type="range" min="0" max="100" aria-label="身体评分" :style="{ '--range-fill': `${bodyScore}%` }" />
                  <input v-model="bodyNote" placeholder="备注（可选）" maxlength="200" />
                  <div class="metric-editor-actions">
                    <button v-if="data.bodies.some(item => !!item.secret === props.secret)" class="text-button danger" type="button" :disabled="deletingMetric" @click="deleteLastState('body')"><AppIcon name="trash" :size="14" />删除上次状态</button>
                    <button class="primary" type="submit" :disabled="deletingMetric">记录</button>
                  </div>
                </form>
              </Transition>
            </section>
          </div>
        </article>

        <article class="card diary-card" :class="{ 'drag-source': dragCard === 'diary' }" :style="cardStyle('diary')" @dragover="onCardDragOver('diary', $event)" @drop.prevent="onCardDragEnd">
          <header class="card-head">
            <h2><span class="card-title-main"><span class="card-grip" draggable="true" title="拖动调整卡片顺序" @dragstart="onCardDragStart('diary', $event)" @dragend="onCardDragEnd"><AppIcon name="grip" :size="14" /></span>日记</span><span v-if="secret" class="secret-badge">绝密模式</span></h2>
            <Transition name="fade" mode="out-in"><small :key="savedAt + String(saving)" class="faint mono">{{ saving ? '' : savedAt ? `已保存 ${savedAt}` : '' }}</small></Transition>
          </header>
          <DiaryEditor :key="secret ? 'secret' : 'public'" :model-value="activeDiary.content" :editor-id="secret ? 'diary-editor-secret' : 'diary-editor'" :theme="theme" :vault-key="vaultKey" :secret="secret" :placeholder="secret ? '只有你自己能看到的内心 OS……' : '今天……'" @update:model-value="activeDiary.content = $event" @change="scheduleSave" />
        </article>
      </div>

      <div class="divider-v now-divider" :class="{ dragging }" role="separator" aria-orientation="vertical" aria-label="拖动调整左右栏宽度" @pointerdown="startResize" />

      <aside v-stagger class="now-right">
        <ScheduleNowCard :agenda="agenda" :now="now" />
        <section class="task-head">
          <h2 class="card-title">待办<small>{{ Math.round(pendingDisplay) }} 项未完成</small></h2>
          <form class="task-add" @submit.prevent="addTask"><input v-model="newTask" placeholder="添加今天的待办，回车保存" maxlength="120" aria-label="新待办" /><button class="icon-button" type="submit" aria-label="添加" :disabled="!newTask.trim()"><AppIcon name="plus" /></button></form>
        </section>
        <section class="card tasks-card">
          <TransitionGroup v-if="sortedTasks.length" name="list" tag="ul" class="today-tasks">
            <li v-for="task in sortedTasks" :key="task.id" :class="{ done: task.done, 'in-progress': task.inProgress, [`priority-${task.priority}`]: true }">
              <div class="task-row" role="button" tabindex="0" @click="openTask(task)" @keydown.enter="openTask(task)">
                <input type="checkbox" :checked="task.done" :aria-label="`${task.title}完成状态`" @click.stop @change="toggleTask(task)" />
                <span class="task-body">
                  <span class="task-title" :class="{ done: task.done }">{{ task.title }}</span>
                  <small class="task-meta faint"><span>{{ task.taskDate }}</span><span>{{ task.priority === 'high' ? '高优先级' : task.priority === 'low' ? '低优先级' : '普通' }}</span><span v-if="task.inProgress || task.accumulatedActiveSeconds">已进行 {{ activeDuration(task) }}</span></small>
                </span>
                <button v-if="!task.done" class="task-progress-button" :class="{ active: task.inProgress }" type="button" :aria-label="task.inProgress ? `停止${task.title}的进行计时` : `开始${task.title}的进行计时`" @click.stop="toggleTaskProgress(task)"><AppIcon name="target" :size="17" /></button>
                <AppIcon name="chevron-right" :size="15" class="task-open-icon" />
              </div>
            </li>
          </TransitionGroup>
          <EmptyState v-else icon="check" text="今天还没有待办" />
        </section>
        <Transition name="drawer">
          <div v-if="selectedTask" class="task-drawer-layer" @click.self="closeTask">
            <aside class="task-drawer" role="dialog" aria-modal="true" aria-label="任务详情">
              <header class="drawer-head">
                <div><small class="faint mono">{{ selectedTask.taskDate }}</small><h2>任务详情</h2></div>
                <div class="drawer-actions"><button class="icon-button" :aria-label="taskEditing ? '退出编辑' : '编辑任务'" @click="taskEditing = !taskEditing"><AppIcon :name="taskEditing ? 'close' : 'edit'" /></button><button class="icon-button" aria-label="关闭任务详情" @click="closeTask"><AppIcon name="close" /></button></div>
              </header>
              <div class="drawer-content">
                <div class="task-drawer-status">
                  <span class="task-priority-label" :class="`priority-${selectedTask.priority}`">{{ selectedTask.priority === 'high' ? '高优先级' : selectedTask.priority === 'low' ? '低优先级' : '普通优先级' }}</span>
                  <span v-if="selectedTask.inProgress || selectedTask.accumulatedActiveSeconds" class="faint mono">已进行 {{ activeDuration(selectedTask) }}</span>
                  <button v-if="!selectedTask.done" class="text-button" @click="toggleTaskProgress(selectedTask)"><AppIcon name="target" :size="14" />{{ selectedTask.inProgress ? '停止计时' : '开始计时' }}</button>
                  <button class="text-button" @click="toggleTaskFromDrawer"><AppIcon :name="selectedTask.done ? 'repeat' : 'check'" :size="14" />{{ selectedTask.done ? '标记未完成' : '标记完成' }}</button>
                </div>
                <template v-if="taskEditing">
                  <input v-model="taskDraft.title" class="drawer-title-input" maxlength="120" aria-label="任务标题" />
                  <select v-model="taskDraft.priority" aria-label="任务优先级"><option value="high">高优先级</option><option value="normal">普通优先级</option><option value="low">低优先级</option></select>
                  <DiaryEditor :model-value="taskDraft.description" editor-id="task-detail-editor" :theme="theme" :vault-key="`${vaultKey}:task:${selectedTask.id}`" placeholder="任务详细描述（支持 Markdown）" @update:model-value="taskDraft.description = $event" />
                  <button class="primary" :disabled="taskBusy || !taskDraft.title.trim()" @click="saveTask">保存任务</button>
                </template>
                <template v-else>
                  <h1 class="drawer-title">{{ selectedTask.title }}</h1>
                  <MarkdownPreview :model-value="selectedTask.description" editor-id="task-detail-preview" :theme="theme" :theme-class="themeClass" />
                  <EmptyState v-if="!selectedTask.description" icon="book" text="没有详细描述" compact />
                </template>
                <section v-if="authState.actor?.type === 'writer'" class="drawer-settings">
                  <header class="drawer-section-head"><b>权限与互动</b><small class="faint">附件默认继承任务权限</small></header>
                  <label class="field"><span>权限预设</span><select v-model="taskAccessDraft.presetId"><option value="">锚点范围内公开</option><option v-for="preset in presets" :key="preset.id" :value="preset.id">{{ preset.name }}</option></select></label>
                  <label class="check-field"><input v-model="taskAccessDraft.secret" type="checkbox" />绝密（仅书写者可见）</label>
                  <label class="check-field"><input v-model="taskAccessDraft.commentable" type="checkbox" />允许评论</label>
                  <button class="text-button" :disabled="taskBusy" @click="saveTaskAccess"><AppIcon name="shield" :size="14" />保存权限设置</button>
                  <div class="milestone-form"><b>标记里程碑</b><input v-model="milestoneDraft.description" placeholder="里程碑描述" maxlength="120" /><textarea v-model="milestoneDraft.detail" placeholder="详细信息（可选）" maxlength="500" rows="2" /><button class="text-button" :disabled="taskBusy || !milestoneDraft.description.trim()" @click="addTaskMilestone"><AppIcon name="medal" :size="14" />添加里程碑</button></div>
                </section>
                <div class="drawer-footer"><button class="text-button danger" :disabled="taskBusy" @click="removeTask"><AppIcon name="trash" :size="14" />删除任务</button><span class="faint">附件与权限设置沿用内容权限规则</span></div>
              </div>
            </aside>
          </div>
        </Transition>
      </aside>
    </section>
  </main>
</template>
