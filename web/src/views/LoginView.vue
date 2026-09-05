<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api/client'
import { authState } from '../stores/auth'
const credential = ref('')
const error = ref('')
const busy = ref(false)
const phase = ref<'idle' | 'leaving' | 'greeting'>('idle')
const greeting = ref('')
function greetingForNow() { const hour = new Date().getHours(); return hour < 6 ? '凌晨好' : hour < 9 ? '早上好' : hour < 12 ? '上午好' : hour < 14 ? '中午好' : hour < 18 ? '下午好' : '晚上好' }
async function submit() {
  if (!credential.value.trim() || busy.value) return
  error.value = ''; busy.value = true
  try {
    const result = await api.keyLogin(credential.value.trim())
    phase.value = 'leaving'
    await new Promise<void>(resolve => setTimeout(resolve, matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 500))
    greeting.value = `${greetingForNow()}，${result.actor.type === 'writer' ? '书写者' : '阅读者'}`
    phase.value = 'greeting'
    await new Promise<void>(resolve => setTimeout(resolve, matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 2000))
    authState.actor = result.actor
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '密钥无效' }
  finally { busy.value = false; credential.value = '' }
}
</script>
<template>
  <main class="login-page" :class="`login-${phase}`">
    <section v-if="phase !== 'greeting'" class="login-form" @submit.prevent="submit">
      <h1 class="glitch-logo" data-text="CYBERLIFE">CYBERLIFE</h1>
      <form><input class="login-key-input" v-model="credential" autocomplete="current-password" placeholder="粘贴主密钥或阅读密钥" type="password" :disabled="busy" autofocus /><button class="login-submit glow-spot" aria-label="登录" :disabled="busy">进入</button></form>
      <p v-if="error" class="login-error" role="alert">{{ error }}</p>
    </section>
    <p v-else class="login-greeting">{{ greeting }}</p>
  </main>
</template>