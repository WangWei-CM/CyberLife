<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { authState, logout, restoreSession } from './stores/auth'
import LoginView from './views/LoginView.vue'
import AdminView from './views/AdminView.vue'
import NowView from './views/NowView.vue'
import ReaderView from './views/ReaderView.vue'
onMounted(restoreSession)
const isAdmin=computed(()=>authState.actor?.type==='admin')
</script>
<template><div v-if="authState.loading" class="splash">CYBERLIFE 正在恢复会话…</div><LoginView v-else-if="!authState.actor"/><template v-else><nav><b>CYBERLIFE</b><span>{{authState.actor.type==='admin'?'系统管理员':authState.actor.type==='writer'?'书写者':'阅读者'}}</span><button @click="logout">登出</button></nav><AdminView v-if="isAdmin"/><NowView v-else-if="authState.actor.type==='writer'"/><ReaderView v-else/></template></template>