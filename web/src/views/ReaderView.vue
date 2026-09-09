<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import MarkdownPreview from '../components/MarkdownPreview.vue'
import { api, type Comment, type Milestone, type NowData, type ScheduleAgenda, type Task } from '../api/client'
import { beijingNow, fullDateLabel, lunarLabel, timeLabel, weekdayLabel } from '../lib/dates'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import ScheduleNowCard from '../components/ScheduleNowCard.vue'

const data = ref<NowData>({ diary: { id: '', entryDate: '', content: '', secret: false, commentable: false }, moods: [], bodies: [], tasks: [] })
const comments = ref<Comment[]>([])
const milestones = ref<Milestone[]>([])
const agenda = ref<ScheduleAgenda | null>(null)
const error = ref('')
const comment = ref('')
const now = ref(beijingNow())
let clock: number | undefined

const dateLabel = computed(() => fullDateLabel(now.value))
const weekday = computed(() => weekdayLabel(now.value))
const lunar = computed(() => lunarLabel(now.value))
const hhmm = computed(() => timeLabel(now.value))
const seconds = computed(() => String(now.value.getSeconds()).padStart(2, '0'))
const dayProgress = computed(() => ((now.value.getHours() * 60 + now.value.getMinutes()) / 1440) * 100)
const theme = computed(() => (document.querySelector('.app-shell')?.classList.contains('light') ? 'light' : 'dark') as 'light' | 'dark')
const activeTasks = computed(() => data.value.tasks.filter(task => !task.done && task.inProgress))
const todoTasks = computed(() => data.value.tasks.filter(task => !task.inProgress))

function activeSeconds(task: Task) {
  const live = task.inProgress && task.inProgressSince ? Math.max(0, Math.floor((now.value.getTime() - new Date(task.inProgressSince).getTime()) / 1000)) : 0
  return task.accumulatedActiveSeconds + live
}
function activeDuration(task: Task) {
  const seconds = activeSeconds(task)
  if (seconds < 60) return '不足 1 分钟'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours ? `${hours}小时 ${String(minutes).padStart(2, '0')}分` : `${minutes}分钟`
}

async function load() {
  try {
    const [todayData, scheduleData] = await Promise.all([api.visibleToday(), api.scheduleAgenda().catch(() => null)])
    data.value = todayData
    agenda.value = scheduleData
    if (data.value.diary.id) {
      const [commentResult, milestoneResult] = await Promise.all([api.comments('diary', data.value.diary.id), api.milestones('diary', data.value.diary.id)])
      comments.value = commentResult.items
      milestones.value = milestoneResult.items
    } else { comments.value = []; milestones.value = [] }
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '无法读取内容' }
}
async function addComment() {
  const text = comment.value.trim()
  if (!data.value.diary.id || !text) return
  try { await api.addComment('diary', data.value.diary.id, text); comment.value = ''; comments.value = (await api.comments('diary', data.value.diary.id)).items } catch (cause) { error.value = cause instanceof Error ? cause.message : '评论失败' }
}
onMounted(() => { load(); clock = window.setInterval(() => { now.value = beijingNow() }, 1000) })
onBeforeUnmount(() => { if (clock) clearInterval(clock) })
</script>

