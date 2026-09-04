<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api/client'
import { authState } from '../stores/auth'
const mode = ref<'key' | 'admin'>('key'); const credential = ref(''); const error = ref(''); const busy = ref(false)
async function submit() { error.value=''; busy.value=true; try { authState.actor = mode.value === 'key' ? (await api.keyLogin(credential.value)).actor : (await api.adminLogin(credential.value)).actor; credential.value='' } catch (cause) { error.value = cause instanceof Error ? cause.message : '登录失败' } finally { busy.value=false } }
</script>
<template><main class="login"><section class="login-card"><p class="eyebrow">CYBERLIFE / PHASE 1</p><h1 data-text="CYBERLIFE" class="glitch">CYBERLIFE</h1><p>以密钥进入一段人生。</p><div class="mode"><button :class="{active:mode==='key'}" @click="mode='key'">人生密钥</button><button :class="{active:mode==='admin'}" @click="mode='admin'">管理员</button></div><form @submit.prevent="submit"><label>{{ mode==='key' ? '主密钥或阅读密钥' : '管理员密码' }}<input v-model="credential" :type="mode==='admin'?'password':'password'" autocomplete="current-password" required :disabled="busy" /></label><p v-if="error" class="error" role="alert">{{ error }}</p><button class="primary" :disabled="busy">{{ busy ? '验证中…' : '进入' }}</button></form></section></main></template>