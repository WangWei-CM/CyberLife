<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { authState, logout, restoreSession } from './stores/auth'
import LoginView from './views/LoginView.vue'
import AdminView from './views/AdminView.vue'
import NowView from './views/NowView.vue'
import ReaderView from './views/ReaderView.vue'
import PastView from './views/PastView.vue'
import FutureView from './views/FutureView.vue'
import SettingsView from './views/SettingsView.vue'
import MusicControl from './components/MusicControl.vue'
import NotificationCenter from './components/NotificationCenter.vue'

type Screen = 'past' | 'now' | 'future' | 'settings'
const screen = ref<Screen>('now')
const navPosition = ref<'top'|'left'|'right'|'bottom'>((localStorage.getItem('cyberlife-nav-position') as 'top'|'left'|'right'|'bottom') || 'top')
const appearance = ref<'dark' | 'light' | 'auto'>((localStorage.getItem('cyberlife-theme') as 'dark' | 'light' | 'auto') || 'dark')
const secretMode = ref(localStorage.getItem('cyberlife-secret-mode') === 'true')
const isAdmin = computed(() => authState.actor?.type === 'admin')
const screenTheme = computed(() => `theme-${screen.value === 'settings' ? 'now' : screen.value}`)
const resolvedAppearance = computed(() => appearance.value === 'auto' ? (new Date().getHours() >= 7 && new Date().getHours() < 18 ? 'light' : 'dark') : appearance.value)
function navigate(next: Screen) {
  if (next === screen.value) return
  const apply = () => { screen.value = next }
  if ('startViewTransition' in document && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
    ;(document as Document & { startViewTransition?: (callback: () => void) => void }).startViewTransition?.(apply)
  } else apply()
}
function setAppearance(next: 'dark' | 'light' | 'auto') { appearance.value = next }
function setNavPosition(event: DragEvent) { const x=event.clientX/window.innerWidth; const y=event.clientY/window.innerHeight; navPosition.value=y<.2?'top':y>.8?'bottom':x<.5?'left':'right'; localStorage.setItem('cyberlife-nav-position',navPosition.value) }
watch(appearance, value => localStorage.setItem('cyberlife-theme', value))
watch(secretMode, value => localStorage.setItem('cyberlife-secret-mode',String(value)))
onMounted(restoreSession)
</script>

<template>
  <div v-if="authState.loading" class="app-boot" aria-label="加载中"><i /></div>
  <LoginView v-else-if="!authState.actor" />
  <div v-else class="app-shell" :class="[screenTheme, resolvedAppearance, `nav-${navPosition}`, { 'secret-mode': secretMode }]" @dragover.prevent @drop="setNavPosition">
    <header class="topbar" draggable="true">
      <button class="topbar-arrow" aria-label="上一页" @click="navigate(screen === 'now' ? 'past' : 'now')">‹</button>
      <nav class="screen-nav" aria-label="页面导航">
        <button :class="{ active: screen === 'past' }" @click="navigate('past')">过去</button>
        <button :class="{ active: screen === 'now' }" @click="navigate('now')">现在</button>
        <button :class="{ active: screen === 'future' }" @click="navigate('future')">未来</button>
      </nav>
      <button class="topbar-arrow" aria-label="下一页" @click="navigate(screen === 'now' ? 'future' : 'now')">›</button>
      <div class="topbar-tools">
        <MusicControl :page="screen === 'settings' ? 'now' : screen" />
        <button class="icon-button" :aria-label="resolvedAppearance === 'dark' ? '切换亮色' : '切换暗色'" @click="setAppearance(resolvedAppearance === 'dark' ? 'light' : 'dark')">◐</button>
        <button class="icon-button" aria-label="设置" @click="navigate('settings')">⚙</button>
        <NotificationCenter v-if="!isAdmin" />
        <button v-if="authState.actor.type === 'writer'" class="secret-toggle" :class="{ active: secretMode }" @click="secretMode = !secretMode">⌁</button>
        <span class="actor-name">{{ authState.actor.type === 'admin' ? '管理员' : authState.actor.type === 'writer' ? '书写者' : '阅读者' }}</span>
        <button class="text-button" @click="logout">登出</button>
      </div>
    </header>
    <AdminView v-if="isAdmin" />
    <PastView v-else-if="screen === 'past'" />
    <FutureView v-else-if="screen === 'future'" />
    <SettingsView v-else-if="screen === 'settings'" :appearance="appearance" @update:appearance="setAppearance" />
    <NowView v-else-if="authState.actor.type === 'writer'" />
    <ReaderView v-else />
  </div>
</template>