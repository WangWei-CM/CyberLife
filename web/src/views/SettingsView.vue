<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type Playlist, type PlaylistMode, type PlaylistPage, type Preset, type PresetRule, type ReaderKey } from '../api/client'
import { isWriter } from '../stores/auth'
import { ui } from '../stores/ui'
import SegmentedControl from '../components/SegmentedControl.vue'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'
import type { Appearance } from '../App.vue'

const props = defineProps<{ appearance: Appearance; navPosition: 'top' | 'left' | 'right' | 'bottom' }>()
const emit = defineEmits<{ (event: 'update:appearance', value: Appearance): void; (event: 'update:navPosition', value: 'top' | 'left' | 'right' | 'bottom'): void; (event: 'logout'): void }>()

type Tab = 'music' | 'presets' | 'theme' | 'keys'
const tabs = computed(() => isWriter.value
  ? [{ value: 'music', label: '歌单' }, { value: 'presets', label: '权限预设' }, { value: 'theme', label: '主题' }, { value: 'keys', label: '密钥' }]
  : [{ value: 'theme', label: '主题' }, { value: 'music', label: '音乐' }])
const tab = ref<Tab>((ui.pendingSettingsTab as Tab) || (isWriter.value ? 'music' : 'theme'))
ui.pendingSettingsTab = ''
const error = ref('')
const volume = ref(Number(localStorage.getItem('cyberlife-volume') || 70))
const carouselSeconds = ref(Math.round(Number(localStorage.getItem('now-plan-carousel-ms') || 6000) / 1000))
const page = ref<PlaylistPage>('now')
const playlists = ref<Record<PlaylistPage, Playlist | null>>({ now: null, past: null, future: null })
const uploading = ref(false)
const presets = ref<Preset[]>([])
const readerKeys = ref<ReaderKey[]>([])
const selectedPreset = ref<Preset | null>(null)
const draftRules = ref<Record<string, boolean>>({})
const presetName = ref('')
const busy = ref(false)

const appearanceOptions = [{ value: 'dark', label: '始终黑色' }, { value: 'light', label: '始终白色' }, { value: 'auto', label: '日出日落' }]
const navOptions = [{ value: 'top', label: '顶部' }, { value: 'left', label: '左侧' }, { value: 'right', label: '右侧' }, { value: 'bottom', label: '底部' }]
const pageOptions = [{ value: 'now', label: '现在' }, { value: 'past', label: '过去' }, { value: 'future', label: '未来' }]
const modeOptions = [{ value: 'list', label: '列表循环' }, { value: 'random', label: '随机' }, { value: 'single', label: '单曲循环' }]
const playlist = computed(() => playlists.value[page.value])
const tracks = computed(() => playlist.value?.tracks ?? [])
const defaultEnabled = computed(() => playlist.value?.defaultEnabled ?? true)

