<script setup lang="ts">
import { computed } from 'vue'
import type { ScheduleAgenda, ScheduleOccurrence } from '../api/client'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ agenda: ScheduleAgenda | null; now: Date }>()
const current = computed(() => props.agenda?.current ?? null)
const next = computed(() => props.agenda?.next ?? [])
function date(value: string) { return new Date(value) }
function range(item: ScheduleOccurrence) { return `${date(item.startsAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}–${date(item.endsAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}` }
function time(value: string) { return date(value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
const featured = computed(() => current.value ?? next.value[0] ?? null)
const progress = computed(() => { if (!current.value) return 0; const a = date(current.value.startsAt).getTime(); const b = date(current.value.endsAt).getTime(); return Math.max(0, Math.min(100, ((props.now.getTime() - a) / (b - a)) * 100)) })
const countdown = computed(() => { if (!featured.value || current.value) return ''; const seconds = Math.max(0, Math.floor((date(featured.value.startsAt).getTime() - props.now.getTime()) / 1000)); const h = Math.floor(seconds / 3600); const m = Math.floor((seconds % 3600) / 60); return h ? `${h}小时${m}分后` : `${m}分钟后` })
</script>

<template>
  <section class="card schedule-card" aria-label="进行中的课表">
    <header class="card-title"><span>{{ current ? '进行中' : '下一节课' }}</span><small><AppIcon name="calendar" :size="14" />课表</small></header>
    <div class="schedule-layout">
      <article v-if="featured" class="schedule-featured" :class="{ active: current }" :style="{ '--course-progress': `${progress}%` }">
        <div class="schedule-featured-kicker">{{ current ? '正在上课' : countdown }}</div>
        <h3>{{ featured.title }}</h3>
        <p v-if="featured.location" class="schedule-location">{{ featured.location }}</p>
        <time class="schedule-time mono">{{ range(featured) }}</time>
        <i v-if="current" class="schedule-progress" aria-hidden="true"><b /></i>
      </article>
      <article v-else class="schedule-featured schedule-empty"><AppIcon name="calendar" :size="26" /><span>暂无课程安排</span></article>
      <div class="schedule-next-grid" aria-label="接下来四节课">
        <article v-for="(item, index) in next.slice(0, 4)" :key="item.classId + item.startsAt" class="schedule-next-item">
          <small class="mono">{{ index + 1 }}</small><strong>{{ item.title }}</strong><time class="mono">{{ time(item.startsAt) }}–{{ time(item.endsAt) }}</time><span v-if="item.location">{{ item.location }}</span>
        </article>
        <i v-for="index in Math.max(0, 4 - next.length)" :key="`empty-${index}`" class="schedule-next-item schedule-next-empty" aria-hidden="true" />
      </div>
    </div>
  </section>
</template>
