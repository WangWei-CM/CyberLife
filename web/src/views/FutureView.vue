<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import ZoomCalendar from '../components/ZoomCalendar.vue'
import ProgressBar from '../components/ProgressBar.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import { api, type FutureTask, type Plan } from '../api/client'
import { isWriter } from '../stores/auth'
import { useCountUp } from '../lib/motion'
import { addDaysISO, diffDays, monthDayLabel, relativeDayLabel, todayISO } from '../lib/dates'

const today = todayISO()
const plans = ref<Plan[]>([])
const tasks = ref(new Map<string, FutureTask>())
const fetched = ref(new Set<string>())
const selectedPlanId = ref('')
const selectedDate = ref(today)
const editMode = ref(false)
const progress = ref(0)
const progressDate = ref(today)
const edit = ref({ name: '', startDate: today, endDate: today, intro: '' })
const creating = ref(false)
const form = ref({ name: '', startDate: today, endDate: addDaysISO(today, 30), intro: '' })
const newTask = ref('')
const error = ref('')
const listWidth = ref(Number(localStorage.getItem('future-list-width') || 30))
const topHeight = ref(Number(localStorage.getItem('future-top-height') || 420))
const dragging = ref<'col' | 'row' | null>(null)
const busy = ref(false)

const sortedPlans = computed(() => [...plans.value].sort((a, b) => a.endDate.localeCompare(b.endDate)))
const selectedPlan = computed(() => plans.value.find(plan => plan.id === selectedPlanId.value) ?? null)
const taskList = computed(() => [...tasks.value.values()])
const dayTasks = computed(() => taskList.value.filter(task => task.date === selectedDate.value).sort((a, b) => Number(a.done) - Number(b.done)))
const dayPlans = computed(() => plans.value.filter(plan => plan.startDate <= selectedDate.value && selectedDate.value <= plan.endDate))
const timeDisplay = useCountUp(() => selectedPlan.value?.timeProgress ?? 0)
const planDisplay = useCountUp(() => selectedPlan.value?.progress ?? 0)
const remainingDays = computed(() => selectedPlan.value ? diffDays(today, selectedPlan.value.endDate) : 0)
const isOngoing = (plan: Plan) => plan.startDate <= today && today <= plan.endDate && plan.progress < 100

