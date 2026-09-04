<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type Playlist, type PlaylistPage } from '../api/client'

const props = defineProps<{ appearance: 'dark' | 'light' | 'auto' }>()
const emit = defineEmits<{ (event: 'update:appearance', value: 'dark' | 'light' | 'auto'): void }>()
const volume = ref(Number(localStorage.getItem('cyberlife-volume') || 70))
const page = ref<PlaylistPage>('now')
const playlists = ref<Record<PlaylistPage, Playlist | null>>({ now: null, past: null, future: null })
const loading = ref(false)
const error = ref('')
const selectedPlaylist = computed(() => playlists.value[page.value])
const tracks = computed(() => selectedPlaylist.value?.tracks ?? [])
const defaultEnabled = computed(() => selectedPlaylist.value?.defaultEnabled ?? true)

async function load() {
  loading.value = true
  try {
    const items = (await api.musicPlaylists()).items
    playlists.value = { now: null, past: null, future: null }
    for (const item of items) playlists.value[item.page] = item
    window.dispatchEvent(new Event('cyberlife-playlist-change'))
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '读取歌单失败' } finally { loading.value = false }
}
function saveVolume() { localStorage.setItem('cyberlife-volume', String(volume.value)) }
async function addTrack(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 20 * 1024 * 1024) { error.value = '音频不能超过 20MB'; return }
  if (tracks.value.length >= 50) { error.value = '每页最多 50 首'; return }
  try { await api.uploadMusicTrack(page.value, file); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '上传失败' } finally { (event.target as HTMLInputElement).value = '' }
}
async function removeTrack(id: string) { try { await api.deleteMusicTrack(id); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败' } }
async function removeDefault() { const playlist = selectedPlaylist.value; if (!playlist) return; try { await api.replaceMusicPlaylist(page.value, { name: playlist.name, mode: playlist.mode, volume: playlist.volume, default_enabled: false }); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '更新失败' } }
onMounted(load)
</script>

<template>
  <main class="settings-page"><section class="settings-grid">
    <article class="panel"><h2>主题</h2><div class="segmented"><button v-for="item in [['dark', '始终黑色'], ['light', '始终白色'], ['auto', '日出日落']]" :key="item[0]" :class="{ active: props.appearance === item[0] }" @click="emit('update:appearance', item[0] as 'dark' | 'light' | 'auto')">{{ item[1] }}</button></div><label class="setting-row">全局音量 <input v-model.number="volume" type="range" min="0" max="100" @change="saveVolume" /> {{ volume }}%</label></article>
    <article class="panel"><h2>歌单</h2><p v-if="error" class="error">{{ error }}</p><div class="segmented"><button v-for="item in ['now', 'past', 'future']" :key="item" :class="{ active: page === item }" @click="page = item as PlaylistPage">{{ item === 'now' ? '现在' : item === 'past' ? '过去' : '未来' }}</button></div><label class="upload-audio">添加音频<input type="file" accept="audio/*" @change="addTrack" /></label><ul class="track-list"><li v-if="!tracks.length && defaultEnabled"><span>默认音频</span><button @click="removeDefault">移除</button></li><li v-for="track in tracks" :key="track.id"><span>{{ track.title }}</span><button @click="removeTrack(track.id)">移除</button></li><li v-if="!loading && !tracks.length && !defaultEnabled" class="empty">尚无曲目</li></ul></article>
  </section></main>
</template>
