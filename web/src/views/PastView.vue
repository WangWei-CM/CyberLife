<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import LifeAxis from '../components/LifeAxis.vue'
import MetricLine, { type MetricPoint } from '../components/MetricLine.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import { api, type Comment, type HistoryDay, type Milestone, type TrendPoint } from '../api/client'
import { isWriter, lifeStartISO } from '../stores/auth'
import { addDaysISO, dayIndex, diffDays, fullDateLabel, monthDayLabel, relativeDayLabel, timeLabel, todayISO, weekdayLabel, parseISO } from '../lib/dates'

const props = withDefaults(defineProps<{ secret?: boolean }>(), { secret: false })
const today = todayISO()
const selected = ref(today)
const dock = ref<'left' | 'right'>(localStorage.getItem('past-axis-dock') === 'right' ? 'right' : 'left')
function toggleDock() { dock.value = dock.value === 'left' ? 'right' : 'left'; localStorage.setItem('past-axis-dock', dock.value) }
const visible = ref<{ from: string; to: string }>({ from: addDaysISO(today, -13), to: today })
const days = ref(new Map<string, HistoryDay>())
const points = ref(new Map<string, TrendPoint>())
const fetched = ref(new Set<string>())
const loading = ref(0)
const error = ref('')
const milestones = ref<Milestone[]>([])
const comments = ref<Comment[]>([])
const comment = ref('')
const theme = computed(() => 'dark' as const)

const selectedDay = computed(() => days.value.get(selected.value))
/** 绝密模式下书写者看到的是当天的绝密层日记；其余情况看公开层。 */
const shownDiary = computed(() => {
  const day = selectedDay.value
  if (!day) return undefined
  return props.secret && day.secretDiary?.id ? day.secretDiary : day.diary
})
const milestoneDates = computed(() => [...days.value.values()].filter(day => day.milestoneCount > 0).map(day => day.date))
const visibleDates = computed(() => { const list: string[] = []; for (let date = visible.value.from; date <= visible.value.to; date = addDaysISO(date, 1)) list.push(date); return list })
const moodPoints = computed<MetricPoint[]>(() => visibleDates.value.map(date => ({ x: dayIndex(date), y: points.value.get(date)?.mood ?? null, label: monthDayLabel(date) })))
const bodyPoints = computed<MetricPoint[]>(() => visibleDates.value.map(date => ({ x: dayIndex(date), y: points.value.get(date)?.body ?? null, label: monthDayLabel(date) })))
const selectedLabel = computed(() => fullDateLabel(selected.value))
const selectedWeekday = computed(() => weekdayLabel(parseISO(selected.value)))
const chapterYear = computed(() => selected.value.slice(0, 4))
const diaryMilestones = computed(() => milestones.value.filter(item => item.targetType === 'diary'))

