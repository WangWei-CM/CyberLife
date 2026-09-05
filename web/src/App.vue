<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { authState, isAdmin, isWriter, logout, restoreSession, roleLabel } from './stores/auth'
import { DUR, reducedMotion, transitionTheme } from './lib/motion'
import { isDaytime } from './lib/dates'
import AppIcon from './components/AppIcon.vue'
import MusicControl from './components/MusicControl.vue'
import NotificationCenter from './components/NotificationCenter.vue'
import LoginView from './views/LoginView.vue'
import AdminLoginView from './views/AdminLoginView.vue'
import AdminView from './views/AdminView.vue'
import NowView from './views/NowView.vue'
import ReaderView from './views/ReaderView.vue'
import PastView from './views/PastView.vue'
import FutureView from './views/FutureView.vue'
import SettingsView from './views/SettingsView.vue'

type Screen = 'past' | 'now' | 'future' | 'settings'
type NavPosition = 'top' | 'left' | 'right' | 'bottom'
export type Appearance = 'dark' | 'light' | 'auto'

const screens: { key: Screen; label: string }[] = [{ key: 'past', label: '过去' }, { key: 'now', label: '现在' }, { key: 'future', label: '未来' }]
const screen = ref<Screen>('now')
const navPosition = ref<NavPosition>((localStorage.getItem('cyberlife-nav-position') as NavPosition) || 'top')
const appearance = ref<Appearance>((localStorage.getItem('cyberlife-theme') as Appearance) || 'dark')
const secretMode = ref(localStorage.getItem('cyberlife-secret-mode') === 'true')
const isAdminPath = window.location.pathname === '/admin'
const shell = ref<HTMLElement>()
const nav = ref<HTMLElement>()
const indicator = ref({ x: 0, y: 0, w: 0, h: 0 })
const minute = ref(Date.now())
const dropTarget = ref(false)
const entering = ref(false)
let minuteTimer: number | undefined
let enterTimer: number | undefined
let navObserver: ResizeObserver | undefined

const resolvedAppearance = computed<'dark' | 'light'>(() => {
  void minute.value
  return appearance.value === 'auto' ? (isDaytime() ? 'light' : 'dark') : appearance.value
})
const pageTheme = computed(() => screen.value === 'settings' ? 'now' : screen.value)
const musicPage = computed(() => pageTheme.value)
const secretActive = computed(() => secretMode.value && isWriter.value)
const shellClass = computed(() => [`theme-${pageTheme.value}`, resolvedAppearance.value, `nav-${navPosition.value}`, { 'secret-mode': secretActive.value, 'shell-enter': entering.value, 'drop-target': dropTarget.value }])
const indicatorStyle = computed(() => ({ '--x': `${indicator.value.x}px`, '--y': `${indicator.value.y}px`, '--w': `${indicator.value.w}px`, '--h': `${indicator.value.h}px` }))
const canGoBack = computed(() => screen.value !== 'past')
const canGoForward = computed(() => screen.value !== 'future')

function updateIndicator() {
  const active = nav.value?.querySelector<HTMLElement>('button.active')
  if (!active) { indicator.value = { ...indicator.value, w: 0, h: 0 }; return }
  indicator.value = { x: active.offsetLeft, y: active.offsetTop, w: active.offsetWidth, h: active.offsetHeight }
}
function navigate(next: Screen) {
  if (next === screen.value) return
  const current = pageTheme.value
  const target = next === 'settings' ? 'now' : next
  const apply = () => { screen.value = next }
  const doc = document as Document & { startViewTransition?: (callback: () => void) => { finished: Promise<void> } }
  if (doc.startViewTransition && current !== target && !reducedMotion()) {
    document.documentElement.dataset.transition = `from-${current}-to-${target}`
    doc.startViewTransition(apply).finished.finally(() => { delete document.documentElement.dataset.transition })
  } else apply()
}
function step(direction: -1 | 1) {
  if (screen.value === 'settings') { navigate('now'); return }
  const index = screens.findIndex(item => item.key === screen.value) + direction
  const target = screens[index]
  if (target) navigate(target.key)
}
function setAppearance(next: Appearance) { transitionTheme(shell.value ?? null, () => { appearance.value = next }) }
function toggleAppearance() { setAppearance(resolvedAppearance.value === 'dark' ? 'light' : 'dark') }
function toggleSecret() { transitionTheme(shell.value ?? null, () => { secretMode.value = !secretMode.value }) }
function onGripDrag(event: DragEvent) { event.dataTransfer?.setData('text/plain', 'topbar'); dropTarget.value = true }
function onDrop(event: DragEvent) {
  dropTarget.value = false
  if (event.dataTransfer?.getData('text/plain') !== 'topbar') return
  const x = event.clientX / window.innerWidth
  const y = event.clientY / window.innerHeight
  navPosition.value = y < .2 ? 'top' : y > .8 ? 'bottom' : x < .5 ? 'left' : 'right'
}
function onKey(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null
  if (target && (target.closest('input, textarea, select, [contenteditable="true"], .md-editor'))) return
  if (event.altKey && event.key === 'ArrowLeft') step(-1)
  if (event.altKey && event.key === 'ArrowRight') step(1)
}

