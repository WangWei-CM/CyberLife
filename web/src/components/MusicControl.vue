<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, type Playlist, type PlaylistMode, type PlaylistPage } from '../api/client'
import { isWriter } from '../stores/auth'
import AppIcon from './AppIcon.vue'
import SegmentedControl from './SegmentedControl.vue'

const props = defineProps<{ page: PlaylistPage }>()
type PageState = { index: number; time: number }
const audio = ref<HTMLAudioElement>()
const playing = ref(false)
const open = ref(false)
const root = ref<HTMLElement>()
const volume = ref(Number(localStorage.getItem('cyberlife-volume') || 70))
const mode = ref<PlaylistMode>('list')
const playlists = ref<Record<PlaylistPage, Playlist | null>>({ now: null, past: null, future: null })
const states = ref<Record<PlaylistPage, PageState>>(JSON.parse(localStorage.getItem('cyberlife-play-states') || '{"now":{"index":0,"time":0},"past":{"index":0,"time":0},"future":{"index":0,"time":0}}'))
let context: AudioContext | undefined
let defaultNodes: { stop: () => void } | undefined

const playlist = computed(() => playlists.value[props.page])
const tracks = computed(() => playlist.value?.tracks ?? [])
const current = computed(() => tracks.value[states.value[props.page]?.index || 0] ?? null)
const usingDefault = computed(() => !current.value && (playlist.value?.defaultEnabled ?? true))
const pageLabel = computed(() => props.page === 'now' ? '现在' : props.page === 'past' ? '过去' : '未来')
const trackTitle = computed(() => current.value ? current.value.title : usingDefault.value ? `${pageLabel.value} · 默认音频` : '歌单为空')
const modeOptions = [{ value: 'list', label: '循环' }, { value: 'random', label: '随机' }, { value: 'single', label: '单曲' }]

