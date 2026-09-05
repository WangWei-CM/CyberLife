<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { api } from '../api/client'
import { signIn } from '../stores/auth'
import { greetingForHour } from '../lib/dates'
import { DUR, wait } from '../lib/motion'
import AppIcon from '../components/AppIcon.vue'

/** 问候语时序（毫秒），可配置：淡入 / 停留 / 淡出。 */
const GREETING = { fade: 500, hold: 1000 }

const credential = ref('')
const error = ref('')
const busy = ref(false)
const phase = ref<'idle' | 'leaving' | 'greeting'>('idle')
const greeting = ref('')
const greetingVisible = ref(false)

async function submit() {
  const key = credential.value.trim()
  if (!key || busy.value) return
  error.value = ''
  busy.value = true
  try {
    const result = await api.keyLogin(key)
    credential.value = ''
    // 1. logo 放大淡出 + 2. 输入框透明（同时，0.5s）
    phase.value = 'leaving'
    await wait(DUR.slow - 100)
    // 3. 问候语：淡入 → 停留 → 淡出
    greeting.value = `${greetingForHour()}，${result.actor.type === 'writer' ? '书写者' : '阅读者'}`
    phase.value = 'greeting'
    await nextTick()
    requestAnimationFrame(() => { greetingVisible.value = true })
    await wait(GREETING.fade + GREETING.hold)
    greetingVisible.value = false
    await wait(GREETING.fade)
    // 4. 现在页以页面正中为原点缩放淡入（App.vue 的 shell-enter）
    signIn(result.actor)
  } catch (cause) {
    phase.value = 'idle'
    error.value = cause instanceof Error ? cause.message : '密钥无效'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="login-page" :class="`phase-${phase}`" :style="{ '--greet-fade': `${GREETING.fade}ms` }">
    <section v-if="phase !== 'greeting'" class="login-stage">
      <h1 class="glitch-logo" data-text="CYBERLIFE">CYBERLIFE</h1>
      <form class="login-form" @submit.prevent="submit">
        <label class="login-key">
          <span class="visually-hidden">密钥</span>
          <input v-model="credential" class="login-key-input" type="password" autocomplete="current-password" spellcheck="false" placeholder="粘贴主密钥或阅读密钥" :disabled="busy" autofocus />
          <i class="login-key-ring" aria-hidden="true" />
        </label>
        <button v-glow class="login-submit" type="submit" aria-label="登录" :disabled="busy || !credential.trim()"><AppIcon name="chevron-right" :size="20" /></button>
      </form>
      <p class="login-error" role="alert" aria-live="polite">{{ error }}</p>
    </section>
    <p v-else class="login-greeting" :class="{ show: greetingVisible }">{{ greeting }}</p>
  </main>
</template>