watch(appearance, value => localStorage.setItem('cyberlife-theme', value))
watch(secretMode, value => localStorage.setItem('cyberlife-secret-mode', String(value)))
watch(navPosition, value => { localStorage.setItem('cyberlife-nav-position', value); nextTick(updateIndicator) })
watch(screen, () => nextTick(updateIndicator))
watch(() => authState.actor, actor => {
  if (!actor) { navObserver?.disconnect(); navObserver = undefined; return }
  if (authState.justLoggedIn) {
    entering.value = true
    if (enterTimer) clearTimeout(enterTimer)
    enterTimer = window.setTimeout(() => { entering.value = false; authState.justLoggedIn = false }, DUR.slow + 50)
  }
  nextTick(() => {
    updateIndicator()
    if (nav.value && 'ResizeObserver' in window) { navObserver = new ResizeObserver(updateIndicator); navObserver.observe(nav.value) }
  })
})
onMounted(() => {
  restoreSession()
  minuteTimer = window.setInterval(() => { minute.value = Date.now() }, 60_000)
  window.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => { if (minuteTimer) clearInterval(minuteTimer); if (enterTimer) clearTimeout(enterTimer); navObserver?.disconnect(); window.removeEventListener('keydown', onKey) })
</script>

<template>
  <div v-if="authState.loading" class="app-boot" aria-label="加载中"><i /></div>
  <AdminLoginView v-else-if="!authState.actor && isAdminPath" />
  <LoginView v-else-if="!authState.actor" />
  <div v-else ref="shell" class="app-shell" :class="shellClass" @dragover.prevent @dragleave="dropTarget = false" @drop.prevent="onDrop">
    <header class="topbar">
      <span class="topbar-grip" draggable="true" title="拖到屏幕边缘可改变位置" @dragstart="onGripDrag" @dragend="dropTarget = false"><AppIcon name="grip" :size="16" /></span>
      <div v-if="!isAdmin" class="topbar-center">
        <button class="topbar-arrow" :disabled="!canGoBack" aria-label="上一页" @click="step(-1)"><AppIcon name="chevron-left" :size="20" /></button>
        <nav ref="nav" class="screen-nav" aria-label="页面导航">
          <button v-for="item in screens" :key="item.key" v-glow :class="{ active: screen === item.key }" @click="navigate(item.key)">{{ item.label }}</button>
          <i class="nav-indicator" :style="indicatorStyle" />
        </nav>
        <button class="topbar-arrow" :disabled="!canGoForward" aria-label="下一页" @click="step(1)"><AppIcon name="chevron-right" :size="20" /></button>
      </div>
      <div class="topbar-tools">
        <MusicControl v-if="!isAdmin" :page="musicPage" />
        <button v-glow class="icon-button" :aria-label="resolvedAppearance === 'dark' ? '切换为亮色' : '切换为暗色'" @click="toggleAppearance">
          <span class="icon-morph"><AppIcon name="sun" :class="{ on: resolvedAppearance === 'light' }" /><AppIcon name="moon" :class="{ on: resolvedAppearance === 'dark' }" /></span>
        </button>
        <button v-if="!isAdmin" v-glow class="icon-button" :class="{ active: screen === 'settings' }" aria-label="设置" @click="navigate('settings')"><AppIcon name="gear" /></button>
        <NotificationCenter v-if="!isAdmin" />
        <button v-if="isWriter" v-glow class="icon-button secret-toggle" :class="{ active: secretMode }" :aria-label="secretMode ? '退出绝密模式' : '进入绝密模式'" :title="secretMode ? '绝密模式已开启' : '绝密模式'" @click="toggleSecret">
          <span class="icon-morph"><AppIcon name="lock" :class="{ on: secretMode }" /><AppIcon name="unlock" :class="{ on: !secretMode }" /></span>
        </button>
        <span class="actor-name">{{ roleLabel }}</span>
        <button v-glow class="text-button" aria-label="登出" @click="logout"><AppIcon name="logout" :size="16" /><span>登出</span></button>
      </div>
    </header>
    <div class="screen">
      <AdminView v-if="isAdmin" />
      <PastView v-else-if="screen === 'past'" />
      <FutureView v-else-if="screen === 'future'" />
      <SettingsView v-else-if="screen === 'settings'" :appearance="appearance" :nav-position="navPosition" @update:appearance="setAppearance" @update:nav-position="navPosition = $event" @logout="logout" />
      <NowView v-else-if="isWriter" :secret="secretActive" @navigate-future="navigate('future')" />
      <ReaderView v-else />
    </div>
  </div>
</template>