function persist() { localStorage.setItem('cyberlife-volume', String(volume.value)); localStorage.setItem('cyberlife-play-states', JSON.stringify(states.value)) }
async function load() {
  try {
    const items = (await api.musicPlaylists()).items
    playlists.value = { now: null, past: null, future: null }
    for (const item of items) playlists.value[item.page] = item
    if (playlist.value) mode.value = playlist.value.mode
    sync()
  } catch { /* 阅读者或离线时用合成默认音频 */ }
}
function stopDefault() { defaultNodes?.stop(); defaultNodes = undefined }
/** 内置默认音频：Web Audio 合成的柔和垫音，三页音色与和声不同。 */
function playDefault() {
  stopDefault()
  context ??= new AudioContext()
  if (context.state === 'suspended') context.resume().catch(() => undefined)
  const chords: Record<PlaylistPage, number[]> = { now: [261.63, 329.63, 392.0], past: [196.0, 246.94, 293.66], future: [220.0, 277.18, 329.63] }
  const wave: Record<PlaylistPage, OscillatorType> = { now: 'sine', past: 'triangle', future: 'sawtooth' }
  const master = context.createGain()
  master.gain.value = 0
  master.gain.linearRampToValueAtTime(Math.min(.12, volume.value / 600), context.currentTime + 1.2)
  const filter = context.createBiquadFilter()
  filter.type = 'lowpass'
  filter.frequency.value = props.page === 'future' ? 900 : 1800
  const lfo = context.createOscillator()
  const lfoGain = context.createGain()
  lfo.frequency.value = props.page === 'past' ? .08 : .14
  lfoGain.gain.value = props.page === 'future' ? 500 : 250
  lfo.connect(lfoGain).connect(filter.frequency)
  const oscillators = chords[props.page].flatMap(frequency => [0, 3].map(detune => {
    const oscillator = context!.createOscillator()
    oscillator.type = wave[props.page]
    oscillator.frequency.value = frequency
    oscillator.detune.value = detune
    oscillator.connect(filter)
    oscillator.start()
    return oscillator
  }))
  filter.connect(master).connect(context.destination)
  lfo.start()
  const ctx = context
  defaultNodes = { stop: () => { master.gain.cancelScheduledValues(ctx.currentTime); master.gain.linearRampToValueAtTime(0, ctx.currentTime + .3); setTimeout(() => { oscillators.forEach(item => item.stop()); lfo.stop() }, 350) } }
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
function onEnded() {
  if (usingDefault.value) { playDefault(); return }
  if (!tracks.value.length) return
  let index = states.value[props.page]?.index || 0
  if (mode.value === 'random') index = Math.floor(Math.random() * tracks.value.length)
  else if (mode.value === 'list') index = (index + 1) % tracks.value.length
  states.value[props.page] = { index, time: 0 }
  sync()
}
function skip(direction: 1 | -1) {
  if (!tracks.value.length) return
  const index = ((states.value[props.page]?.index || 0) + direction + tracks.value.length) % tracks.value.length
  states.value[props.page] = { index, time: 0 }
  persist()
  sync()
}
async function saveMode(next: string) {
  mode.value = next as PlaylistMode
  if (!playlist.value || !isWriter.value) return
  try { await api.replaceMusicPlaylist(props.page, { name: playlist.value.name, mode: mode.value, volume: volume.value, default_enabled: playlist.value.defaultEnabled }) } catch { /* 本会话内仍然生效 */ }
}
function onDocumentClick(event: MouseEvent) { if (open.value && root.value && !root.value.contains(event.target as Node)) open.value = false }
function refresh() { load() }
watch(() => props.page, sync)
watch(volume, () => { if (audio.value) audio.value.volume = volume.value / 100; persist(); if (defaultNodes) { stopDefault(); if (playing.value && usingDefault.value) playDefault() } })
onMounted(() => { window.addEventListener('cyberlife-playlist-change', refresh); document.addEventListener('click', onDocumentClick); load() })
onBeforeUnmount(() => { window.removeEventListener('cyberlife-playlist-change', refresh); document.removeEventListener('click', onDocumentClick); audio.value?.pause(); stopDefault(); context?.close() })
</script>

<template>
  <div ref="root" class="music-control" :class="{ playing }">
    <audio ref="audio" @timeupdate="onTime" @ended="onEnded" />
    <button v-glow class="icon-button music-toggle" :disabled="!current && !usingDefault" :aria-label="playing ? '停止播放' : '开始播放'" @click="toggle">
      <span class="icon-morph"><AppIcon name="play" :class="{ on: !playing }" /><AppIcon name="stop" :class="{ on: playing }" /></span>
    </button>
    <button v-glow class="icon-button music-more" :class="{ active: open }" aria-label="播放设置" :aria-expanded="open" @click="open = !open"><AppIcon name="music" /></button>
    <Transition name="popover">
      <section v-if="open" class="popover music-popover" aria-label="播放设置">
        <div class="music-now">
          <span class="music-status" :class="{ live: playing }" aria-hidden="true" />
          <div class="marquee" :class="{ run: playing && trackTitle.length > 14 }"><span>{{ trackTitle }}</span><span v-if="playing && trackTitle.length > 14" aria-hidden="true">{{ trackTitle }}</span></div>
          <div v-if="tracks.length > 1" class="music-skip">
            <button class="icon-button" aria-label="上一首" @click="skip(-1)"><AppIcon name="chevron-left" :size="16" /></button>
            <button class="icon-button" aria-label="下一首" @click="skip(1)"><AppIcon name="chevron-right" :size="16" /></button>
          </div>
        </div>
        <SegmentedControl :model-value="mode" :options="modeOptions" aria-label="播放模式" @update:model-value="saveMode" />
        <label class="music-volume">
          <AppIcon name="volume" :size="16" />
          <input v-model.number="volume" type="range" min="0" max="100" aria-label="音量" :style="{ '--range-fill': `${volume}%` }" />
          <b class="mono">{{ volume }}%</b>
        </label>
      </section>
    </Transition>
  </div>
</template>