<template>
  <main class="page now-page reader-writer-page">
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <section class="now-layout reader-writer-layout">
      <div v-stagger class="now-left reader-writer-left">
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

        <article class="card metrics-card reader-metrics-card" style="order: 1">
          <div class="metrics-grid">
            <section class="metric-panel">
              <header class="card-title metric-panel-head"><span class="card-title-main">心情 <small>今天 {{ data.moods.length ? `${data.moods.length} 次` : '' }}</small></span></header>
              <ul v-if="data.moods.length" class="reader-list reader-metric-list">
                <li v-for="item in data.moods" :key="item.id"><span class="emoji">{{ item.tags.map(tag => tag.emoji).join(' ') }}</span><span>{{ item.tags.map(tag => tag.name).join('、') }}<small v-if="item.note" class="faint"> · {{ item.note }}</small></span><span class="when">{{ timeLabel(item.recordedAt) }}</span></li>
              </ul>
              <EmptyState v-else icon="smile" text="今天还没有心情记录" compact />
            </section>
            <section class="metric-panel">
              <header class="card-title metric-panel-head"><span class="card-title-main">身体 <small>今天 {{ data.bodies.length ? `${data.bodies.length} 次` : '' }}</small></span></header>
              <ul v-if="data.bodies.length" class="reader-list reader-metric-list">
                <li v-for="item in data.bodies" :key="item.id"><span class="reader-score">{{ item.score }}</span><span>{{ item.note || '' }}</span><span class="when">{{ timeLabel(item.recordedAt) }}</span></li>
              </ul>
              <EmptyState v-else icon="pulse" text="今天还没有身体记录" compact />
            </section>
          </div>
        </article>

        <article class="card diary-card reader-diary writer-readonly-diary">
          <header class="card-head">
            <h2><span class="card-title-main">日记</span></h2>
            <small class="faint mono">{{ data.diary.entryDate }}</small>
          </header>
          <div v-if="data.diary.id" class="writer-readonly-preview">
            <MarkdownPreview editor-id="reader-diary" :model-value="data.diary.content" :theme="theme" theme-class="theme-now" />
          </div>
          <EmptyState v-else icon="book" text="今天没有你可以查看的日记" />
          <section v-if="data.diary.id && (comments.length || data.diary.commentable)" class="past-comments">
            <h3 class="card-title">评论<small v-if="comments.length">{{ comments.length }}</small></h3>
            <ul class="comment-list"><li v-for="item in comments" :key="item.id"><p>{{ item.content }}</p><small class="mono faint">{{ timeLabel(item.createdAt) }}</small></li></ul>
            <form v-if="data.diary.commentable" class="comment-form" @submit.prevent="addComment"><input v-model="comment" placeholder="写下评论" maxlength="500" /><button class="icon-button" type="submit" aria-label="发送" :disabled="!comment.trim()"><AppIcon name="send" :size="16" /></button></form>
          </section>
        </article>
      </div>

      <div class="divider-v now-divider" role="separator" aria-orientation="vertical" aria-label="左右栏分隔" />

      <aside v-stagger class="now-right reader-writer-right">
        <ScheduleNowCard :agenda="agenda" :now="now" />
        <section v-if="activeTasks.length" class="task-progress-section">
          <header class="task-head task-progress-head"><h2 class="card-title">进行中<small>{{ activeTasks.length }} 项正在进行</small></h2></header>
          <section class="card tasks-card task-progress-card reader-tasks-card">
            <ul class="today-tasks">
              <li v-for="task in activeTasks" :key="`active-${task.id}`" class="in-progress" :class="{ [`priority-${task.priority}`]: true }">
                <div class="task-row reader-task-row">
                  <input type="checkbox" :checked="task.done" disabled :aria-label="`${task.title}完成状态`" />
                  <span class="task-body"><span class="task-title">{{ task.title }}</span><small class="task-meta faint"><span>{{ task.taskDate }}</span><span>已进行 {{ activeDuration(task) }}</span></small></span>
                </div>
              </li>
            </ul>
          </section>
        </section>
        <section v-if="milestones.length" class="card reader-milestones-card">
          <header class="card-title"><span class="card-title-main">里程碑</span><small>{{ milestones.length }} 项</small></header>
          <ul class="reader-list">
            <li v-for="item in milestones" :key="item.id"><span class="milestone-badge"><AppIcon name="medal" :size="14" /></span><span><b>{{ item.description }}</b><small v-if="item.detail" class="faint">{{ item.detail }}</small></span></li>
          </ul>
        </section>
        <section class="task-head"><h2 class="card-title">待办<small>{{ todoTasks.filter(task => !task.done).length }} 项未完成</small></h2></section>
        <section class="card tasks-card reader-tasks-card">
          <ul v-if="todoTasks.length" class="today-tasks">
            <li v-for="task in todoTasks" :key="task.id" :class="{ done: task.done, [`priority-${task.priority}`]: true }">
              <div class="task-row reader-task-row">
                <input type="checkbox" :checked="task.done" disabled :aria-label="`${task.title}完成状态`" />
                <span class="task-body"><span class="task-title" :class="{ done: task.done }">{{ task.title }}</span><small v-if="task.description" class="task-meta faint">{{ task.description }}</small></span>
                <span class="task-meta faint">{{ task.taskDate }}</span>
              </div>
            </li>
          </ul>
          <EmptyState v-else icon="check" text="今天还没有待办" />
        </section>
      </aside>
    </section>
  </main>
</template>

<style scoped>
.reader-writer-page .reader-metrics-card { order: 1; }
.reader-metric-list { max-height: 150px; overflow: auto; scrollbar-width: thin; }
.reader-metric-list li { min-width: 0; }
.reader-metric-list li > span:nth-child(2) { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.writer-readonly-diary { order: 2; }
.writer-readonly-preview { min-height: 0; overflow: auto; scrollbar-width: thin; }
.writer-readonly-preview :deep(.md-editor) { min-height: 0; border: 0 !important; background: transparent !important; }
.writer-readonly-preview :deep(.md-editor-preview-wrapper) { padding: 0 !important; }
.writer-readonly-diary > .empty-state { min-height: 180px; }
.reader-milestones-card .reader-list li { align-items: flex-start; }
.reader-milestones-card .reader-list li > span:last-child { display: grid; gap: 3px; min-width: 0; }
.reader-milestones-card .reader-list li small { display: block; }
.reader-task-row { cursor: default; }
.reader-task-row input[type="checkbox"] { cursor: default; }

@media (min-width: 901px) {
  .reader-writer-page .writer-readonly-diary { min-height: 0; }
  .reader-writer-page .writer-readonly-diary > .writer-readonly-preview { height: 100%; }
}
</style>