function fail(cause: unknown, fallback: string) { error.value = cause instanceof Error ? cause.message : fallback }
async function loadPlaylists() {
  if (!isWriter.value) return
  try { const items = (await api.musicPlaylists()).items; playlists.value = { now: null, past: null, future: null }; for (const item of items) playlists.value[item.page] = item; window.dispatchEvent(new Event('cyberlife-playlist-change')) } catch (cause) { fail(cause, '读取歌单失败') }
}
async function updatePlaylist(patch: { mode?: PlaylistMode; default_enabled?: boolean }) {
  const current = playlist.value
  if (!current) return
  try { await api.replaceMusicPlaylist(page.value, { name: current.name, mode: patch.mode ?? current.mode, volume: current.volume, default_enabled: patch.default_enabled ?? current.defaultEnabled }); await loadPlaylists() } catch (cause) { fail(cause, '更新失败') }
}
async function addTrack(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (file.size > 20 * 1024 * 1024) { error.value = '音频不能超过 20MB'; input.value = ''; return }
  if (tracks.value.length >= 50) { error.value = '每页最多 50 首'; input.value = ''; return }
  uploading.value = true
  try { await api.uploadMusicTrack(page.value, file); await loadPlaylists() } catch (cause) { fail(cause, '上传失败') } finally { uploading.value = false; input.value = '' }
}
async function removeTrack(id: string) { try { await api.deleteMusicTrack(id); await loadPlaylists() } catch (cause) { fail(cause, '删除失败') } }
function saveVolume() { localStorage.setItem('cyberlife-volume', String(volume.value)); window.dispatchEvent(new Event('cyberlife-playlist-change')) }
function saveCarousel() { localStorage.setItem('now-plan-carousel-ms', String(Math.max(2, carouselSeconds.value) * 1000)) }
async function loadPresets() {
  if (!isWriter.value) return
  try { const [presetResult, keyResult] = await Promise.all([api.presets(), api.readerKeysForWriter()]); presets.value = presetResult.items; readerKeys.value = keyResult.items; if (selectedPreset.value) selectPreset(presets.value.find(item => item.id === selectedPreset.value?.id) ?? null) } catch (cause) { fail(cause, '读取权限预设失败') }
}
function selectPreset(preset: Preset | null) {
  selectedPreset.value = preset
  const rules: Record<string, boolean> = {}
  for (const key of readerKeys.value) rules[key.id] = preset?.rules.find(rule => rule.readerKeyId === key.id)?.allowed ?? false
  draftRules.value = rules
}
function rulesFromDraft(): PresetRule[] { return Object.entries(draftRules.value).map(([readerKeyId, allowed]) => ({ readerKeyId, allowed })) }
async function saveRules() { if (!selectedPreset.value || busy.value) return; busy.value = true; try { await api.replacePresetRules(selectedPreset.value.id, rulesFromDraft()); await loadPresets() } catch (cause) { fail(cause, '保存失败') } finally { busy.value = false } }
async function createPreset() { const name = presetName.value.trim(); if (!name || busy.value) return; busy.value = true; try { const preset = await api.createPreset(name, []); presetName.value = ''; await loadPresets(); selectPreset(presets.value.find(item => item.id === preset.id) ?? preset) } catch (cause) { fail(cause, '创建失败') } finally { busy.value = false } }
function keyStatus(key: ReaderKey) { if (key.revoked_at) return '已作废'; if (key.expires_at && new Date(key.expires_at) < new Date()) return '已过期'; return '有效' }
watch(tab, value => { if (value === 'presets') loadPresets() })
onMounted(() => { loadPlaylists(); if (isWriter.value) loadPresets() })
</script>

