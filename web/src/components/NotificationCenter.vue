<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type Notice } from '../api/client'
const notices=ref<Notice[]>([])
const loading=ref(false)
async function load(){loading.value=true;try{notices.value=(await api.notifications()).items}catch{notices.value=[]}finally{loading.value=false}}
onMounted(load)
const open=ref(false)
const unread=computed(()=>notices.value.filter(x=>!x.read).length)
async function markAll(){for(const notice of notices.value.filter(x=>!x.read)){await api.markNotificationRead(notice.id)};await load()}
function toggle(){open.value=!open.value}
function close(){open.value=false}
defineExpose({toggle,close})
</script>
<template><div class="notice-wrap"><button class="notice-button glow-spot text-glow" aria-label="通知中心" @click="toggle">铃铛<span v-if="unread" class="notice-dot">{{unread>99?'99+':unread}}</span></button><section v-if="open" class="notice-popover"><header><b>通知中心</b><button @click="markAll">全部已读</button></header><p v-if="!notices.length" class="empty">暂无通知</p><article v-for="notice in notices" :key="notice.id" :class="{unread:!notice.read}"><b>{{notice.type}}</b><p>{{notice.text}}</p><small>{{notice.createdAt}}</small></article></section></div></template>