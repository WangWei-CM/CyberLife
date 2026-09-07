<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import ZoomCalendar from '../components/ZoomCalendar.vue'
import ProgressBar from '../components/ProgressBar.vue'
import MarkdownPreview from '../components/MarkdownPreview.vue'
import DiaryEditor from '../components/DiaryEditor.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import { api, type FutureTask, type Plan, type Task } from '../api/client'
import { authState, isWriter } from '../stores/auth'
import { ui } from '../stores/ui'
import { useCountUp } from '../lib/motion'
import { addDaysISO, diffDays, monthDayLabel, relativeDayLabel, todayISO } from '../lib/dates'

const today = todayISO()
const plans = ref<Plan[]>([])
const tasks = ref(new Map<string, FutureTask>())
const fetched = ref(new Set<string>())
const selectedPlanId = ref('')
const selectedDate = ref(today)
const editMode = ref(false)
const planDialogOpen = ref(false)
const progress = ref(0)
const progressDate = ref(today)
const edit = ref({ name: '', startDate: today, endDate: today, intro: '' })
const creating = ref(false)
const form = ref({ name: '', startDate: today, endDate: addDaysISO(today, 30), intro: '' })
const newTask = ref('')
const error = ref('')
const listWidth = ref(Number(localStorage.getItem('future-list-width') || 30))
const topHeight = ref(Number(localStorage.getItem('future-top-height-v2') || 340))
const dragging = ref<'col' | 'row' | null>(null)
const busy = ref(false)
const uploading = ref('')
const draggingPlanId = ref('')
const dropPlanId = ref('')
const planOrderBusy = ref(false)
const taskDrawerOpen = ref(false)
const taskLoading = ref(false)
const selectedTask = ref<Task | null>(null)
const taskEditing = ref(false)
const taskBusy = ref(false)
const taskError = ref('')
const taskDraft = ref({ title: '', description: '', priority: 'normal' as Task['priority'] })
const taskAccessDraft = ref({ presetId: '', secret: false, commentable: false })
const milestoneDraft = ref({ description: '', detail: '' })
const presets = ref<{ id: string; name: string }[]>([])

const comparePlans = (a: Plan, b: Plan) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0) || a.endDate.localeCompare(b.endDate)
const sortedPlans = computed(() => [...plans.value].sort(comparePlans))
const selectedPlan = computed(() => plans.value.find(plan => plan.id === selectedPlanId.value) ?? null)
const taskList = computed(() => [...tasks.value.values()])
const dayTasks = computed(() => taskList.value.filter(task => task.date === selectedDate.value).sort((a, b) => Number(a.done) - Number(b.done)))
const dayPlans = computed(() => plans.value.filter(plan => plan.startDate <= selectedDate.value && selectedDate.value <= plan.endDate).sort(comparePlans))
const timeDisplay = useCountUp(() => selectedPlan.value?.timeProgress ?? 0)
const planDisplay = useCountUp(() => selectedPlan.value?.progress ?? 0)
const remainingDays = computed(() => selectedPlan.value ? diffDays(today, selectedPlan.value.endDate) : 0)
const isOngoing = (plan: Plan) => plan.startDate <= today && today <= plan.endDate && plan.progress < 100
const fileSize = (bytes: number) => bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
const theme = computed(() => (document.querySelector('.app-shell')?.classList.contains('light') ? 'light' : 'dark') as 'light' | 'dark')
const taskVaultKey = computed(() => `${authState.actor?.lifeId ?? 'life'}:${selectedTask.value?.taskDate ?? selectedDate.value}:future-task:${selectedTask.value?.id ?? ''}`)

