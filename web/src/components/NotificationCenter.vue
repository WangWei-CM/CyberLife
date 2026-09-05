<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api, type Notice } from '../api/client'
import { timeLabel } from '../lib/dates'
import AppIcon from './AppIcon.vue'
import EmptyState from './EmptyState.vue'

const emit = defineEmits<{ (event: 'navigate', notice: Notice): void }>()
const notices = ref<Notice[]>([])
const open = ref(false)
const busy = ref(false)
const root = ref<HTMLElement>()
let poll: number | undefined

const unread = computed(() => notices.value.filter(item => !item.read).length)
const typeLabel: Record<string, string> = { comment: '评论', key_expired: '密钥过期', plan_due: '规划到期', life_status: '状态变化', secret_opened: '绝密解除' }
const sorted = computed(() => [...notices.value].sort((a, b) => b.createdAt.localeCompare(a.createdAt)))

async function load() { try { notices.value = (await api.notifications()).items } catch { /* 通知不可用时保持静默 */ } }
async function markAll() {
  if (busy.value) return
  busy.value = true
  try { await Promise.all(notices.value.filter(item => !item.read).map(item => api.markNotificationRead(item.id))); await load() } finally { busy.value = false }
}
async function markOne(notice: Notice) { if (notice.read) return; try { await api.markNotificationRead(notice.id); notice.read = true } catch { /* ignore */ } }
/** 点击通知：标记已读并跳转到对应内容（评论→该日，规划→详情，密钥→密钥管理）。 */
async function openNotice(notice: Notice) {
  await markOne(notice)
  const jumpable = (notice.type === 'comment' && notice.refDate) || (notice.type === 'plan_due' && notice.refID) || notice.type === 'key_expired'
  if (!jumpable) return
  close()
  emit('navigate', notice)
}
function toggle() { open.value = !open.value }
function close() { open.value = false }
function onDocumentClick(event: MouseEvent) { if (open.value && root.value && !root.value.contains(event.target as Node)) close() }
function onKey(event: KeyboardEvent) { if (event.key === 'Escape') close() }
function dateLabel(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : `${date.getMonth() + 1}月${date.getDate()}日 ${timeLabel(date)}` }

onMounted(() => { load(); poll = window.setInterval(load, 60_000); document.addEventListener('click', onDocumentClick); document.addEventListener('keydown', onKey) })
onBeforeUnmount(() => { if (poll) clearInterval(poll); document.removeEventListener('click', onDocumentClick); document.removeEventListener('keydown', onKey) })
defineExpose({ toggle, close, load })
</script>

<template>
  <div ref="root" class="notice-wrap">
    <button v-glow class="icon-button notice-button" :class="{ active: open }" aria-label="通知中心" :aria-expanded="open" @click="toggle">
      <AppIcon name="bell" />
      <Transition name="pop"><span v-if="unread" class="notice-dot" :key="unread">{{ unread > 99 ? '99+' : unread }}</span></Transition>
    </button>
    <Transition name="popover">
      <section v-if="open" class="popover notice-popover" aria-label="通知列表">
        <header class="popover-head"><b>通知</b><button v-if="unread" class="text-button" :disabled="busy" @click="markAll"><AppIcon name="check" :size="14" />全部已读</button></header>
        <EmptyState v-if="!sorted.length" icon="inbox" text="还没有通知" compact />
        <ul v-else class="notice-list">
          <li v-for="notice in sorted" :key="notice.id" :class="{ unread: !notice.read }">
            <button class="notice-item" @click="openNotice(notice)">
              <span class="notice-type">{{ typeLabel[notice.type] ?? notice.type }}<i v-if="notice.refDate || notice.refID" class="notice-jump"><AppIcon name="chevron-right" :size="12" /></i></span>
              <p>{{ notice.text }}</p>
              <small>{{ dateLabel(notice.createdAt) }}</small>
            </button>
          </li>
        </ul>
      </section>
    </Transition>
  </div>
</template>
