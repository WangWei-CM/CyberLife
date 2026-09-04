<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { authState, logout, restoreSession } from './stores/auth'
import LoginView from './views/LoginView.vue'
import AdminView from './views/AdminView.vue'
onMounted(restoreSession)
const isAdmin=computed(()=>authState.actor?.type==='admin')
</script>
<template><div v-if="authState.loading" class="splash">CYBERLIFE 正在恢复会话…</div><LoginView v-else-if="!authState.actor"/><template v-else><nav><b>CYBERLIFE</b><span>{{authState.actor.type==='admin'?'系统管理员':authState.actor.type==='writer'?'书写者':'阅读者'}}</span><button @click="logout">登出</button></nav><AdminView v-if="isAdmin"/><main v-else class="welcome"><p class="eyebrow">PHASE 1 READY</p><h1>欢迎回来</h1><p>身份验证、会话与人生空间已就绪。Now、Past、Future 的内容功能将在下一阶段接入。</p></main></template></template>