async function loadPlans() {
  try {
    plans.value = (await api.plans()).items
    if (ui.pendingPlanId) { const pending = plans.value.find(plan => plan.id === ui.pendingPlanId); ui.pendingPlanId = ''; if (pending) { choose(pending); return } }
    if (!selectedPlanId.value && sortedPlans.value.length) choose(sortedPlans.value.find(isOngoing) ?? sortedPlans.value[0])
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取失败' }
}
async function loadTaskPresets() {
  if (!isWriter.value) return
  try { presets.value = (await api.presets()).items } catch { presets.value = [] }
}
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
function openPlanDialog() { if (!selectedPlan.value && sortedPlans.value.length) choose(sortedPlans.value.find(isOngoing) ?? sortedPlans.value[0]); planDialogOpen.value = true }
function closePlanDialog() { planDialogOpen.value = false; editMode.value = false }
function replacePlan(next: Plan) { plans.value = plans.value.map(plan => plan.id === next.id ? { ...plan, ...next } : plan) }
function startPlanDrag(plan: Plan, event: DragEvent) {
  if (!isWriter.value || planOrderBusy.value) return
  draggingPlanId.value = plan.id
  dropPlanId.value = ''
  event.dataTransfer?.setData('text/plain', plan.id)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}
function hoverPlanDrop(plan: Plan, event: DragEvent) {
  if (!draggingPlanId.value || draggingPlanId.value === plan.id) return
  event.preventDefault()
  dropPlanId.value = plan.id
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}
async function dropPlan(plan: Plan, event: DragEvent) {
  event.preventDefault()
  const sourceID = draggingPlanId.value || event.dataTransfer?.getData('text/plain') || ''
  draggingPlanId.value = ''
  dropPlanId.value = ''
  if (!sourceID || sourceID === plan.id || planOrderBusy.value) return
  const next = [...sortedPlans.value]
  const sourceIndex = next.findIndex(item => item.id === sourceID)
  const targetIndex = next.findIndex(item => item.id === plan.id)
  if (sourceIndex < 0 || targetIndex < 0) return
  const [moved] = next.splice(sourceIndex, 1)
  next.splice(targetIndex, 0, moved)
  const previous = plans.value
  const order = new Map(next.map((item, index) => [item.id, index]))
  plans.value = plans.value.map(item => ({ ...item, sortOrder: order.get(item.id) ?? item.sortOrder }))
  planOrderBusy.value = true
  try { await api.reorderPlans(next.map(item => item.id)) } catch (cause) { plans.value = previous; error.value = cause instanceof Error ? cause.message : '排序保存失败' } finally { planOrderBusy.value = false }
}
function endPlanDrag() { draggingPlanId.value = ''; dropPlanId.value = '' }
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
async function uploadImage(kind: 'cover' | 'icon', event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const plan = selectedPlan.value
  if (!file || !plan) return
  if (file.size > 5 * 1024 * 1024) { error.value = '图片不能超过 5MB'; input.value = ''; return }
  uploading.value = kind
  try { const updated = await api.uploadPlanImage(plan.id, kind, file); replacePlan({ ...updated, coverUrl: updated.coverUrl ? `${updated.coverUrl}?v=${Date.now()}` : '', iconUrl: updated.iconUrl ? `${updated.iconUrl}?v=${Date.now()}` : '' }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '上传失败' } finally { uploading.value = ''; input.value = '' }
}
async function uploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const plan = selectedPlan.value
  if (!file || !plan) return
  if (file.size > 20 * 1024 * 1024) { error.value = '文件不能超过 20MB'; input.value = ''; return }
  uploading.value = 'file'
  try { const added = await api.uploadPlanFile(plan.id, file); replacePlan({ ...plan, files: [...(plan.files ?? []), added] }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '上传失败' } finally { uploading.value = ''; input.value = '' }
}
async function removeFile(fileID: string) {
  const plan = selectedPlan.value
  if (!plan) return
  try { await api.deletePlanFile(plan.id, fileID); replacePlan({ ...plan, files: (plan.files ?? []).filter(file => file.id !== fileID) }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败' }
}
async function addTask() {
  const title = newTask.value.trim()
  if (!title) return
  try { const task = await api.addTask(title, '', 'normal', selectedDate.value); newTask.value = ''; syncTask(task) } catch (cause) { error.value = cause instanceof Error ? cause.message : '添加失败' }
}
function asFutureTask(task: Task): FutureTask { return { id: task.id, date: task.taskDate, title: task.title, priority: task.priority, done: task.done, presetId: task.presetId, secret: task.secret } }
function syncTask(task: Task) {
  tasks.value = new Map(tasks.value).set(task.id, asFutureTask(task))
  if (selectedTask.value?.id === task.id) selectedTask.value = task
}
async function toggleTask(task: FutureTask) {
  if (!isWriter.value) return
  const next = !task.done
  tasks.value = new Map(tasks.value).set(task.id, { ...task, done: next })
  try { syncTask(await api.setTaskDone(task.id, next, task.date)) } catch (cause) { tasks.value = new Map(tasks.value).set(task.id, { ...task, done: !next }); error.value = cause instanceof Error ? cause.message : '更新失败' }
}
function taskPriorityLabel(priority: Task['priority']) { return priority === 'high' ? '高优先级' : priority === 'low' ? '低优先级' : '普通优先级' }
function closeTaskDrawer() { taskDrawerOpen.value = false; taskLoading.value = false; selectedTask.value = null; taskEditing.value = false; taskError.value = '' }
async function openTaskDrawer(task: FutureTask) {
  if (!isWriter.value) return
  taskDrawerOpen.value = true
  taskLoading.value = true
  taskError.value = ''
  selectedTask.value = null
  taskEditing.value = false
  try {
    const detail = await api.task(task.id, task.date)
    selectedTask.value = detail
    taskDraft.value = { title: detail.title, description: detail.description, priority: detail.priority }
    taskAccessDraft.value = { presetId: detail.presetId ?? '', secret: !!detail.secret, commentable: !!detail.commentable }
  } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '读取任务详情失败' } finally { taskLoading.value = false }
}
async function saveTask() {
  const task = selectedTask.value
  if (!task || taskBusy.value || !taskDraft.value.title.trim()) return
  taskBusy.value = true
  taskError.value = ''
  try { syncTask(await api.updateFutureTask(task.id, task.taskDate, taskDraft.value)); taskEditing.value = false } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '保存失败' } finally { taskBusy.value = false }
}
async function saveTaskAccess() {
  const task = selectedTask.value
  if (!task || taskBusy.value) return
  taskBusy.value = true
  taskError.value = ''
  try { syncTask(await api.setTaskAccess(task.id, taskAccessDraft.value.presetId, taskAccessDraft.value.secret, taskAccessDraft.value.commentable)) } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '权限保存失败' } finally { taskBusy.value = false }
}
async function addTaskMilestone() {
  const task = selectedTask.value
  if (!task || taskBusy.value || !milestoneDraft.value.description.trim()) return
  taskBusy.value = true
  taskError.value = ''
  try { await api.addMilestone({ target_type: 'task', target_id: task.id, description: milestoneDraft.value.description.trim(), detail: milestoneDraft.value.detail.trim(), preset_id: taskAccessDraft.value.presetId, secret: taskAccessDraft.value.secret }); milestoneDraft.value = { description: '', detail: '' } } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '里程碑保存失败' } finally { taskBusy.value = false }
}
async function removeTask() {
  const task = selectedTask.value
  if (!task || taskBusy.value) return
  taskBusy.value = true
  taskError.value = ''
  try { await api.deleteFutureTask(task.id, task.taskDate); const next = new Map(tasks.value); next.delete(task.id); tasks.value = next; closeTaskDrawer() } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '删除失败' } finally { taskBusy.value = false }
}
async function toggleTaskFromDrawer() {
  const task = selectedTask.value
  if (!task || taskBusy.value) return
  taskBusy.value = true
  taskError.value = ''
  try { syncTask(await api.setTaskDone(task.id, !task.done, task.taskDate)) } catch (cause) { taskError.value = cause instanceof Error ? cause.message : '更新失败' } finally { taskBusy.value = false }
}
function startResize(kind: 'col' | 'row', event: PointerEvent) {
  dragging.value = kind
  document.body.classList.add(kind === 'col' ? 'resizing-col' : 'resizing-row')
  const container = (event.currentTarget as HTMLElement).parentElement!.getBoundingClientRect()
  const move = (next: PointerEvent) => {
    if (kind === 'col') listWidth.value = Math.max(22, Math.min(50, ((next.clientX - container.left) / container.width) * 100))
    else {
      const minTop = 220
      const minBottom = 240
      const maxTop = Math.max(minTop, Math.min(720, container.height - 6 - minBottom))
      topHeight.value = Math.max(minTop, Math.min(maxTop, next.clientY - container.top))
    }
  }
  const up = () => {
    dragging.value = null
    document.body.classList.remove('resizing-col', 'resizing-row')
    localStorage.setItem('future-list-width', String(listWidth.value))
    localStorage.setItem('future-top-height-v2', String(topHeight.value))
    window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move); window.addEventListener('pointerup', up)
  event.preventDefault()
}
watch(selectedPlan, plan => { if (plan) progress.value = plan.progress })
watch(() => ui.pendingPlanId, id => { if (id && plans.value.length) { choosePlanById(id); ui.pendingPlanId = '' } })
onMounted(() => { loadPlans(); loadTaskPresets(); ensureTasks(addDaysISO(today, -7), addDaysISO(today, 60)) })
</script>