<template>
  <main class="page page-narrow settings-page">
    <header class="settings-head">
      <SegmentedControl v-model="tab" :options="tabs" aria-label="设置分组" />
      <button class="text-button danger" @click="emit('logout')"><AppIcon name="logout" :size="16" />登出</button>
    </header>
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <Transition name="tab" mode="out-in">
      <section v-if="tab === 'music'" key="music" v-stagger class="settings-panel">
        <template v-if="isWriter">
          <article class="card">
            <h2 class="card-title">页面歌单<small>三页各自独立，切页换列表、切回续播</small></h2>
            <SegmentedControl v-model="page" :options="pageOptions" aria-label="选择页面" />
            <ul class="track-list">
              <li v-if="defaultEnabled"><span class="track-name"><AppIcon name="music" :size="14" />默认音频<small class="faint">内置合成</small></span><button class="text-button" @click="updatePlaylist({ default_enabled: false })">移除</button></li>
              <li v-else-if="!tracks.length"><span class="track-name faint">默认音频已移除</span><button class="text-button" @click="updatePlaylist({ default_enabled: true })">恢复</button></li>
              <li v-for="track in tracks" :key="track.id"><span class="track-name"><AppIcon name="music" :size="14" />{{ track.title }}<small class="faint mono">{{ (track.byteSize / 1048576).toFixed(1) }} MB</small></span><button class="text-button danger" @click="removeTrack(track.id)">移除</button></li>
            </ul>
            <div class="track-actions">
              <label class="btn upload-audio" :class="{ busy: uploading }"><AppIcon name="upload" :size="16" />{{ uploading ? '上传中' : '上传音频' }}<input type="file" accept="audio/*" :disabled="uploading" @change="addTrack" /></label>
              <small class="faint">MP3 / OGG / FLAC / M4A / WAV，单曲 ≤ 20MB，每页 ≤ 50 首</small>
            </div>
          </article>
          <article class="card">
            <h2 class="card-title">播放模式</h2>
            <SegmentedControl :model-value="playlist?.mode ?? 'list'" :options="modeOptions" aria-label="播放模式" @update:model-value="updatePlaylist({ mode: $event as PlaylistMode })" />
          </article>
        </template>
        <article class="card">
          <h2 class="card-title">音量<small>三页共用</small></h2>
          <label class="volume-row"><AppIcon name="volume" :size="16" /><input v-model.number="volume" type="range" min="0" max="100" :style="{ '--range-fill': `${volume}%` }" @change="saveVolume" /><b class="mono">{{ volume }}%</b></label>
        </article>
      </section>

      <section v-else-if="tab === 'presets'" key="presets" v-stagger class="settings-panel presets">
        <article class="card">
          <h2 class="card-title">权限预设<small>无规则的预设默认拒绝</small></h2>
          <form class="form-row" @submit.prevent="createPreset"><input v-model="presetName" placeholder="新预设名称，如：仅家人可见" maxlength="30" /><button class="primary" type="submit" :disabled="busy || !presetName.trim()">创建</button></form>
          <ul v-if="presets.length" class="preset-list">
            <li v-for="preset in presets" :key="preset.id"><button v-glow class="preset-item" :class="{ active: selectedPreset?.id === preset.id }" @click="selectPreset(preset)"><b>{{ preset.name }}</b><small class="faint">{{ preset.rules.filter(rule => rule.allowed).length }} 把密钥可见</small></button></li>
          </ul>
          <EmptyState v-else icon="shield" text="还没有权限预设" compact />
        </article>
        <article class="card">
          <Transition name="fade-slide" mode="out-in">
            <div v-if="selectedPreset" :key="selectedPreset.id" class="preset-editor">
              <h2 class="card-title">{{ selectedPreset.name }}<small>勾选允许查看的阅读密钥</small></h2>
              <ul v-if="readerKeys.length" class="rule-list">
                <li v-for="key in readerKeys" :key="key.id"><label class="task-row"><input v-model="draftRules[key.id]" type="checkbox" :disabled="!!key.revoked_at" /><span class="task-body"><span>{{ key.nickname }}</span><small class="faint">锚点 {{ key.anchor_local_date }}{{ key.note ? ` · ${key.note}` : '' }}</small></span><small class="faint">{{ keyStatus(key) }}</small></label></li>
              </ul>
              <EmptyState v-else icon="key" text="还没有阅读密钥" compact />
              <button class="primary" :disabled="busy || !readerKeys.length" @click="saveRules">保存规则</button>
            </div>
            <EmptyState v-else icon="shield" text="选择一个预设来编辑规则" />
          </Transition>
        </article>
      </section>

      <section v-else-if="tab === 'theme'" key="theme" v-stagger class="settings-panel">
        <article class="card">
          <h2 class="card-title">外观<small>日出日落按 6:30 – 18:30 自动切换</small></h2>
          <SegmentedControl :model-value="props.appearance" :options="appearanceOptions" aria-label="外观" @update:model-value="emit('update:appearance', $event as Appearance)" />
        </article>
        <article class="card">
          <h2 class="card-title">顶栏位置<small>也可以直接拖动顶栏左侧手柄</small></h2>
          <SegmentedControl :model-value="props.navPosition" :options="navOptions" aria-label="顶栏位置" @update:model-value="emit('update:navPosition', $event as 'top' | 'left' | 'right' | 'bottom')" />
        </article>
        <article v-if="isWriter" class="card">
          <h2 class="card-title">规划横幅轮播间隔</h2>
          <label class="form-row interval-row"><input v-model.number="carouselSeconds" type="number" min="2" max="120" @change="saveCarousel" /><span class="faint">秒</span></label>
        </article>
        <article v-if="!isWriter" class="card">
          <h2 class="card-title">音量</h2>
          <label class="volume-row"><AppIcon name="volume" :size="16" /><input v-model.number="volume" type="range" min="0" max="100" :style="{ '--range-fill': `${volume}%` }" @change="saveVolume" /><b class="mono">{{ volume }}%</b></label>
        </article>
      </section>

      <section v-else key="keys" v-stagger class="settings-panel">
        <article class="card">
          <h2 class="card-title">阅读密钥<small>由管理员在 /admin 签发与作废</small></h2>
          <ul v-if="readerKeys.length" class="key-list">
            <li v-for="key in readerKeys" :key="key.id" :class="{ revoked: key.revoked_at }">
              <span class="key-main"><b>{{ key.nickname }}</b><small class="faint">{{ key.note || '无备注' }}</small></span>
              <span class="key-meta mono faint">锚点 {{ key.anchor_local_date }}<template v-if="key.expires_at"> · 到期 {{ key.expires_at.slice(0, 10) }}</template></span>
              <span class="key-status" :class="keyStatus(key) === '有效' ? 'ok' : 'off'">{{ keyStatus(key) }}</span>
            </li>
          </ul>
          <EmptyState v-else icon="key" text="还没有签发阅读密钥" />
        </article>
      </section>
    </Transition>
  </main>
</template>
