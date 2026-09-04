<script setup lang="ts">
import { computed, onMounted } from 'vue'
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
import { ref } from 'vue'
onMounted(restoreSession)
const screen=ref<'past'|'now'|'future'|'settings'>('now')
const isAdmin=computed(()=>authState.actor?.type==='admin')
</script>
<template><div v-if="authState.loading" class="splash">CYBERLIFE 正在恢复会话…</div><LoginView v-else-if="!authState.actor"/><template v-else><nav><b>CYBERLIFE</b><button v-if="!isAdmin" :class="{active:screen==='past'}" @click="screen='past'">过去</button><button v-if="!isAdmin" :class="{active:screen==='now'}" @click="screen='now'">现在</button><button v-if="!isAdmin" :class="{active:screen==='future'}" @click="screen='future'">未来</button><button v-if="!isAdmin" :class="{active:screen==='settings'}" @click="screen='settings'">设置</button><MusicControl v-if="!isAdmin"/><NotificationCenter v-if="!isAdmin"/><span>{{authState.actor.type==='admin'?'系统管理员':authState.actor.type==='writer'?'书写者':'阅读者'}}</span><button @click="logout">登出</button></nav><AdminView v-if="isAdmin"/><template v-else-if="screen==='past'"><PastView/></template><template v-else-if="screen==='future'"><FutureView/></template><template v-else-if="screen==='settings'"><SettingsView/></template><template v-else-if="authState.actor.type==='writer'"><NowView/></template><template v-else><ReaderView/></template></template></template>