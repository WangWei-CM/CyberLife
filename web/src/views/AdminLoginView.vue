<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api/client'
import { authState } from '../stores/auth'

const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  if (!password.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    authState.actor = (await api.adminLogin(password.value)).actor
    window.history.replaceState({}, '', '/')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '管理员凭证无效'
  } finally {
    busy.value = false
    password.value = ''
  }
}
</script>

<template>
  <main class="admin-login-page">
    <form class="admin-login-form" @submit.prevent="submit">
      <h1>管理员登录</h1>
      <input v-model="password" type="password" autocomplete="current-password" placeholder="管理员密码" :disabled="busy" autofocus />
      <button class="primary" :disabled="busy">登录</button>
      <p v-if="error" class="login-error" role="alert">{{ error }}</p>
    </form>
  </main>
</template>