async function loadPlans() { try { plans.value = (await api.plans()).items; if (!selectedPlanId.value && sortedPlans.value.length) choose(sortedPlans.value.find(isOngoing) ?? sortedPlans.value[0]) } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' } }
/** 任务按需分块拉取（单次 ≤ 366 天），只覆盖今天前后两年，更远处只显示规划横条。 */
async function ensureTasks(from: string, to: string) {
  const floor = addDaysISO(today, -365)
  const ceiling = addDaysISO(today, 730)
  const start = from < floor ? floor : from
  const end = to > ceiling ? ceiling : to
  if (start > end) return
  const spans: { from: string; to: string }[] = []
  for (let date = start; date <= end; date = addDaysISO(date, 1)) {
    if (fetched.value.has(date)) continue
    const last = spans[spans.length - 1]
    if (last && diffDays(last.to, date) === 1 && diffDays(last.from, date) < 365) last.to = date
    else spans.push({ from: date, to: date })
  }
  if (!spans.length) return
  try {
    await Promise.all(spans.map(async span => {
      const items = (await api.calendar(span.from, span.to)).items
      const next = new Map(tasks.value)
      for (const item of items) next.set(item.id, item)
      tasks.value = next
      const marks = new Set(fetched.value)
      for (let date = span.from; date <= span.to; date = addDaysISO(date, 1)) marks.add(date)
      fetched.value = marks
    }))
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
function onRange(from: string, to: string) { ensureTasks(from, to) }
function choose(plan: Plan) { selectedPlanId.value = plan.id; progress.value = plan.progress; progressDate.value = today; editMode.value = false; edit.value = { name: plan.name, startDate: plan.startDate, endDate: plan.endDate, intro: plan.intro } }
function choosePlanById(id: string) { const plan = plans.value.find(item => item.id === id); if (plan) choose(plan) }
async function createPlan() {
  if (!form.value.name.trim() || busy.value) return
  busy.value = true
  try { const plan = await api.createPlan({ name: form.value.name.trim(), startDate: form.value.startDate, endDate: form.value.endDate, intro: form.value.intro }); form.value = { name: '', startDate: today, endDate: addDaysISO(today, 30), intro: '' }; creating.value = false; await loadPlans(); choose(plan) } catch (cause) { error.value = cause instanceof Error ? cause.message : '创建失败' } finally { busy.value = false }
}
/** 编辑模式：保存名称/起止/简介，以及（若有变化）当次标记的完成百分比。 */
async function savePlan() {
  const plan = selectedPlan.value
  if (!plan || busy.value || !edit.value.name.trim()) return
  busy.value = true
  try {
    await api.updatePlan(plan.id, { name: edit.value.name.trim(), startDate: edit.value.startDate, endDate: edit.value.endDate, intro: edit.value.intro })
    if (progress.value !== plan.progress) await api.setPlanProgress(plan.id, progressDate.value, progress.value)
    await loadPlans()
    const refreshed = plans.value.find(item => item.id === plan.id)
    if (refreshed) edit.value = { name: refreshed.name, startDate: refreshed.startDate, endDate: refreshed.endDate, intro: refreshed.intro }
    editMode.value = false
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { busy.value = false }
}
async function addTask() {
  const title = newTask.value.trim()
  if (!title) return
  try { const task = await api.addTask(title, '', 'normal', selectedDate.value); newTask.value = ''; tasks.value = new Map(tasks.value).set(task.id, { id: task.id, date: task.taskDate, title: task.title, priority: task.priority, done: task.done }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '添加失败' }
}
async function toggleTask(task: FutureTask) {
  const next = !task.done
  tasks.value = new Map(tasks.value).set(task.id, { ...task, done: next })
  try { await api.setTaskDone(task.id, next, task.date) } catch (cause) { tasks.value = new Map(tasks.value).set(task.id, { ...task, done: !next }); error.value = cause instanceof Error ? cause.message : '更新失败' }
}
function startResize(kind: 'col' | 'row', event: PointerEvent) {
  dragging.value = kind
  document.body.classList.add(kind === 'col' ? 'resizing-col' : 'resizing-row')
  const container = (event.currentTarget as HTMLElement).parentElement!.getBoundingClientRect()
  const move = (next: PointerEvent) => {
    if (kind === 'col') listWidth.value = Math.max(22, Math.min(50, ((next.clientX - container.left) / container.width) * 100))
    else topHeight.value = Math.max(260, Math.min(720, next.clientY - container.top))
  }
  const up = () => {
    dragging.value = null
    document.body.classList.remove('resizing-col', 'resizing-row')
    localStorage.setItem('future-list-width', String(listWidth.value))
    localStorage.setItem('future-top-height', String(topHeight.value))
    window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move); window.addEventListener('pointerup', up)
  event.preventDefault()
}
watch(selectedPlan, plan => { if (plan) progress.value = plan.progress })
onMounted(() => { loadPlans(); ensureTasks(addDaysISO(today, -7), addDaysISO(today, 60)) })
</script>

<template>
  <main class="page future-page">
    <div class="scanlines" aria-hidden="true" />
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <template v-if="isWriter">
      <section class="future-top" :style="{ '--list-width': `${listWidth}%`, height: `${topHeight}px` }">
        <aside v-stagger class="future-list cyber-panel bracket">
          <header class="cyber-head"><span class="cyber-heading"><AppIcon name="target" :size="14" />规划</span><small class="mono faint">{{ sortedPlans.filter(isOngoing).length }} 进行中 / {{ plans.length }}</small><button class="text-button" :aria-expanded="creating" @click="creating = !creating"><AppIcon :name="creating ? 'close' : 'plus'" :size="14" />{{ creating ? '取消' : '新建' }}</button></header>
          <Transition name="fade-slide">
            <form v-if="creating" class="plan-form" @submit.prevent="createPlan">
              <input v-model="form.name" placeholder="规划名称" maxlength="60" required />
              <div class="form-row"><input v-model="form.startDate" type="date" required aria-label="开始日期" /><input v-model="form.endDate" type="date" required aria-label="截止日期" /></div>
              <textarea v-model="form.intro" placeholder="简介（支持 Markdown）" rows="3" />
              <button class="primary" type="submit" :disabled="busy || !form.name.trim()">创建规划</button>
            </form>
          </Transition>
          <TransitionGroup v-if="sortedPlans.length" name="list" tag="ul" class="plan-list">
            <li v-for="plan in sortedPlans" :key="plan.id">
              <button v-glow class="plan-card" :class="{ active: plan.id === selectedPlanId, ongoing: isOngoing(plan), done: plan.progress >= 100 }" @click="choose(plan)">
                <span class="plan-card-inner">
                  <b class="plan-name">{{ plan.name }}</b>
                  <small class="plan-dates mono">{{ monthDayLabel(plan.startDate) }} → {{ monthDayLabel(plan.endDate) }}<em>{{ relativeDayLabel(plan.endDate) }}</em></small>
                  <ProgressBar :value="plan.timeProgress" :height="3" />
                  <ProgressBar :value="plan.progress" tone="accent-2" :height="3" />
                </span>
              </button>
            </li>
          </TransitionGroup>
          <EmptyState v-else icon="target" text="还没有规划" />
        </aside>
        <div class="divider-v" :class="{ dragging: dragging === 'col' }" role="separator" aria-orientation="vertical" @pointerdown="startResize('col', $event)" />
        <section class="future-detail cyber-panel glass bracket">
          <Transition name="fade-slide" mode="out-in">
            <div v-if="selectedPlan" :key="selectedPlan.id" class="detail-body">
              <header class="detail-head">
                <div>
                  <h1 class="glitch-text detail-title" :data-text="selectedPlan.name">{{ selectedPlan.name }}</h1>
                  <p class="detail-dates mono">{{ selectedPlan.startDate }} — {{ selectedPlan.endDate }}<span>{{ remainingDays >= 0 ? `剩余 ${remainingDays} 天` : `已过 ${-remainingDays} 天` }}</span></p>
                </div>
                <button class="text-button" :aria-pressed="editMode" @click="editMode = !editMode"><AppIcon :name="editMode ? 'close' : 'edit'" :size="14" />{{ editMode ? '退出编辑' : '编辑模式' }}</button>
              </header>
              <div class="dual-progress">
                <div class="dual-row"><span class="cyber-heading">时间进度</span><ProgressBar :value="selectedPlan.timeProgress" :height="6" /><b class="mono">{{ Math.round(timeDisplay) }}%</b></div>
                <div class="dual-row"><span class="cyber-heading magenta">计划进度</span><ProgressBar :value="selectedPlan.progress" tone="accent-2" :height="6" /><b class="mono">{{ Math.round(planDisplay) }}%</b></div>
              </div>
              <Transition name="fade-slide" mode="out-in">
                <form v-if="editMode" class="progress-form" @submit.prevent="savePlan">
                  <input v-model="edit.name" placeholder="规划名称" maxlength="60" required aria-label="规划名称" />
                  <div class="form-row"><input v-model="edit.startDate" type="date" required aria-label="开始日期" /><input v-model="edit.endDate" type="date" required aria-label="截止日期" /></div>
                  <textarea v-model="edit.intro" rows="6" placeholder="简介（支持 Markdown）" aria-label="简介" />
                  <label class="field"><span>标记完成百分比 <b class="mono">{{ progress }}%</b></span><input v-model.number="progress" type="range" min="0" max="100" :style="{ '--range-fill': `${progress}%` }" /></label>
                  <div class="form-row"><input v-model="progressDate" type="date" :max="today" aria-label="标记日期" /><button class="primary" type="submit" :disabled="busy || !edit.name.trim()">保存</button></div>
                </form>
                <section v-else class="detail-intro">
                  <MdPreview v-if="selectedPlan.intro" :editor-id="`plan-${selectedPlan.id}`" :model-value="selectedPlan.intro" theme="dark" language="zh-CN" :no-img-zoom-in="true" />
                  <EmptyState v-else icon="book" text="这项规划还没有简介" compact />
                </section>
              </Transition>
            </div>
            <EmptyState v-else icon="target" text="从左侧选择一项规划" />
          </Transition>
        </section>
      </section>
      <div class="divider-h" :class="{ dragging: dragging === 'row' }" role="separator" aria-orientation="horizontal" @pointerdown="startResize('row', $event)" />
      <section class="future-bottom cyber-panel bracket">
        <ZoomCalendar v-model:selected="selectedDate" :tasks="taskList" :plans="plans" :selected-plan="selectedPlanId" :today="today" @range="onRange" @select-plan="choosePlanById" />
        <Transition name="fade-slide" mode="out-in">
          <section :key="selectedDate" class="day-panel">
            <header class="cyber-head"><span class="cyber-heading"><AppIcon name="calendar" :size="14" />{{ selectedDate }}</span><small class="faint">{{ relativeDayLabel(selectedDate) }}</small><small v-if="dayTasks.length" class="faint mono">{{ dayTasks.filter(task => task.done).length }} / {{ dayTasks.length }}</small></header>
            <div class="day-columns">
              <div class="day-col">
                <form class="task-add" @submit.prevent="addTask"><input v-model="newTask" :placeholder="selectedDate === today ? '为今天添加待办，回车保存' : `为 ${monthDayLabel(selectedDate)} 添加待办，回车保存`" maxlength="120" /><button class="icon-button" type="submit" aria-label="添加" :disabled="!newTask.trim()"><AppIcon name="plus" /></button></form>
                <ul v-if="dayTasks.length" class="day-tasks">
                  <li v-for="task in dayTasks" :key="task.id" :class="{ done: task.done }"><label class="task-row"><input type="checkbox" :checked="task.done" @change="toggleTask(task)" /><span class="task-title" :class="{ done: task.done }">{{ task.title }}</span><i v-if="task.priority === 'high'" class="task-priority high" title="高优先级" /></label></li>
                </ul>
                <EmptyState v-else icon="check" :text="selectedDate === today ? '今天还没有待办' : '这一天还没有待办'" compact />
              </div>
              <div class="day-col day-plans">
                <span class="cyber-heading">当天进行中的规划</span>
                <ul v-if="dayPlans.length" class="day-plan-list">
                  <li v-for="plan in dayPlans" :key="plan.id"><button class="day-plan" :class="{ active: plan.id === selectedPlanId }" @click="choose(plan)"><b>{{ plan.name }}</b><ProgressBar :value="plan.timeProgress" :height="3" /><ProgressBar :value="plan.progress" tone="accent-2" :height="3" /></button></li>
                </ul>
                <EmptyState v-else icon="target" text="这一天没有进行中的规划" compact />
              </div>
            </div>
          </section>
        </Transition>
      </section>
    </template>
    <section v-else class="cyber-panel bracket reader-future"><EmptyState icon="shield" text="规划与待办日历仅书写者可见" /></section>
  </main>
</template>
