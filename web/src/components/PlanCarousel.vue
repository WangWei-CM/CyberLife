<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Plan } from '../api/client'
import { monthDayLabel } from '../lib/dates'
import ProgressBar from './ProgressBar.vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{ plans: Plan[]; interval?: number }>(), { interval: 6000 })
const emit = defineEmits<{ (event: 'select', plan: Plan): void }>()
const index = ref(0)
const paused = ref(false)
let timer: number | undefined

const current = computed(() => props.plans[Math.min(index.value, props.plans.length - 1)] ?? null)
function restart() {
  if (timer) clearInterval(timer)
  timer = undefined
  if (paused.value || props.plans.length < 2) return
  timer = window.setInterval(() => { index.value = (index.value + 1) % props.plans.length }, Math.max(1500, props.interval))
}
function go(next: number) { index.value = (next + props.plans.length) % props.plans.length; restart() }
function pause(value: boolean) { paused.value = value; restart() }
watch(() => [props.plans.length, props.interval], () => { index.value = Math.min(index.value, Math.max(props.plans.length - 1, 0)); restart() })
onMounted(restart)
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <section v-if="current" class="plan-carousel" aria-roledescription="轮播" aria-label="进行中的规划" @mouseenter="pause(true)" @mouseleave="pause(false)" @focusin="pause(true)" @focusout="pause(false)">
    <div class="carousel-stage">
      <Transition name="carousel">
        <button :key="current.id" v-glow class="plan-banner glow-edge" @click="emit('select', current)">
          <span class="banner-head">
            <b class="banner-title">{{ current.name }}</b>
            <small class="banner-dates mono">{{ monthDayLabel(current.startDate) }} → {{ monthDayLabel(current.endDate) }}</small>
          </span>
          <span class="banner-bars">
            <span class="banner-row"><small>时间</small><ProgressBar :value="current.timeProgress" label /></span>
            <span class="banner-row"><small>进度</small><ProgressBar :value="current.progress" tone="accent-2" label /></span>
          </span>
          <AppIcon name="chevron-right" :size="16" class="banner-arrow" />
        </button>
      </Transition>
    </div>
    <div v-if="plans.length > 1" class="carousel-dots" role="tablist" aria-label="切换规划">
      <button v-for="(plan, i) in plans" :key="plan.id" role="tab" :aria-selected="i === index" :aria-label="plan.name" :class="{ active: i === index }" @click="go(i)" />
    </div>
  </section>
</template>