<template>
  <main class="page future-page" :style="{ '--future-top-height': `${topHeight}px` }">
    <div class="scanlines" aria-hidden="true" />
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <section v-if="planDialogOpen" class="future-top plan-dialog" :style="{ '--list-width': `${listWidth}%`, height: `${topHeight}px` }" role="dialog" aria-modal="true" aria-label="规划详情与编辑">
      <button class="icon-button plan-dialog-close" aria-label="关闭规划面板" @click="closePlanDialog"><AppIcon name="close" :size="16" /></button>
      <aside v-stagger class="future-list cyber-panel bracket">
        <header class="cyber-head"><span class="cyber-heading"><AppIcon name="target" :size="14" />规划</span><small class="mono faint">{{ sortedPlans.filter(isOngoing).length }} 进行中 / {{ plans.length }}</small><button v-if="isWriter" class="text-button" :aria-expanded="creating" @click="creating = !creating"><AppIcon :name="creating ? 'close' : 'plus'" :size="14" />{{ creating ? '取消' : '新建' }}</button></header>
        <Transition name="fade-slide">
          <form v-if="creating && isWriter" class="plan-form" @submit.prevent="createPlan">
            <div class="form-control"><input v-model="form.name" placeholder="规划名称" maxlength="60" required /></div>
            <div class="form-row"><input v-model="form.startDate" type="date" required aria-label="开始日期" /><input v-model="form.endDate" type="date" required aria-label="截止日期" /></div>
            <div class="form-control"><textarea v-model="form.intro" placeholder="简介（支持 Markdown）" rows="3" /></div>
            <button class="primary" type="submit" :disabled="busy || !form.name.trim()">创建规划</button>
          </form>
        </Transition>
        <TransitionGroup v-if="sortedPlans.length" name="list" tag="ul" class="plan-list">
          <li v-for="plan in sortedPlans" :key="plan.id" :class="{ 'plan-drop-target': dropPlanId === plan.id && draggingPlanId !== plan.id }" @dragover="hoverPlanDrop(plan, $event)" @drop="dropPlan(plan, $event)">
            <button v-glow class="plan-card" :class="{ active: plan.id === selectedPlanId, ongoing: isOngoing(plan), done: plan.progress >= 100, dragging: draggingPlanId === plan.id }" :draggable="isWriter && !planOrderBusy" @dragstart="startPlanDrag(plan, $event)" @dragend="endPlanDrag" @click="choose(plan)">
              <span class="plan-card-inner">
                <span class="plan-name-row"><img v-if="plan.iconUrl" class="plan-icon small" :src="plan.iconUrl" alt="" /><b class="plan-name">{{ plan.name }}</b></span>
                <small class="plan-dates mono">{{ monthDayLabel(plan.startDate) }} → {{ monthDayLabel(plan.endDate) }}<em>{{ relativeDayLabel(plan.endDate) }}</em></small>
                <ProgressBar :value="plan.timeProgress" :height="3" />
                <ProgressBar :value="plan.progress" tone="accent-2" :height="3" />
              </span>
            </button>
          </li>
        </TransitionGroup>
        <EmptyState v-else icon="target" :text="isWriter ? '还没有规划' : '没有可查看的规划'" />
      </aside>
      <div class="divider-v" :class="{ dragging: dragging === 'col' }" role="separator" aria-orientation="vertical" @pointerdown="startResize('col', $event)" />
      <section class="future-detail cyber-panel glass bracket">
        <Transition name="fade-slide" mode="out-in">
          <div v-if="selectedPlan" :key="selectedPlan.id" class="detail-body">
            <div v-if="selectedPlan.coverUrl" class="plan-cover" :style="{ backgroundImage: `url(${selectedPlan.coverUrl})` }" role="img" :aria-label="`${selectedPlan.name} 的封面`" />
            <header class="detail-head">
              <div class="detail-title-row">
                <img v-if="selectedPlan.iconUrl" class="plan-icon" :src="selectedPlan.iconUrl" alt="" />
                <div>
                  <h1 class="glitch-text detail-title" :data-text="selectedPlan.name">{{ selectedPlan.name }}</h1>
                  <p class="detail-dates mono">{{ selectedPlan.startDate }} — {{ selectedPlan.endDate }}<span>{{ remainingDays >= 0 ? `剩余 ${remainingDays} 天` : `已过 ${-remainingDays} 天` }}</span></p>
                </div>
              </div>
              <button v-if="isWriter" class="text-button" :aria-pressed="editMode" @click="editMode = !editMode"><AppIcon :name="editMode ? 'close' : 'edit'" :size="14" />{{ editMode ? '退出编辑' : '编辑模式' }}</button>
            </header>
            <div class="dual-progress">
              <div class="dual-row"><span class="cyber-heading">时间进度</span><ProgressBar :value="selectedPlan.timeProgress" :height="6" /><b class="mono">{{ Math.round(timeDisplay) }}%</b></div>
              <div class="dual-row"><span class="cyber-heading magenta">计划进度</span><ProgressBar :value="selectedPlan.progress" tone="accent-2" :height="6" /><b class="mono">{{ Math.round(planDisplay) }}%</b></div>
            </div>
            <Transition name="fade-slide" mode="out-in">
              <form v-if="editMode && isWriter" class="progress-form" @submit.prevent="savePlan">
                <div class="form-control"><input v-model="edit.name" placeholder="规划名称" maxlength="60" required aria-label="规划名称" /></div>
                <div class="form-row"><input v-model="edit.startDate" type="date" required aria-label="开始日期" /><input v-model="edit.endDate" type="date" required aria-label="截止日期" /></div>
                <div class="form-control"><textarea v-model="edit.intro" rows="6" placeholder="简介（支持 Markdown）" aria-label="简介" /></div>
                <div class="asset-row">
                  <label class="text-button upload-trigger" :class="{ busy: uploading === 'cover' }"><AppIcon name="image" :size="14" />{{ selectedPlan.coverUrl ? '更换封面' : '上传封面' }}<input type="file" accept="image/*" :disabled="!!uploading" @change="uploadImage('cover', $event)" /></label>
                  <label class="text-button upload-trigger" :class="{ busy: uploading === 'icon' }"><AppIcon name="spark" :size="14" />{{ selectedPlan.iconUrl ? '更换图标' : '上传图标' }}<input type="file" accept="image/*" :disabled="!!uploading" @change="uploadImage('icon', $event)" /></label>
                  <label class="text-button upload-trigger" :class="{ busy: uploading === 'file' }"><AppIcon name="upload" :size="14" />添加文件<input type="file" :disabled="!!uploading" @change="uploadFile" /></label>
                  <small class="faint">图片 ≤ 5MB，文件 ≤ 20MB</small>
                </div>
                <label class="field"><span>标记完成百分比 <b class="mono">{{ progress }}%</b></span><input v-model.number="progress" type="range" min="0" max="100" :style="{ '--range-fill': `${progress}%` }" /></label>
                <div class="form-row"><input v-model="progressDate" type="date" :max="today" aria-label="标记日期" /><button class="primary" type="submit" :disabled="busy || !edit.name.trim()">保存</button></div>
              </form>
              <section v-else class="detail-intro">
                <MarkdownPreview v-if="selectedPlan.intro" :editor-id="`plan-${selectedPlan.id}`" :model-value="selectedPlan.intro" theme="dark" theme-class="theme-future" />
                <EmptyState v-else icon="book" text="这项规划还没有简介" compact />
              </section>
            </Transition>
            <section v-if="selectedPlan.files?.length" class="plan-files">
              <span class="cyber-heading"><AppIcon name="download" :size="12" />文件</span>
              <TransitionGroup name="list" tag="ul" class="file-list">
                <li v-for="file in selectedPlan.files" :key="file.id">
                  <a class="file-link" :href="file.url" download><AppIcon name="book" :size="14" /><span class="file-name">{{ file.originalName }}</span><small class="mono faint">{{ fileSize(file.byteSize) }}</small></a>
                  <button v-if="isWriter && editMode" class="icon-button" aria-label="删除文件" @click="removeFile(file.id)"><AppIcon name="trash" :size="14" /></button>
                </li>
              </TransitionGroup>
            </section>
          </div>
          <EmptyState v-else icon="target" :text="sortedPlans.length ? '从左侧选择一项规划' : (isWriter ? '新建一项规划开始' : '没有可查看的规划')" />
        </Transition>
      </section>
    </section>
    <section class="future-bottom cyber-panel bracket">
      <ZoomCalendar v-model:selected="selectedDate" :tasks="taskList" :plans="plans" :selected-plan="selectedPlanId" :today="today" @range="onRange" @select-plan="choosePlanById" />
      <Transition name="fade-slide" mode="out-in">
        <section :key="selectedDate" class="day-panel">
          <div class="day-frame">
            <div class="day-content">
              <header class="cyber-head"><span class="cyber-heading"><AppIcon name="calendar" :size="14" />{{ selectedDate }}</span><small class="faint">{{ relativeDayLabel(selectedDate) }}</small><small v-if="dayTasks.length" class="faint mono">{{ dayTasks.filter(task => task.done).length }} / {{ dayTasks.length }}</small></header>
              <div class="day-columns">
                <div class="day-col">
                  <form v-if="isWriter" class="task-add" @submit.prevent="addTask"><input v-model="newTask" :placeholder="selectedDate === today ? '为今天添加待办，回车保存' : `为 ${monthDayLabel(selectedDate)} 添加待办，回车保存`" maxlength="120" /><button class="icon-button" type="submit" aria-label="添加" :disabled="!newTask.trim()"><AppIcon name="plus" /></button></form>
                  <ul v-if="dayTasks.length" class="day-tasks">
                    <li v-for="task in dayTasks" :key="task.id" :class="{ done: task.done }">
                      <div class="task-row future-task-row" :class="{ editable: isWriter }" :role="isWriter ? 'button' : undefined" :tabindex="isWriter ? 0 : undefined" @click="openTaskDrawer(task)" @keydown.enter="openTaskDrawer(task)">
                        <input type="checkbox" :checked="task.done" :disabled="!isWriter" :aria-label="`${task.title}完成状态`" @click.stop @change="toggleTask(task)" />
                        <span class="task-title" :class="{ done: task.done }">{{ task.title }}</span>
                        <i v-if="task.priority === 'high'" class="task-priority high" title="高优先级" />
                        <AppIcon v-if="isWriter" name="chevron-right" :size="14" class="future-task-open" />
                      </div>
                    </li>
                  </ul>
                  <EmptyState v-else icon="check" :text="selectedDate === today ? '今天还没有待办' : '这一天还没有待办'" compact />
                </div>
                <div class="day-col day-plans">
                  <header class="day-plan-head"><span class="cyber-heading">当天进行中的规划</span><button class="icon-button plan-editor-trigger" aria-label="查看或编辑规划" title="查看或编辑规划" @click="openPlanDialog"><AppIcon name="edit" :size="14" /></button></header>
                  <ul v-if="dayPlans.length" class="day-plan-list">
                    <li v-for="plan in dayPlans" :key="plan.id" :class="{ 'plan-drop-target': dropPlanId === plan.id && draggingPlanId !== plan.id }" @dragover="hoverPlanDrop(plan, $event)" @drop="dropPlan(plan, $event)"><button class="day-plan" :class="{ active: plan.id === selectedPlanId, dragging: draggingPlanId === plan.id }" :draggable="isWriter && !planOrderBusy" @dragstart="startPlanDrag(plan, $event)" @dragend="endPlanDrag" @click="choose(plan)"><b>{{ plan.name }}</b><ProgressBar :value="plan.timeProgress" :height="3" /><ProgressBar :value="plan.progress" tone="accent-2" :height="3" /></button></li>
                  </ul>
                  <EmptyState v-else icon="target" text="这一天没有进行中的规划" compact />
                </div>
              </div>
            </div>
          </div>
        </section>
      </Transition>
    </section>
    <Teleport to=".app-shell">
      <Transition name="future-drawer">
        <div v-if="taskDrawerOpen" class="future-task-drawer-layer" @click.self="closeTaskDrawer">
          <aside class="future-task-drawer" role="dialog" aria-modal="true" aria-label="未来待办详情">
          <header class="future-task-drawer-head">
            <div><small class="faint mono">{{ selectedTask?.taskDate ?? selectedDate }}</small><h2>待办详情</h2></div>
            <div class="future-task-drawer-actions"><button v-if="selectedTask" class="icon-button" :aria-label="taskEditing ? '退出编辑' : '编辑待办'" @click="taskEditing = !taskEditing"><AppIcon :name="taskEditing ? 'close' : 'edit'" :size="16" /></button><button class="icon-button" aria-label="关闭待办详情" @click="closeTaskDrawer"><AppIcon name="close" :size="16" /></button></div>
          </header>
          <div class="future-task-drawer-content">
            <EmptyState v-if="taskLoading" icon="target" text="正在读取待办详情" compact />
            <template v-else-if="selectedTask">
              <p v-if="taskError" class="error future-task-error" role="alert">{{ taskError }}</p>
              <div class="future-task-status">
                <span class="task-priority-label" :class="`priority-${selectedTask.priority}`">{{ taskPriorityLabel(selectedTask.priority) }}</span>
                <span class="future-slant-control future-slant-button"><button class="text-button" :disabled="taskBusy" @click="toggleTaskFromDrawer"><AppIcon :name="selectedTask.done ? 'repeat' : 'check'" :size="14" />{{ selectedTask.done ? '标记未完成' : '标记完成' }}</button></span>
              </div>
              <form v-if="taskEditing" class="future-task-editor" @submit.prevent="saveTask">
                <label class="future-slant-control future-slant-field"><input v-model="taskDraft.title" maxlength="120" aria-label="任务标题" /></label>
                <label class="future-slant-control future-slant-field"><select v-model="taskDraft.priority" aria-label="任务优先级"><option value="high">高优先级</option><option value="normal">普通优先级</option><option value="low">低优先级</option></select></label>
                <div class="future-slant-control future-slant-editor"><DiaryEditor :model-value="taskDraft.description" editor-id="future-task-detail-editor" :theme="theme" :vault-key="taskVaultKey" placeholder="任务详细描述（支持 Markdown）" @update:model-value="taskDraft.description = $event" /></div>
                <span class="future-slant-control future-slant-button future-task-save"><button class="primary" type="submit" :disabled="taskBusy || !taskDraft.title.trim()">保存待办</button></span>
              </form>
              <section v-else class="future-task-preview">
                <h1>{{ selectedTask.title }}</h1>
                <MarkdownPreview v-if="selectedTask.description" :model-value="selectedTask.description" :editor-id="`future-task-${selectedTask.id}`" :theme="theme" theme-class="theme-future" />
                <EmptyState v-else icon="book" text="没有详细描述" compact />
              </section>
              <section class="future-task-settings">
                <header><b>权限与互动</b><small class="faint">附件默认继承任务权限</small></header>
                <label class="field"><span>权限预设</span><span class="future-slant-control future-slant-field"><select v-model="taskAccessDraft.presetId"><option value="">锚点范围内公开</option><option v-for="preset in presets" :key="preset.id" :value="preset.id">{{ preset.name }}</option></select></span></label>
                <label class="check-field future-slant-check"><input v-model="taskAccessDraft.secret" type="checkbox" />绝密（仅书写者可见）</label>
                <label class="check-field future-slant-check"><input v-model="taskAccessDraft.commentable" type="checkbox" />允许评论</label>
                <span class="future-slant-control future-slant-button"><button class="text-button" :disabled="taskBusy" @click="saveTaskAccess"><AppIcon name="shield" :size="14" />保存权限设置</button></span>
                <div class="future-milestone-form"><b>标记里程碑</b><label class="future-slant-control future-slant-field"><input v-model="milestoneDraft.description" placeholder="里程碑描述" maxlength="120" /></label><label class="future-slant-control future-slant-field"><textarea v-model="milestoneDraft.detail" placeholder="详细信息（可选）" maxlength="500" rows="2" /></label><span class="future-slant-control future-slant-button"><button class="text-button" :disabled="taskBusy || !milestoneDraft.description.trim()" @click="addTaskMilestone"><AppIcon name="medal" :size="14" />添加里程碑</button></span></div>
              </section>
              <footer class="future-task-drawer-footer"><span class="future-slant-control future-slant-button"><button class="text-button danger" :disabled="taskBusy" @click="removeTask"><AppIcon name="trash" :size="14" />删除待办</button></span><span class="faint">日期固定为 {{ selectedTask.taskDate }}</span></footer>
            </template>
            <EmptyState v-else icon="target" :text="taskError || '未找到待办'" compact />
          </div>
          </aside>
        </div>
      </Transition>
    </Teleport>
  </main>
</template>
