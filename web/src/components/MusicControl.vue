<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
const props=defineProps<{page:'past'|'now'|'future'}>()
type PageState={index:number;time:number}
const audio=ref<HTMLAudioElement>();const playing=ref(false);const volume=ref(Number(localStorage.getItem('cyberlife-volume')||70));const mode=ref<'list'|'random'|'single'>((localStorage.getItem('cyberlife-play-mode') as 'list'|'random'|'single')||'list');const tracks=ref<Record<string,string[]>>(JSON.parse(localStorage.getItem('cyberlife-playlists')||'{"now":[],"past":[],"future":[]}'));const states=ref<Record<string,PageState>>(JSON.parse(localStorage.getItem('cyberlife-play-states')||'{"now":{"index":0,"time":0},"past":{"index":0,"time":0},"future":{"index":0,"time":0}}'))
const current=computed(()=>tracks.value[props.page]?.[states.value[props.page]?.index||0]||'')
function persist(){localStorage.setItem('cyberlife-volume',String(volume.value));localStorage.setItem('cyberlife-play-mode',mode.value);localStorage.setItem('cyberlife-play-states',JSON.stringify(states.value))}
function sync(){if(!audio.value)return;audio.value.volume=volume.value/100;if(!current.value){audio.value.removeAttribute('src');return};audio.value.src=current.value;audio.value.currentTime=states.value[props.page]?.time||0;if(playing.value)audio.value.play().catch(()=>{playing.value=false})}
function toggle(){playing.value=!playing.value;if(playing.value)sync();else audio.value?.pause()}
function onTime(){if(audio.value){states.value[props.page]={index:states.value[props.page]?.index||0,time:audio.value.currentTime};persist()}}
function onEnded(){const list=tracks.value[props.page]||[];if(!list.length)return;let index=states.value[props.page]?.index||0;if(mode.value==='random')index=Math.floor(Math.random()*list.length);else if(mode.value==='list')index=(index+1)%list.length;states.value[props.page]={index,time:0};sync()}
function refreshTracks(){tracks.value=JSON.parse(localStorage.getItem('cyberlife-playlists')||'{"now":[],"past":[],"future":[]}');sync()}
watch(()=>props.page,()=>sync());watch(volume,()=>{if(audio.value)audio.value.volume=volume.value/100;persist()})
onMounted(()=>window.addEventListener('cyberlife-playlist-change',refreshTracks));onBeforeUnmount(()=>{window.removeEventListener('cyberlife-playlist-change',refreshTracks);audio.value?.pause()})
</script>
<template><div class="music-control"><audio ref="audio" @timeupdate="onTime" @ended="onEnded"/><button :disabled="!current" @click="toggle">{{playing?'停止':'播放'}}</button><select v-model="mode" aria-label="播放模式" @change="persist"><option value="list">循环</option><option value="random">随机</option><option value="single">单曲</option></select><label>音量 <input v-model.number="volume" type="range" min="0" max="100"/></label></div></template>