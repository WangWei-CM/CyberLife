<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, type Playlist, type PlaylistMode, type PlaylistPage } from '../api/client'

const props = defineProps<{ page: PlaylistPage }>()
type PageState = { index: number; time: number }
const audio = ref<HTMLAudioElement>()
const playing = ref(false)
const volume = ref(Number(localStorage.getItem('cyberlife-volume') || 70))
const mode = ref<PlaylistMode>('list')
const playlists = ref<Record<PlaylistPage, Playlist | null>>({ now: null, past: null, future: null })
const states = ref<Record<PlaylistPage, PageState>>(JSON.parse(localStorage.getItem('cyberlife-play-states') || '{"now":{"index":0,"time":0},"past":{"index":0,"time":0},"future":{"index":0,"time":0}}'))
let context: AudioContext | undefined
let oscillator: OscillatorNode | undefined
let gain: GainNode | undefined

const playlist = computed(() => playlists.value[props.page])
const tracks = computed(() => playlist.value?.tracks ?? [])
const current = computed(() => tracks.value[states.value[props.page]?.index || 0] ?? null)
const usingDefault = computed(() => !current.value && (playlist.value?.defaultEnabled ?? true))
function persist() { localStorage.setItem('cyberlife-volume', String(volume.value)); localStorage.setItem('cyberlife-play-states', JSON.stringify(states.value)) }
async function load() {
  try {
    const items = (await api.musicPlaylists()).items
    playlists.value = { now: null, past: null, future: null }
    for (const item of items) playlists.value[item.page] = item
    if (playlist.value) mode.value = playlist.value.mode
    sync()
  } catch { /* The player remains usable with synthesized defaults offline. */ }
}
function stopDefault() { oscillator?.stop(); oscillator = undefined; gain = undefined }
function playDefault() {
  stopDefault()
  context ??= new AudioContext()
  const frequencies: Record<PlaylistPage, number> = { now: 261.63, past: 196, future: 329.63 }
  oscillator = context.createOscillator(); gain = context.createGain()
  oscillator.type = props.page === 'future' ? 'sawtooth' : props.page === 'past' ? 'triangle' : 'sine'
  oscillator.frequency.value = frequencies[props.page]
  gain.gain.value = Math.min(.08, volume.value / 1000)
  oscillator.connect(gain).connect(context.destination); oscillator.start()
}
function sync() {
  stopDefault()
  if (!audio.value) return
  audio.value.volume = volume.value / 100
  if (!current.value) { audio.value.removeAttribute('src'); audio.value.load(); if (playing.value && usingDefault.value) playDefault(); return }
  audio.value.src = current.value.url
  audio.value.currentTime = states.value[props.page]?.time || 0
  if (playing.value) audio.value.play().catch(() => { playing.value = false })
}
function toggle() { playing.value = !playing.value; if (playing.value) sync(); else { audio.value?.pause(); stopDefault() } }
function onTime() { if (audio.value) { states.value[props.page] = { index: states.value[props.page]?.index || 0, time: audio.value.currentTime }; persist() } }
function onEnded() { if (usingDefault.value) { playDefault(); return }; if (!tracks.value.length) return; let index = states.value[props.page]?.index || 0; if (mode.value === 'random') index = Math.floor(Math.random() * tracks.value.length); else if (mode.value === 'list') index = (index + 1) % tracks.value.length; states.value[props.page] = { index, time: 0 }; sync() }
async function saveMode() { if (!playlist.value) return; try { await api.replaceMusicPlaylist(props.page, { name: playlist.value.name, mode: mode.value, volume: volume.value, default_enabled: playlist.value.defaultEnabled }) } catch { /* Playback mode remains effective for this session. */ } }
function refresh() { load() }
watch(() => props.page, sync)
watch(volume, () => { if (audio.value) audio.value.volume = volume.value / 100; persist() })
onMounted(() => { window.addEventListener('cyberlife-playlist-change', refresh); load() })
onBeforeUnmount(() => { window.removeEventListener('cyberlife-playlist-change', refresh); audio.value?.pause(); stopDefault(); context?.close() })
</script>

<template><div class="music-control"><audio ref="audio" @timeupdate="onTime" @ended="onEnded" /><button :disabled="!current && !usingDefault" @click="toggle">{{ playing ? '停止' : '播放' }}</button><select v-model="mode" aria-label="播放模式" @change="saveMode"><option value="list">循环</option><option value="random">随机</option><option value="single">单曲</option></select><label>音量 <input v-model.number="volume" type="range" min="0" max="100" /></label></div></template>
