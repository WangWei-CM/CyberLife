<script setup lang="ts">
import { computed, ref } from 'vue'
type Notice={id:string;type:string;text:string;createdAt:string;read:boolean}
const notices=ref<Notice[]>(JSON.parse(localStorage.getItem('cyberlife-notices')||'[]'))
const open=ref(false)
const unread=computed(()=>notices.value.filter(x=>!x.read).length)
function markAll(){notices.value=notices.value.map(x=>({...x,read:true}));persist()}
function persist(){localStorage.setItem('cyberlife-notices',JSON.stringify(notices.value))}
function toggle(){open.value=!open.value}
function close(){open.value=false}
defineExpose({toggle,close})
</script>
<template><div class="notice-wrap"><button class="notice-button" aria-label="通知中心" @click="toggle">铃铛<span v-if="unread" class="notice-dot">{{unread>99?'99+':unread}}</span></button><section v-if="open" class="notice-popover"><header><b>通知中心</b><button @click="markAll">全部已读</button></header><p v-if="!notices.length" class="empty">暂无通知</p><article v-for="notice in notices" :key="notice.id" :class="{unread:!notice.read}"><b>{{notice.type}}</b><p>{{notice.text}}</p><small>{{notice.createdAt}}</small></article></section></div></template>