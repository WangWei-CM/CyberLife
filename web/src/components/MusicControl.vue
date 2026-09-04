<script setup lang="ts">
import { computed, ref } from 'vue'
const playing=ref(false);const volume=ref(Number(localStorage.getItem('cyberlife-volume')||70));const page=ref<'now'|'past'|'future'>('now');const tracks=ref<Record<string,string[]>>(JSON.parse(localStorage.getItem('cyberlife-playlists')||'{"now":[],"past":[],"future":[]}'))
const current=computed(()=>tracks.value[page.value]?.[0]||'未选择曲目')
function toggle(){playing.value=!playing.value}function save(){localStorage.setItem('cyberlife-volume',String(volume.value))}
defineExpose({setPage:(v:'now'|'past'|'future')=>page.value=v})
</script>
<template><div class="music-control"><button @click="toggle">{{playing?'停止':'播放'}} · {{current}}</button><label>音量 <input v-model.number="volume" type="range" min="0" max="100" @change="save"/> {{volume}}%</label></div></template>