/** 按需拉取可见范围内尚未缓存的日期，单次请求 ≤ 366 天。 */
async function ensure(from: string, to: string) {
  const missing: string[] = []
  for (let date = from; date <= to; date = addDaysISO(date, 1)) if (!fetched.value.has(date)) missing.push(date)
  if (!missing.length) return
  const spans: { from: string; to: string }[] = []
  for (const date of missing) {
    const last = spans[spans.length - 1]
    if (last && diffDays(last.to, date) === 1 && diffDays(last.from, date) < 365) last.to = date
    else spans.push({ from: date, to: date })
  }
  loading.value++
  try {
    await Promise.all(spans.map(async span => {
      const range = await api.history(span.from, span.to)
      const nextDays = new Map(days.value)
      const nextPoints = new Map(points.value)
      const nextFetched = new Set(fetched.value)
      for (const day of range.days) { nextDays.set(day.date, day); nextFetched.add(day.date) }
      for (const point of range.points) nextPoints.set(point.date, point)
      days.value = nextDays; points.value = nextPoints; fetched.value = nextFetched
    }))
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' } finally { loading.value-- }
}
function onRange(from: string, to: string) { visible.value = { from, to }; ensure(from, to) }
async function loadDetails(day: HistoryDay | undefined) {
  milestones.value = []
  comments.value = []
  if (!day?.diary.id) return
  try {
    const [milestoneResult, commentResult] = await Promise.all([api.milestones('diary', day.diary.id), api.comments('diary', day.diary.id)])
    milestones.value = milestoneResult.items
    comments.value = commentResult.items
  } catch { /* 无权限或无数据时保持空 */ }
}
async function submitComment() {
  const day = selectedDay.value
  const text = comment.value.trim()
  if (!day?.diary.id || !text) return
  try { await api.addComment('diary', day.diary.id, text); comment.value = ''; comments.value = (await api.comments('diary', day.diary.id)).items } catch (cause) { error.value = cause instanceof Error ? cause.message : '评论失败' }
}
watch(selectedDay, loadDetails, { immediate: true })
watch(selected, () => { ensure(selected.value, selected.value) })
</script>

<template>
  <main class="page past-page">
    <div class="past-dust" aria-hidden="true"><i v-for="n in 7" :key="n" /></div>
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <section class="past-layout" :class="`dock-${dock}`">
      <LifeAxis v-model:selected="selected" :life-start="lifeStartISO || undefined" :today="today" :milestones="milestoneDates" :mirror="dock === 'right'" @range="onRange" @toggle-dock="toggleDock" />
      <section v-stagger class="past-content" :class="{ loading: loading > 0 }">
        <header class="past-head">
          <div class="chapter" :data-year="chapterYear"><span class="chapter-year">{{ chapterYear }}</span><h1 class="chapter-title">{{ selectedLabel.replace(/^\d+年/, '') }}<small>{{ selectedWeekday }} · {{ relativeDayLabel(selected) }}</small></h1></div>
          <span class="past-range mono">{{ monthDayLabel(visible.from) }} — {{ monthDayLabel(visible.to) }}</span>
        </header>
        <div class="past-trends">
          <article class="card past-card"><h2 class="card-title">心情</h2><MetricLine :points="moodPoints" :min="0" :max="100" :height="140" empty="这段时间没有心情记录" /></article>
          <article class="card past-card"><h2 class="card-title">身体</h2><MetricLine :points="bodyPoints" :min="0" :max="100" :height="140" empty="这段时间没有身体记录" /></article>
        </div>
        <div class="past-day">
          <article class="card past-card diary-render">
            <Transition name="fade-slide" mode="out-in">
              <div :key="selected" class="past-diary">
                <section v-if="diaryMilestones.length" class="monument">
                  <span class="milestone-badge"><AppIcon name="medal" :size="14" />里程碑</span>
                  <h2 v-for="item in diaryMilestones" :key="item.id" class="monument-title">{{ item.description }}<small v-if="item.detail">{{ item.detail }}</small></h2>
                </section>
                <header class="card-head"><h2>日记</h2><small v-if="shownDiary?.secret" class="secret-badge">绝密层</small><small v-else-if="props.secret && selectedDay?.diary.id" class="faint">这一天没有绝密层，显示公开层</small></header>
                <MdPreview v-if="shownDiary?.id" :editor-id="`past-${selected}-${shownDiary.secret ? 's' : 'p'}`" :model-value="shownDiary.content" :theme="theme" language="zh-CN" preview-theme="default" :no-img-zoom-in="true" />
                <EmptyState v-else icon="book" text="这一天没有可查看的日记" />
                <section v-if="selectedDay?.diary.id && !shownDiary?.secret && (comments.length || selectedDay.diary.commentable)" class="past-comments">
                  <h3 class="card-title">评论<small v-if="comments.length">{{ comments.length }}</small></h3>
                  <TransitionGroup name="list" tag="ul" class="comment-list">
                    <li v-for="item in comments" :key="item.id"><p>{{ item.content }}</p><small class="mono faint">{{ timeLabel(item.createdAt) }}</small></li>
                  </TransitionGroup>
                  <form v-if="selectedDay.diary.commentable || isWriter" class="comment-form" @submit.prevent="submitComment"><input v-model="comment" placeholder="写下评论" maxlength="500" /><button class="icon-button" type="submit" aria-label="发送" :disabled="!comment.trim()"><AppIcon name="send" :size="16" /></button></form>
                </section>
              </div>
            </Transition>
          </article>
          <article class="card past-card">
            <header class="card-head"><h2>日程</h2><small v-if="selectedDay?.tasks.length" class="faint">{{ selectedDay.tasks.filter(task => task.done).length }} / {{ selectedDay.tasks.length }} 完成</small></header>
            <Transition name="fade-slide" mode="out-in">
              <ul v-if="selectedDay?.tasks.length" :key="selected" class="past-tasks">
                <li v-for="task in selectedDay.tasks" :key="task.id" :class="{ done: task.done }">
                  <i class="task-state"><AppIcon v-if="task.done" name="check" :size="12" :stroke-width="2.5" /></i>
                  <div><span class="task-title" :class="{ done: task.done }">{{ task.title }}</span><small v-if="task.description" class="faint">{{ task.description }}</small></div>
                </li>
              </ul>
              <EmptyState v-else :key="`empty-${selected}`" icon="calendar" text="这一天没有可查看的日程" />
            </Transition>
          </article>
        </div>
      </section>
    </section>
  </main>
</template>
