<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import { api, type Comment, type Milestone, type NowData } from '../api/client'
import { beijingNow, lunarLabel, monthDayLabel, timeLabel, weekdayLabel } from '../lib/dates'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'

const data = ref<NowData>({ diary: { id: '', entryDate: '', content: '', secret: false, commentable: false }, moods: [], bodies: [], tasks: [] })
const comments = ref<Comment[]>([])
const milestones = ref<Milestone[]>([])
const error = ref('')
const comment = ref('')
const now = ref(beijingNow())
let clock: number | undefined

const dateLabel = computed(() => monthDayLabel(now.value))
const weekday = computed(() => weekdayLabel(now.value))
const lunar = computed(() => lunarLabel(now.value))
const hhmm = computed(() => timeLabel(now.value))
const seconds = computed(() => String(now.value.getSeconds()).padStart(2, '0'))
const theme = computed(() => (document.querySelector('.app-shell')?.classList.contains('light') ? 'light' : 'dark') as 'light' | 'dark')

async function load() {
  try {
    data.value = await api.visibleToday()
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
  <main class="page now-page reader-page">
    <section class="now-clock">
      <strong class="clock-date">{{ dateLabel }}</strong><span class="clock-week">{{ weekday }}</span><span class="clock-lunar">{{ lunar }}</span>
      <time class="clock-time mono">{{ hhmm }}<Transition name="fade" mode="out-in"><small :key="seconds" class="clock-seconds">{{ seconds }}</small></Transition></time>
    </section>
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <div v-stagger class="reader-grid">
      <article class="card reader-diary">
        <header class="card-head"><h2>今日日记</h2><small class="faint mono">{{ data.diary.entryDate }}</small></header>
        <div v-if="milestones.length" class="monument">
          <span class="milestone-badge"><AppIcon name="medal" :size="14" />里程碑</span>
          <h2 v-for="item in milestones" :key="item.id" class="monument-title">{{ item.description }}<small v-if="item.detail">{{ item.detail }}</small></h2>
        </div>
        <MdPreview v-if="data.diary.id" editor-id="reader-diary" :model-value="data.diary.content" :theme="theme" language="zh-CN" :no-img-zoom-in="true" />
        <EmptyState v-else icon="book" text="今天没有你可以查看的日记" />
        <section v-if="data.diary.id && (comments.length || data.diary.commentable)" class="past-comments">
          <h3 class="card-title">评论<small v-if="comments.length">{{ comments.length }}</small></h3>
          <ul class="comment-list"><li v-for="item in comments" :key="item.id"><p>{{ item.content }}</p><small class="mono faint">{{ timeLabel(item.createdAt) }}</small></li></ul>
          <form v-if="data.diary.commentable" class="comment-form" @submit.prevent="addComment"><input v-model="comment" placeholder="写下评论" maxlength="500" /><button class="icon-button" type="submit" aria-label="发送" :disabled="!comment.trim()"><AppIcon name="send" :size="16" /></button></form>
        </section>
      </article>
      <div class="reader-side">
        <article class="card">
          <h2 class="card-title">心情<small v-if="data.moods.length">{{ data.moods.length }} 条</small></h2>
          <ul v-if="data.moods.length" class="reader-list">
            <li v-for="item in data.moods" :key="item.id"><span class="emoji">{{ item.tags.map(tag => tag.emoji).join(' ') }}</span><span>{{ item.tags.map(tag => tag.name).join('、') }}<small v-if="item.note" class="faint"> · {{ item.note }}</small></span><span class="when">{{ timeLabel(item.recordedAt) }}</span></li>
          </ul>
          <EmptyState v-else icon="smile" text="今天没有可查看的心情记录" compact />
        </article>
        <article class="card">
          <h2 class="card-title">身体</h2>
          <ul v-if="data.bodies.length" class="reader-list">
            <li v-for="item in data.bodies" :key="item.id"><span class="reader-score">{{ item.score }}</span><span>{{ item.note || '' }}</span><span class="when">{{ timeLabel(item.recordedAt) }}</span></li>
          </ul>
          <EmptyState v-else icon="pulse" text="今天没有可查看的身体记录" compact />
        </article>
        <article class="card">
          <h2 class="card-title">今日任务<small v-if="data.tasks.length">{{ data.tasks.filter(task => task.done).length }} / {{ data.tasks.length }}</small></h2>
          <ul v-if="data.tasks.length" class="past-tasks">
            <li v-for="task in data.tasks" :key="task.id" :class="{ done: task.done }"><i class="task-state"><AppIcon v-if="task.done" name="check" :size="12" :stroke-width="2.5" /></i><div><span class="task-title" :class="{ done: task.done }">{{ task.title }}</span><small v-if="task.description" class="faint">{{ task.description }}</small></div></li>
          </ul>
          <EmptyState v-else icon="check" text="今天没有可查看的任务" compact />
        </article>
      </div>
    </div>
  </main>
</template>
